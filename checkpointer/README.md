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
curl "http://localhost:3333/checkpoint" \
--request POST \
--header "Content-Type: application/json" \
--data '{"containerPath": "default/timer-sleep/timer", "containerImageName": "pbaran555/kaniko-checkpointed"}' \
--verbose
```

## Checking checkpoint state
```shell
curl "http://localhost:3333/checkpoint?containerPath=default/timer-sleep/timer" \
--verbose
```
