#!/usr/bin/env bash

. docker.config.sh
docker logs -f $WEBSOCKET_DEVELOPMENT_SERVICE_NAME;
