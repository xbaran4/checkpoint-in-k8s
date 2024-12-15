# Checkpoint in Kubernetes
This project implements JupyterHub Spawner capable of checkpoint/restore of Jupyter Notebook containers within
Kubernetes cluster.

## Repository Structure
The project consist of the main components:
- `CheckpointSpawner`, which integrates checkpoint/restore functionality into Z2JH JupyterHub distribution.
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
checkpoint/restore of containers. See [Timer's README.md](timer/README.md).

## How to run the project
The project cannot be deployed to Kubernetes clusters of public cloud providers like AWS EKS or Google GKE, as it
requires installation of CRIU binary and additional configuration of each Kubernetes Node.

### 1. Setup Kubernetes cluster
First, a Kubernetes cluster needs to be installed along with CRIU and chosen container runtime. The script
[k8s-install-control.sh](scripts/k8s-install-control.sh) can be used in cut-and-paste manner to install the cluster.

Note that currently, restoring a container with `containerd` is not possible even in the latest release.
A patched version of containerd must be installed.

### 2. Configure runc with CRIU
The next step is to configure runc to use CRIU with specific flags defined in [runc.conf](etc/runc.conf).

The configuration file ensures that CRIU closes all TCP connections and dumps file locks. Additionally, it instructs
CRIU to redirect its logs to `/tmp/criu.log`, so this is the file to look into when encountering any checkpoint/restore
issues.

Copy the file to the `criu` directory in `etc` by:
```shell
sudo cp ./etc/runc.conf /etc/criu/runc.conf
```
If the directory does not exist, create it by:
```
sudo mkdir /etc/criu
```

### 3. Deploy Checkpointer
Deploying Checkpointer requires applying Kubernetes manifests in the `checkpointer/k8s-manifests` directory and
providing two secret values:
- TLS secret for authentication to Kubelet
- dockerconfigjson secret for pushing checkpoint container images to container registry

See [Checkpointer's README.md](checkpointer/README.md) for details.

### 4. Deploy CheckpointSpawner
Deploying CheckpointSpawner means applying Z2JH Helm charts with overridden values file `config.yaml`.
See [CheckpointSpawner's README.md](checkpoint-spawner/README.md) for details.

### 5. Checkpoint/restore Jupyter Notebook
In case you would like to expose JupyterHub publicly outside the cluster,
[CheckpointSpawner's README.md](checkpoint-spawner/README.md) contains a kubectl command for that.

Login to JupyterHub (Z2JH is by default configured with DummyAuthenticator so any user can log in) and spawn a Notebook.

Do whatever you would like in the Notebook, then stop the Notebook through the JupyterHub UI.

Start the Notebook again, wait and observe that the Notebook is in the same state as before stopping.
Even though the `config.yaml` does not set any permanent storage for the Notebooks, the file system changes are
preserved. 

Note that TCP connection had to be closed due to different Pod IP of the Notebook. This might influence the output of
the running code that Jupyter Notebook streams from Kernel to the browser.
