# Checkpoint Spawner for JupyterHub


## Building

1. Building the wheel package:
    ```shell
    python -m build
    ```

2. Build a dockerfile with the wheel:
    ```shell
    docker build -t pbaran555/hub-checkpointer:0.0.1 .
    ```

## Pushing
```shell
docker push pbaran555/hub-checkpointer:0.0.1
```

## Create JupyterHub with CheckpointSpawner
```shell
helm upgrade --cleanup-on-fail --install jupyterhub jupyterhub/jupyterhub --namespace hub-ns --values deploy/config.yaml
```

## Testing notebook
```python
import time
for i in range(1000):
    print(i)
    time.sleep(1)
```