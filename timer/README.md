# Timer
A simple Go application useful for testing checkpoint/restore functionality in Kubernetes. The application prints
a line to the stdout each second. After 180 seconds the application terminates.

## Dockerfile
The Dockerfile is set up to put the container process to sleep indefinitely, after the application terminates.

## Usage
A pre-built container image is publicly available as `pbaran555/timer:1.0.0`.

To build a custom container image run:
```shell
docker build -t pbaran555/timer:1.0.0 . # replace with custom image name
```

To push the container image to a remote registry, run:
```shell
docker push pbaran555/timer:1.0.0 # replace with custom image name
```

Create a Kubernetes Pod with the Timer container by:
```shell
kubectl apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: timer-sleep
  namespace: default
spec:
  containers:
  - name: timer
    image: pbaran555/timer:1.0.0
EOF
```

The logs should show:
```
Seconds from start: 1
Seconds from start: 2
Seconds from start: 3
.
.
.
Seconds from start: 180
Exiting after 180 seconds
```

See [Checkpointer's README.md](checkpointer/README.md) how you can checkpoint the container.

You can then create Pod from checkpointed container image
```shell
kubectl apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: timer-sleep-checkpointed
  namespace: default
spec:
  containers:
  - name: timer
    image: pbaran555/kaniko-checkpointed:6e18d3f20ce35e2d
EOF
```