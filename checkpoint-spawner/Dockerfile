FROM --platform=linux/amd64 quay.io/jupyterhub/k8s-hub:4.0.0

COPY dist/checkpoint_spawner-1.0.0-py3-none-any.whl /

RUN pip install /checkpoint_spawner-1.0.0-py3-none-any.whl
