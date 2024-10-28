from kubespawner import KubeSpawner
import requests
from traitlets import Unicode


class CheckpointSpawner(KubeSpawner):
    checkpoint_service_name = Unicode(
        "checkpoint-service.kube-system",
        config=True,
        help="""
        Name of the checkpoint service. Value used for nslookup.
        """,
    )

    def __init__(self, *args, **kwargs):
        self.log.info('__init__ called')
        super().__init__(*args, **kwargs)
        self.modify_pod_hook = self.modify_pod
        self.checkpoint_image_name = None

    def load_state(self, state):
        self.log.info('load_state called')
        super().load_state(state)

        if 'checkpoint_image_name' in state:
            self.checkpoint_image_name = state['checkpoint_image_name']

    def get_state(self):
        self.log.info('get_state called')
        state = super().get_state()
        state['checkpoint_image_name'] = self.checkpoint_image_name
        return state

    async def stop(self, now=False):
        container_image_name = f'pbaran555/kaniko-checkpointed:{self.user.name}'
        payload = dict(containerImageName=container_image_name,
                       containerPath=f"{self.namespace}/{self.pod_name}/notebook")
        cp_url = f'http://{self.checkpoint_service_name}/checkpoint'

        self.log.info(f'stop called; calling checkpoint to {cp_url}, with payload {payload}')
        r = requests.post(cp_url, json=payload)

        if r.status_code == 200:
            raise RuntimeError('Unable to checkpoint notebook.')

        self.log.info(f'checkpoint called with status {r.status_code}')

        await super().stop(now)
        # Set the checkpoint_image_name only if the checkpointing was successful and KubeSpawner stop was successful.
        if r.status_code == 200:
            self.checkpoint_image_name = container_image_name
        elif r.status_code != 404: # If 404 was returned, the container runtime could not find the container; nothing we can do.
            raise RuntimeError('Unable to checkpoint notebook.')
        self.log.info('stop done')


    @staticmethod
    def modify_pod(spawner, pod):
        spawner.log.info('modify_pod called')
        if spawner.checkpoint_image_name:
            pod.spec.containers[0].image = spawner.checkpoint_image_name
        return pod
