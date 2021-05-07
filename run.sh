#!/bin/bash

docker run \
        -it --rm \
        -v /pg-utils-web-adapter/backups:/backups \
        --env-file env.list \
        -p "8080:80" \
        pg-utils-web-adapter
