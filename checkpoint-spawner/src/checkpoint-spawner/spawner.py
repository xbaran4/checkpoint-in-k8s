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
        Name of the checkpoint service. Value used for nslookup.
        """,
    )

    @property
    def will_resume(self):
        return True


    def __init__(self, *args, **kwargs):
        self.log.info(f'__init__ CALLED')
        super().__init__(*args, **kwargs)
        self.checkpoint_image_name = None
        self.latest_checkpoint_identifier = None
        self.initial_api_token = None
        self.checkpoints = []
        self.http_client = AsyncHTTPClient()

    def load_state(self, state):
        self.log.info('LOAD_STATE CALLED')
        super().load_state(state)

        if 'checkpoint_image_name' in state:
            self.checkpoint_image_name = state['checkpoint_image_name']
        if 'old_api_token' in state:
            self.initial_api_token = state['old_api_token']
        if 'latest_checkpoint_identifier' in state:
            self.latest_checkpoint_identifier = state['latest_checkpoint_identifier']
        if 'checkpoints' in state:
            self.checkpoints = state['checkpoints']



    def get_state(self):
        self.log.info('GET_STATE CALLED')
        state = super().get_state()
        state['checkpoint_image_name'] = self.checkpoint_image_name
        state['old_api_token'] = self.initial_api_token
        state['latest_checkpoint_identifier'] = self.latest_checkpoint_identifier
        state['checkpoints'] = self.checkpoints
        return state


    async def start(self):
        self.log.info('START CALLED')
        # Always use the initial_api_token as api_token
        if self.initial_api_token:
            self.api_token = self.initial_api_token
        else:
            self.initial_api_token = self.api_token

        if self.latest_checkpoint_identifier:
            self.log.warn('found checkpoint identifier, fetching result')
            try:
                checkpoint_identifier = self.latest_checkpoint_identifier
                self.checkpoint_image_name = await self.get_checkpoint_result(checkpoint_identifier)
                self.checkpoints.append(self.checkpoint_image_name)
                self.modify_pod_hook = self.modify_pod
            except HTTPError:
                self.log.warn('Checkpointer responded with non-200 response')
                if len(self.checkpoints) > 0:
                    self.log.warn('found previous checkpoint which can be used')
                    self.checkpoint_image_name = self.checkpoints[-1]
                    self.modify_pod_hook = self.modify_pod

        start_response = await super().start()
        self.latest_checkpoint_identifier = None
        return start_response

    async def stop(self, now=False):
        self.log.info(f'STOP CALLED, now={now}')
        if now:
            self.log.info(f'stop wanted immediately, will not checkpoint')
            await super().stop(now)
            return

        if not self.latest_checkpoint_identifier:
            self.log.info('will make a checkpoint request')
            self.latest_checkpoint_identifier = await self.make_checkpoint_request()
        else:
            self.log.info('checkpointing should be in progress')

        self.log.info('STOP DONE')

    async def poll(self):
        self.log.info('POLL CALLED')
        # Checkpointing should be in progress
        if self.latest_checkpoint_identifier:
            return 0
        return await super().poll()

    async def make_checkpoint_request(self) -> str:
        payload = {"async": True, "deletePod": True}
        cp_url = f'http://{self.checkpoint_service_name}/checkpoint/{self.namespace}/{self.pod_name}/notebook'
        request = HTTPRequest(
            url=cp_url,
            method="POST",
            headers={"Content-Type": "application/json"},
            body=json.dumps(payload)
        )
        self.log.info(f'Making a checkpoint request to {cp_url}')
        response = await self.http_client.fetch(request)
        data = json.loads(response.body)
        return data['checkpointIdentifier']

    async def get_checkpoint_result(self, checkpoint_identifier: str) -> str:
        cp_url = f'http://{self.checkpoint_service_name}/checkpoint?checkpointIdentifier={checkpoint_identifier}'
        self.log.info(f'Getting checkpoint result from {cp_url}')
        response = await self.http_client.fetch(HTTPRequest(url=cp_url, method="GET"))
        data = json.loads(response.body)
        return data['containerImageName']


    @staticmethod
    def modify_pod(spawner, pod):
        spawner.log.info('MODIFY_POD CALLED')
        if spawner.checkpoint_image_name:
            pod.spec.containers[0].image_pull_policy = 'Always'
            pod.spec.containers[0].image = spawner.checkpoint_image_name
        return pod
