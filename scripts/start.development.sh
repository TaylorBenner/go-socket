#!/usr/bin/env bash

# enable strict mode
set -e;

# set reference directory to our file's working dir
CURRENT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )";

# execute build, run, and log chain
cd $CURRENT_DIR/../docker && \
    ./build.development.sh && \
    ./run.development.sh && \
    ./log.development.sh;
