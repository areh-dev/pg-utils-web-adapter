#!/bin/bash

docker run -it --rm -v /pg-utils-web-adapter/backup:/backup --env-file env.list pg-utils-web-adapter

