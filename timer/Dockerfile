FROM golang:1.18-alpine

WORKDIR /app

RUN apk add --no-cache strace

COPY ./timer.go .

CMD ["/bin/sh", "-c", "go run /app/timer.go;sleep infinity"]