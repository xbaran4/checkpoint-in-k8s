# Checkpoint in Kubernetes
This project implements JupyterHub Spawner capable of checkpoint/restore of Jupyter Notebook containers within
Kubernetes cluster.

## Repository Structure
The project consist of the main components:
- `CheckpointSpawner`, which integrates checkpoint/restore functionality into
    Z2JH JupyterHub distribution
- `Checkpointer` standalone application, which provides HTTP API for checkpointing a container in Kubernetes cluster.

### checkpoint-spawner
Contains CheckpointSpawner JupyterHub Spawner that extends Z2JH's KubeSpawner by integrating checkpoint/restore
functionality. See [CheckpointSpawner's README.md](checkpoint-spawner/README.md).

### checkpointer
Contains Checkpointer, Go application running as a Pod on each Node in Kubernetes.
See [Checkpointer's README.md](checkpointer/README.md).

### scripts
The `scripts` directory contains script(s) for installing container runtime binaries and Kubernetes binaries.

### etc
The `etc` directory contains configuration file(s) required to be set up on each Kubernetes Node to
checkpoint/restore Jupyter Notebook containers without issues.

### timer
The timer directory contains source code and Dockerfile for Timer, a simple Go application useful for testing
checkpointing of containers. See [Timer's README.md](timer/README.md).

## How to run the project
The project cannot be deployed to Kubernetes clusters of cloud providers like AWS EKS or Google GKE, as it requires
installation of CRIU binary and some additional configuration of each Kubernetes Node.

### 1. Setup Kubernetes cluster
First a Kubernetes cluster needs to be built. The script [k8s-install-control.sh](scripts/k8s-install-control.sh)
can be used in cut-and-paste manner to install the cluster 

### 2. Configure runc with CRIU

### 3. Deploy Checkpointer
Creating Kubernetes TLS Secret with certificate and private key for authentication to Kubelet, and dockerconfigjson
Secret with credentials to container registry.

Building Checkpointer image with Docker.

Building CheckpointSpawner package and Hub container image using importing package. 

Deploying Checkpointer, with environment variable pointing to a remote container registry.

### 4. Deploy CheckpointSpawner
Deploying Z2JH Helm charts with overridden config.yaml.