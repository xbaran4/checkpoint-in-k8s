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

# This env should match the location where the template file is copied to withing the WORKDIR.
ENV DOCKERFILE_TMPL_FILE="dockerfile.tmpl"

EXPOSE 3333

ENTRYPOINT ["./checkpointer"]