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
--data '{"containerImageName": "pbaran555/kaniko-checkpointed", "deletePod": "true"}' \
--verbose
```

## Checking checkpoint state
```shell
curl "http://localhost:3333/checkpoint/default/timer-sleep/timer" \
--verbose
```
