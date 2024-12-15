# Checkpointer microservice

Checkpointer runs as single Pod on each Kubernetes Node. Exposes HTTP API to checkpoint a container in the cluster.

## Building
There would be no point in building just the Checkpointer binary as Checkpointer is meant to run as a container within
Kubernetes cluster.

Checkpointer has a pre-built public image `pbaran555/checkpointer:1.0.0`.

To build a custom Checkpointer container, run:
```shell
docker build -t pbaran555/checkpointer:1.0.0 . # replace with custom image name
```

To push the container image to a remote registry, run:
```shell
docker push pbaran555/checkpointer:1.0.0 # replace with custom image name
```

## Deploying
Checkpointer can be deployed to Kubernetes cluster by applying all the manifest in the `k8s-manifests` directory
with kubectl and edit necessary Secrets and ConfigMap afterward.
```shell
kubect apply -f k8s-manifests
```

However, there are two Kubernetes Secrets inside the directory, which have dummy values inside and need
to be deployed with real values instead:

1. `dockerconfig-secret.yaml` needs to contain `dockerconfigjson`, the credentials for pushing (or pulling) to a
    container registry. The easiest way to create the secret is through kubectl, where the variables
    are self-explanatory:
    ```shell
    kubectl create secret -nkube-system docker-registry kaniko-secret \
    --docker-server=$REGISTRY \
    --docker-username=$USERNAME \
    --docker-password=$PASSWORD \
    --docker-email=$EMAIL
    ```

2. `kubelet-tls-secret.yaml` contains certificate and private key required for authentication to kubelet. The easiest
    way to create the secret is through kubectl:
   ```shell
    kubectl create secret -nkube-system tls kubelet-tls-secret --cert=$KUBELET_CRT --key=$KUBELET_KEY
    ```
   where the `KUBELET_CRT` and `KUBELET_KEY` represent paths to the certificate and key on the file system.  


Additionally, Checkpointer needs to know where to push the checkpoint container images. For its configuration
Checkpointer uses environment variables from `configmap.yaml`. You can edit the `configmap.yaml` directly or create
ConfigMap through kubectl:
```shell
kubectl create configmap -nkube-system checkpointer-config-2 \
 --from-literal=KUBELET_ALLOW_INSECURE=true \
  --from-literal=CHECKPOINT_IMAGE_PREFIX=pbaran555/kaniko-checkpointed
```
You can omit `KUBELET_ALLOW_INSECURE` in case your Kubelet is not using self-signed certificate. 

With this configuration Checkpointer will push images to `CHECKPOINT_IMAGE_PREFIX` repository with random
(hex encoded bytes) tag suffix, e.g.: `pbaran555/kaniko-checkpointed:618c816e64b57705`.

