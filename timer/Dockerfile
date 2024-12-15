FROM --platform=linux/amd64 golang:1.22-alpine

WORKDIR /app

# installing strace and using it can help debug checkpoint/restore failures
# RUN apk add --no-cache strace

COPY ./timer.go .

CMD ["/bin/sh", "-c", "go run /app/timer.go;sleep infinity"]