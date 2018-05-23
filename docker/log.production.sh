#!/usr/bin/env bash

. docker.config.sh
docker logs -f $WEBSOCKET_PRODUCTION_SERVICE_NAME;
