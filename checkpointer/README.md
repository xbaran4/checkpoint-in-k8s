# Checkpointer microservice

## Build
```shell
docker build -t pbaran555/checkpointer .
```

## Push
```shell
docker push pbaran555/checkpointer
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
curl "http://localhost:3333/checkpoint?checkpointIdentifier=..." \
--verbose
```

## Configuration

| Name                      | Required | Default                           | Example                       | Description                                                                                                                        |
|---------------------------|----------|-----------------------------------|-------------------------------|------------------------------------------------------------------------------------------------------------------------------------|
| `CHECKPOINT_IMAGE_PREFIX` | Yes      | -                                 | `quay.io/pbaran/checkpointed` | The repository within container registry that Checkpointer will push images to.                                                    |
| `CHECKPOINTER_NAMESPACE`  | Yes      | -                                 | `kube-system`                 | Kubernetes Namespace that Checkpointer is running in. The value should be set by Kubernetes.                                       |
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

