#!/usr/bin/env bash


docker buildx create --use --name bellman-builder

docker buildx build --platform linux/amd64,linux/arm64 -t modfin/bellman:latest --push .

docker buildx rm bellman-builder