#!/usr/bin/env bash

semantic=$(git describe | cut -f1 -d'-')
version=$(git describe | cut -f1,2 -d'-')

BUILDER=$(docker buildx create) || exit 1

docker buildx build \
  --builder "${BUILDER}" \
  --platform linux/amd64,linux/arm64 \
  -f ./Dockerfile \
  -t modfin/bellman:latest \
  -t modfin/bellman:"${semantic}" \
  -t modfin/bellman:"${version}" \
  --push \
  .

docker buildx rm "${BUILDER}"