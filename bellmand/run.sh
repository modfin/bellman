#!/usr/bin/env bash


go run -race bellamnd.go \
  \
  --openai-key="$(cat ./credentials/openai-api-key.txt)" \
  --anthropic-key="$(cat ./credentials/anthropic-api-key.txt)" \
  --voyageai-key="$(cat ./credentials/voyageai-api-key.txt)" \
  \
  --google-project=modular-finance \
  --google-region=europe-north1 \
  --google-credential="$(cat ./credentials/google-service-account.json)" \
  \
  --api-key=qwerty \
  --api-key=12345