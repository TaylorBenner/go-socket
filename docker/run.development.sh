#!/usr/bin/env bash

. docker.config.sh

docker rm -f $WEBSOCKET_DEVELOPMENT_SERVICE_NAME;
docker rm -f $WEBSOCKET_REDIS_NAME;

docker network create $WEBSOCKET_NETWORK_NAME;

docker run --name=$WEBSOCKET_REDIS_NAME \
    --network=$WEBSOCKET_NETWORK_NAME \
    -d redis;

docker run --name=$WEBSOCKET_DEVELOPMENT_SERVICE_NAME \
    -v `pwd`/../source:/go/src/app \
    -p $WEBSOCKET_DEVELOPMENT_PORT:8080 \
    --network=$WEBSOCKET_NETWORK_NAME \
    -d $WEBSOCKET_DEVELOPMENT_SERVICE_NAME;
