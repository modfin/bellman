#!/usr/bin/env bash

go run -race . \
  \
  --openai-key="$(cat ./credentials/openai-api-key.txt)" \
  --anthropic-key="$(cat ./credentials/anthropic-api-key.txt)" \
  --voyageai-key="$(cat ./credentials/voyageai-api-key.txt)" \
  \
  --google-project=modular-finance \
  --google-region=europe-north1 \
  --google-credential="$(cat ./credentials/google-service-account.json)" \
  \
  --ollama-url=http://localhost:11434 \
  \
  --api-key=qwerty \
  --api-key=12345 \
  --rate-limit-config="{\"12345\": {\"burst_tokens\": 200, \"burst_window\": \"20s\", \"sustained_tokens\": 400, \"sustained_window\": \"1m\"}}" \
  --prometheus-metrics-basic-auth="user:pass" \
  --log-format=color \
  --log-level=info