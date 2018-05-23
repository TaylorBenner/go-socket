#!/usr/bin/env bash

. docker.config.sh

# First compile our app inside source
docker run --rm \
  -v `pwd`/../source:/go/src/app \
  -w /go/src/app \
  $WEBSOCKET_DEVELOPMENT_SERVICE_NAME \
  /bin/bash -c "CGO_ENABLED=0 GOOS=linux \
    go build -a -installsuffix cgo \
    && cp /etc/ssl/certs/ca-certificates.crt .";

# Move the compiled app and Linux VM SSL Certs
# to current context
mv ../source/app . \
  && mv ../source/ca-certificates.crt .;

# Build slim production image
docker build \
  -t $WEBSOCKET_PRODUCTION_SERVICE_NAME \
  -f dockerfile.production \
  .;

# Cleanup build files
rm -f app \
  ca-certificates.crt;
