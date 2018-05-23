#!/usr/bin/env bash

. docker.config.sh

docker rm -f $WEBSOCKET_PRODUCTION_SERVICE_NAME;
docker rm -f $WEBSOCKET_REDIS_NAME;

docker network create $WEBSOCKET_NETWORK_NAME;

docker run --name=$WEBSOCKET_REDIS_NAME \
    --network=$WEBSOCKET_NETWORK_NAME \
    -d redis;

docker run --name=$WEBSOCKET_PRODUCTION_SERVICE_NAME \
    -p $WEBSOCKET_PRODUCTION_PORT:8080 \
    --network=$WEBSOCKET_NETWORK_NAME \
    -d $WEBSOCKET_PRODUCTION_SERVICE_NAME;
