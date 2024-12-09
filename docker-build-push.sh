#!/usr/bin/env bash

version=$(git describe | cut -f1,2 -d'-')

docker buildx create --use --name bellman-builder

docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t modfin/bellman:latest \
  -t modfin/bellman:"${version}" \
  --push \
  .

docker buildx rm bellman-builder