See [Configuration](#Configuration) section for more information


With the Secrets and ConfigMap created, the rest of the manifest can be created as well:
```shell
kubect apply -f k8s-manifests/deamonset.yaml
kubect apply -f k8s-manifests/rbac.yaml
kubect apply -f k8s-manifests/service.yaml
```

The deployment process might be improved in the future with Kustomize or Helm.

### Uninstall Checkpointer
It should be enough to remove all the manifest is `k8s-manifests` directory:
```shell
kubectl delete -f k8s-manifests
```


## Usage

By default, the `checkpoint-service` is of type ClusterIP and does not expose Checkpointer outside the cluster.
To expose it, run:
```shell
kubectl port-forward -nkube-system service/checkpoint-service 8000:80
```
Checkpoint HTTP API can now be reached through `http://localhost:8000`.

See [Timer's README.md](../timer/README.md) on how you can create a testing container.


### Checkpointing a container

Checkpointing a container can be requested through:
```
HTTP POST /checkpoint/{namespace}/{pod}/{container}
```
Checkpointer expects a JSON input body with two boolean options: `deletePod` and `async`. Not including any body is the
same as including:
```json
{
  "deletePod": false,
  "async": false
}
```
The `deletePod` tells Checkpointer to delete the container's Pod after checkpointing. The `async` options defines if
checkpointing will be asynchronous. If checkpointing is synchronous Checkpointer will respond to the HTTP request only
after the checkpointing completed (un)successfully. On the other hand, Checkpointer will respond to the HTTP request
immediately with a `checkpointIdentifier`, a string which can be used to obtain the result of checkpointing at a later
time.

#### Synchronous checkpointing
To request a synchronous checkpointing which does not delete the Pod, run:
```shell
curl -X POST "http://localhost:8000/checkpoint/default/timer-sleep/timer" --verbose
```
On success, Checkpointer will respond with `HTTP 201 Created` and JSON body similar to:
```json
{
  "containerIdentifier": {
    "namespace": "default",
    "pod": "timer-sleep",
    "container": "timer"
  },
  "beginTimestamp": 1734281060,
  "endTimestamp": 1734281084,
  "containerImageName": "pbaran555/kaniko-checkpointed:138248b8f5936ca3"
}
```
Checkpointer might also respond with `HTTP 404 Not Found` if the container does not exist or
`HTTP 500 Internal Server Error` if there was an error during checkpointing. In this case Checkpointer will
respond with plain text body.


#### Asynchronous checkpointing

To request an asynchronous checkpointing and delete the Pod afterward, run:
```shell
curl "http://localhost:8000/checkpoint/default/timer-sleep/timer" \
--data '{"deletePod": true, "async": true}' \
--verbose
```
Checkpointer will respond with `HTTP 202 Accepted` and a JSON body similar to:
```json
{"checkpointIdentifier":"containerd-control-plane:b2c79a5bd8520ab5"}
```
To get the actual checkpoint result, use the following endpoint.

### Getting checkpoint result

The result of checkpointing can be requested through:
```
HTTP GET /checkpoint?checkpointIdentifier={checkpointIdentifier}
```

For example:
```shell
curl "http://localhost:8000/checkpoint?checkpointIdentifier=containerd-control-plane:b2c79a5bd8520ab5" --verbose
```
Checkpointer will respond with `HTTP 200 OK` and a JSON body equal to the synchronous checkpoint response.
In case checkpointing in the background failed, Checkpointer will respond with `HTTP 500 Internal Server Error`
and a plaintext message. If Checkpointer does not recognize the `checkpointIdentifier` it will
return `HTTP 404 Not Found`.



## Configuration

The following table provides a summary of all environment variables Checkpointer consumes for configuration:

| Name                      | Required | Default                           | Example                       | Description                                                                                                                        |
|---------------------------|----------|-----------------------------------|-------------------------------|------------------------------------------------------------------------------------------------------------------------------------|
| `CHECKPOINT_IMAGE_PREFIX` | Yes      | -                                 | `quay.io/pbaran/checkpointed` | The repository within container registry that Checkpointer will push images to.                                                    |
| `CHECKPOINTER_NODE`       | Yes      | -                                 | `worker-node`                 | Name of the Node that Checkpointer is running on. The value should be set by Kubernetes.                                           |
| `CHECKPOINTER_NODE_IP`    | Yes      | -                                 | `172.16.23.1`                 | IP address of the Node that Checkpointer is running on. The value should be set by Kubernetes.                                     |
| `CHECKPOINTER_PORT`       | No       | `3333`                            | `<---`                        | Port that Checkpointer should listen on.                                                                                           |
| `KUBELET_PORT`            | No       | `10250`                           | `<---`                        | Port that Kubelet listens on.                                                                                                      |
| `CHECKPOINT_BASE_IMAGE`   | No       | `pbaran555/checkpoint-base:1.0.0` | `<---`                        | Image that is used as base for checkpoint container.                                                                               |
| `KANIKO_SECRET_NAME`      | No       | `kaniko-secret`                   | `<---`                        | Name of the Kubernetes Secret with credentials for remote container registry. The secret has to exist in Checkpointer's Namespace. |
| `KANIKO_TIMEOUT`          | No       | `30`                              | `<---`                        | Time in seconds after which Checkpoint will timeout waiting for Kaniko Pod to reach a certain state.                               |
| `STORAGE_BASE_PATH`       | No       | `/checkpointer/storage`           | `<---`                        | Directory where Checkpointer will store checkpoint results needed for asynchronous API.                                            |
| `KANIKO_BUILD_CTX_DIR`    | No       | `/tmp/build-contexts`             | `<---`                        | Directory where Checkpointer will share build context with Kaniko.                                                                 |
| `KUBELET_CERT_FILE`       | No       | `/etc/kubernetes/tls/tls.crt`     | `<---`                        | File path to the tls certificate used for authentication to Kubelet.                                                               |
| `KUBELET_KEY_FILE`        | No       | `/etc/kubernetes/tls/tls.key`     | `<---`                        | File path to the private key used for authentication to Kubelet.                                                                   |
| `KUBELET_ALLOW_INSECURE`  | No       | -                                 | `true`                        | If set to `true`, Checkpointer will not verify Kubelet's TLS certificate.                                                          |
| `DISABLE_ROUTE_FORWARD`   | No       | -                                 | `true`                        | If set to `true`, disables the RoutingProxy. Should only be used in a single-Node cluster.                                         |
| `USE_KANIKO_FS`           | No       | -                                 | `true`                        | If set to `true`, uses the Kaniko File System strategy for checkpointing.                                                          |
| `ENVIRONMENT`             | No       | -                                 | `prod`                        | If set to `prod`, Checkpointer will run in Production mode. Currently just influences the log level and format.                    |

