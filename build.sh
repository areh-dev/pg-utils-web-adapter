#!/bin/bash

# remove previous version of container
docker image rm pg-utils-web-adapter

# build new
docker build -t pg-utils-web-adapter .

# prune unused images
docker image prune -f
