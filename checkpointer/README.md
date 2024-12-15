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
(hex encoded bytes) tag suffix, e.g.: `pbaran555/kaniko-checkpointed:618c816e64b57705`

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


## Making a checkpoint request
```shell
curl "http://localhost:3333/checkpoint/default/timer-sleep/timer" \
--header "Content-Type: application/json" \
--data '{"deletePod": true}' \
--verbose
```

## Checking checkpoint state
```shell
curl "http://localhost:3333/checkpoint?checkpointIdentifier=..." --verbose
```

## Configuration

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

