#!/usr/bin/env bash

. docker.config.sh

docker build \
  -t $WEBSOCKET_DEVELOPMENT_SERVICE_NAME \
  -f dockerfile.development \
  .
