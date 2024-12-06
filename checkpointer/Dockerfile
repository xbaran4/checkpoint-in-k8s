FROM --platform=linux/amd64 golang:1.22-alpine AS builder

WORKDIR /app-source

COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /checkpointer ./cmd/checkpointer

FROM --platform=linux/amd64 alpine:latest

WORKDIR /checkpointer-app

COPY internal/templates/dockerfile.tmpl ./dockerfile.tmpl
COPY --from=builder /checkpointer ./checkpointer

ENV KUBELET_PORT="10250"
ENV KUBELET_CERT_FILE="/etc/kubernetes/pki/apiserver-kubelet-client.crt"
ENV KUBELET_CERT_KEY="/etc/kubernetes/pki/apiserver-kubelet-client.key"

EXPOSE 3333

ENTRYPOINT ["./checkpointer"]