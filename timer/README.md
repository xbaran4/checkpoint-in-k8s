# Timer
A simple Go application useful for testing checkpoint/restore functionality in Kubernetes. The application prints
a line to the stdout each second. After 180 seconds the application terminates.

## Dockerfile
The Dockerfile is set up to put the container process to sleep indefinitely, after the application terminates.