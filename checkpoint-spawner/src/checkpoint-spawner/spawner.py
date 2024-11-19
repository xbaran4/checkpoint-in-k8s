import asyncio
import json

from kubespawner import KubeSpawner
from traitlets import Unicode
from tornado.httpclient import AsyncHTTPClient, HTTPRequest

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

    # @property
    # def slow_stop_timeout(self):
    #     return 100

    def __init__(self, *args, **kwargs):
        self.log.info(f'__init__ called, with will_resume value: {self.will_resume}')
        self.log.info(f'spawner status: {self}')
        self.log.info(f'oauth info: {self.api_token} and {self.oauth_client_id}')
        super().__init__(*args, **kwargs)
        self.checkpoint_image_name = None
        self.old_api_token = None

    def load_state(self, state):
        self.log.info('load_state called')
        self.log.info(f'spawner status: {self}')
        self.log.info(f'oauth info: {self.api_token} and {self.oauth_client_id}')
        super().load_state(state)

        if 'checkpoint_image_name' in state:
            self.checkpoint_image_name = state['checkpoint_image_name']
        # TODO: experiment
        if 'old_api_token' in state:
            self.old_api_token = state['old_api_token']

    def get_state(self):
        self.log.info('get_state called')
        self.log.info(f'spawner status: {self}')
        self.log.info(f'oauth info: {self.api_token} and {self.oauth_client_id}')
        state = super().get_state()
        state['checkpoint_image_name'] = self.checkpoint_image_name
        state['old_api_token'] = self.old_api_token
        return state

    # Original Spawner clears the api_token and KubeSpawner calls super(). Since we are checkpointing we need
    # preserver the api_token.
    # def clear_state(self):
    #     self.log.info('clear_state called')
    #     api_token = self.api_token
    #     super().clear_state()
    #     self.api_token = api_token

    async def start(self):
        self.log.info('start called')
        self.log.info(f'spawner status: {self}')
        self.log.info(f'LATEST TOKEN: {self.api_token} and {self.oauth_client_id} and {self.old_api_token}')

        if self.old_api_token:
            self.api_token = self.old_api_token
        else:
            self.old_api_token = self.api_token

        self.log.info(f'oauth info: {self.api_token} and {self.oauth_client_id} and {self.old_api_token}')

        if self.checkpoint_image_name is not None:
            # cp_url = f'http://{self.checkpoint_service_name}/checkpoint'
            # self.log.info(f'calling checkpoint status to {cp_url}')
            # http_client = AsyncHTTPClient()
            # request = HTTPRequest(
            #     url=cp_url + f"?containerPath={self.namespace}/{self.pod_name}/notebook&hang=true",
            #     method="GET"
            # )
            #
            # await http_client.fetch(request)
            self.modify_pod_hook = self.modify_pod

        return await super().start()

    async def stop(self, now=False):
        container_image_name = f'pbaran555/kaniko-checkpointed:{self.user.name}'
        payload = dict(containerImageName=container_image_name,
                       containerPath=f"{self.namespace}/{self.pod_name}/notebook",
                       deletePod=True)
        cp_url = f'http://{self.checkpoint_service_name}/checkpoint'

        self.log.info(f'stop called; calling checkpoint to {cp_url}, with payload {payload}, with async client')
        self.log.info(f'spawner status: {self}')
        self.log.info(f'oauth info: {self.api_token} and {self.oauth_client_id}')

        http_client = AsyncHTTPClient()
        request = HTTPRequest(
            url=cp_url,
            method="POST",
            headers={
                "Content-Type": "application/json"
            },
            body=json.dumps(payload)
        )

        response = None
        try:
            response = await http_client.fetch(request)
            self.log.info(f"Response Code: {response.code}")
            self.log.info(f"Response Body: {response.body.decode('utf-8')}")
        except Exception as e:
            self.log.error(f"STOP Exception occurred: {e}")
        self.log.info(f'checkpoint called with status {response.code}')

        # await super().stop(now)
        self.checkpoint_image_name = container_image_name
        self.log.info('STOP DONE')


    @staticmethod
    def modify_pod(spawner, pod):
        spawner.log.info('modify_pod called')
        if spawner.checkpoint_image_name:
            pod.spec.containers[0].image_pull_policy = 'Always'
            pod.spec.containers[0].image = spawner.checkpoint_image_name
        return pod
