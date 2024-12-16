# CheckpointSpawner for JupyterHub
CheckpointSpawner is a class extending KubeSpawner with checkpoint/restore functionality for Jupyter Notebooks.

CheckpointSpawner checkpoints a Notebook when it is about to be stopped. On start, CheckpointSpawner looks if there
is a checkpoint container image ready and if, the Notebook Pod is create from this image.

CheckpointSpawner checkpoints Jupyter Notebooks through [Checkpointer](../checkpointer), which means
deploying Checkpointer is a prerequisite for CheckpointSpawner to work.

## Building
CheckpointSpawner is not a standalone binary. It is imported as a class and runs withing the Hub component
of JupyterHub, specifically the Zero to JupyterHub with Kubernetes (Z2JH) distribution. Therefore, building
CheckpointSpawner container means building the Hub where CheckpointSpawner is installed as well.

CheckpointSpawner has a pre-built public image `pbaran555/hub-checkpoint-spawner:1.0.0`.

Install the `build` package, necessary for building Python wheel packages, by:
```shell
python3 -m pip install build
```

Building the `checkpoint-spawner` wheel package:
```shell
python3 -m build
```
This should produce a `dist/checkpoint_spawner-1.0.0-py3-none-any.whl` wheel.
The name of the package and version can be changed through the `pyproject.toml` file.
The Dockerfile in this directory is set up to copy this wheel into the publicly provided Hub container image
and then install it wit `pip install` as any other wheel package.

Build a dockerfile with the wheel:
 ```shell
 docker build -t pbaran555/hub-checkpoint-spawner:1.0.0 . # replace with custom image name
 ```

You can also push the wheel package to package repository with twine:
```shell
python3 -m pip install twine
python3 -m twine upload -u __token__ -p $YOUR_PYPITOKEN --skip-existing --verbose --repository testpypi dist/*
```

To push the container image to a remote registry, run:
```shell
docker push pbaran555/hub-checkpoint-spawner:1.0.0 # replace with custom image name
```

## Deploying

Inspect the `deploy/config.yaml` if you would like to customize your deployment in any way.
To deploy Z2JH JupyterHub with CheckpointSpawner, run:
```shell
helm upgrade --cleanup-on-fail \
  --install jupyterhub jupyterhub/jupyterhub \
  --namespace hub-ns \
  --create-namespace \
  --version=4.0.0 \
  --values deploy/config.yaml
```

If you would like to change the config.yaml and redeploy, you can:
```shell
helm upgrade --cleanup-on-fail \
  --install jupyterhub jupyterhub/jupyterhub \
  --namespace hub-ns \
  --values deploy/config.yaml
```

To expose JupyterHub, run:
```shell
kubectl --namespace=hub-ns port-forward service/proxy-public --address 0.0.0.0 8080:http
```

### Uninstalling 
Delete Helm release by:
```shell
helm delete jupyterhub
```

Delete JupyterHub namespace:
```shell
kubectl delete namespace hub-ns
```

## Testing checkpoint/restore of Jupyter Notebook
See the main [README.md](../README.md).

