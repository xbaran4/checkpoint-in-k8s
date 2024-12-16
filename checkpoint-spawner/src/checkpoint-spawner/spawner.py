import asyncio
import json

from kubespawner import KubeSpawner
from traitlets import Unicode
from tornado.httpclient import AsyncHTTPClient, HTTPRequest, HTTPError

class CheckpointSpawner(KubeSpawner):
    checkpoint_service_name = Unicode(
        "checkpoint-service.kube-system",
        config=True,
        help="""
        Name of the checkpoint service. Value used for nslookup. If Checkpointer's Kubernetes service is named
        differently than 'checkpoint-service', this setting has to be changed through the config.yaml.
        """,
    )

    # Hub will not delete the Notebook's API token from database.
    # See https://github.com/jupyterhub/jupyterhub/blob/0d57ce2e337a13d69bb302e81304701b712af4e9/jupyterhub/spawner.py#L454
    # and https://github.com/jupyterhub/jupyterhub/blob/0d57ce2e337a13d69bb302e81304701b712af4e9/jupyterhub/user.py#L1109
    @property
    def will_resume(self):
        return True

    # Changing JupyterHub log level to debug results in very messy logs and makes it hard to track what is happening.
    # Therefore, CheckpointSpawner logs everything as info.

    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self.log.info(f'CheckpointSpawner: __init__() with checkpoint-service: {self.checkpoint_service_name}')

        # Name of the checkpoint container image as returned by Checkpointer after checkpointing.
        self.checkpoint_image = None

        # Name of the previous checkpoint container image, which can be used as a backup in case checkpointing fails.
        self.previous_checkpoint_image = None

        # CheckpointIdentifier that Checkpointer returns as part of its asynchronous API.
        self.checkpoint_identifier = None

        # The API token that is generated to the 'non-checkpointed' Jupyter Notebook.
        # It is reused for all subsequent checkpointed Jupyter Notebooks.
        self.initial_api_token = None


        # Asynchronous HTTP client for calling Checkpointer API.
        self.http_client = AsyncHTTPClient()

    # Through load_state the instance can load its in-memory state from the state object which has been fed from DB.
    def load_state(self, state):
        super().load_state(state)

        if 'checkpoint_image' in state:
            self.checkpoint_image = state['checkpoint_image']
        if 'previous_checkpoint_image' in state:
            self.previous_checkpoint_image = state['previous_checkpoint_image']
        if 'initial_api_token' in state:
            self.initial_api_token = state['initial_api_token']
        if 'checkpoint_identifier' in state:
            self.checkpoint_identifier = state['checkpoint_identifier']

        self.log.info('CheckpointSpawner: load_state()')

    # get_state will persist everything fed into state to DB
    def get_state(self):
        state = super().get_state()

        state['checkpoint_image'] = self.checkpoint_image
        state['previous_checkpoint_image'] = self.previous_checkpoint_image
        state['initial_api_token'] = self.initial_api_token
        state['checkpoint_identifier'] = self.checkpoint_identifier

        self.log.info('CheckpointSpawner: get_state()')
        return state

    # If there is no checkpoint_identifier, start will proceed as if only KubeSpawner was in control.
    # However, if there is, it will try to get a result of the checkpoint and modify the Pod manifest to use
    # checkpoint container image. In case checkpoint fails, it can use a previous checkpoint image as fallback.
    # If there is no previous checkpoint, simply make a Pod from standard container image.
    async def start(self):
        self.log.info('CheckpointSpawner: start()')

        # Always use the initial_api_token as api_token
        if self.initial_api_token:
            self.api_token = self.initial_api_token
        else:
            self.initial_api_token = self.api_token

        if self.checkpoint_identifier:
            self.log.info(f'CheckpointSpawner: found checkpoint identifier: {self.checkpoint_identifier}, fetching result')
            try:
                self.checkpoint_image = await self.get_checkpoint_result(self.checkpoint_identifier)
                self.modify_pod_hook = self.modify_pod
            except HTTPError as e:
                self.log.error(f'CheckpointSpawner: Checkpointer responded with non-200 response: {e}')
                if self.previous_checkpoint_image:
                    self.log.warn(f'CheckpointSpawner: found previous checkpoint which can be used: {self.previous_checkpoint_image}')
                    self.checkpoint_image = self.previous_checkpoint_image
                    self.modify_pod_hook = self.modify_pod
                else:
                    self.log.warn(f'CheckpointSpawner: no previous checkpoint which can be used, will start a new Notebook.')
            self.checkpoint_identifier = None

        kubespawner_start_response = await super().start()
        self.previous_checkpoint_image = self.checkpoint_image
        self.checkpoint_image = None
        return kubespawner_start_response


    # If Notebook should be stopped right away, delegate this to KubeSpawner, which will just delete the Pod.
    # Otherwise, make a checkpoint request and make Checkpointer will eventually delete the Pod.
    async def stop(self, now=False):
        self.log.info(f'CheckpointSpawner: stop(now={now})')
        if now:
            self.log.info('CheckpointSpawner: immediate stop wanted, will not checkpoint')
            await super().stop(now)
            return

        if not self.checkpoint_identifier:
            self.log.info('CheckpointSpawner: no checkpoint in progress, will make a checkpoint request')
            self.checkpoint_identifier = await self.make_checkpoint_request()
        else:
            self.log.info(f'CheckpointSpawner: checkpointing should be in progress: {self.checkpoint_identifier}')


    # After stop() is called, Hub needs to think that the Notebook has stopped. However, during checkpoint, the Notebook
    # Pod is still 'running' until Checkpointer deletes the Pod.
    async def poll(self):
        if self.checkpoint_identifier:
            self.log.info('CheckpointSpawner: poll(), checkpointing was triggered, misleading Hub that Notebook is stopped.')
            return None
        return await super().poll()

    # Generator for showing user progress of Notebook spawn. If there is no checkpoint_identifier
    # delegate this to KubeSpawner. Otherwise, it means that CheckpointSpawner is still waiting for
    # checkpointing to finish, so give users some illusion of progress by counting.
    async def progress(self):
        if not self.checkpoint_identifier:
            async for event in super().progress():
                yield event
            return

        for i in range(0, 10):
            if self.checkpoint_identifier:
                yield {
                    'progress': i * 10,
                    'raw_event': "checkpoint in progress",
                    'message': f"Checkpoint with identification: {self.checkpoint_identifier} is still in progress {i * 5} seconds...",
                }
                await asyncio.sleep(5)
            else:
                yield {
                    'progress': 100,
                    'raw_event': "checkpoint done",
                    'message': f"Checkpointing finished after {i * 5} seconds, Notebook Pod will start from container: {self.checkpoint_image}.",
                }
                break

    # Makes a request to Checkpointer API to checkpoint Notebook and delete the Pod, returns checkpointIdentifier.
    async def make_checkpoint_request(self) -> str:
        payload = {"async": True, "deletePod": True}
        cp_url = f'http://{self.checkpoint_service_name}/checkpoint/{self.namespace}/{self.pod_name}/notebook'
        request = HTTPRequest(
            url=cp_url,
            method="POST",
            headers={"Content-Type": "application/json"},
            body=json.dumps(payload)
        )
        self.log.info(f'CheckpointSpawner: make_checkpoint_request(): {cp_url}')
        response = await self.http_client.fetch(request)
        data = json.loads(response.body)
        return data['checkpointIdentifier']


    # Makes a request to Checkpointer API for checkpoint result and returns the container name.
    async def get_checkpoint_result(self, checkpoint_identifier: str) -> str:
        cp_url = f'http://{self.checkpoint_service_name}/checkpoint?checkpointIdentifier={checkpoint_identifier}'
        self.log.info(f'CheckpointSpawner: get_checkpoint_result() to: {cp_url}')

        # Make the request with no timeout, in case it takes too long, the Spawner.start_timeout will be reached,
        # and it will terminate the coroutine.
        response = await self.http_client.fetch(HTTPRequest(url=cp_url, request_timeout=0,  method="GET"))
        data = json.loads(response.body)
        return data['containerImageName']


    # Modifies the container
    @staticmethod
    def modify_pod(spawner, pod):
        spawner.log.info(f'CheckpointSpawner: modify_pod() with container image: {spawner.checkpoint_image}')
        if spawner.checkpoint_image:
            # In the future CheckpointSpawner could also add some annotations to the Pod related to checkpointing.
            # KubeSpawner always puts the Notebook container first into the list.
            pod.spec.containers[0].image_pull_policy = 'Always'
            pod.spec.containers[0].image = spawner.checkpoint_image
        return pod
