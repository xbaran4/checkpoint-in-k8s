FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /main

FROM alpine:latest

WORKDIR /

RUN apk update && \
    apk add --no-cache \
    buildah

COPY --from=builder /main /main

ENV KUBELET_PORT="10250"
ENV KUBELET_CERT_FILE="/etc/kubernetes/pki/apiserver-kubelet-client.crt"
ENV KUBELET_CERT_KEY="/etc/kubernetes/pki/apiserver-kubelet-client.key"

ENV CHECKPOINT_ANNOTATION="org.criu.checkpoint.container.name"

ENV REGISTRY_USERNAME="user"
ENV REGISTRY_PASSWORD="pass"

EXPOSE 3333

ENTRYPOINT ["/main"]