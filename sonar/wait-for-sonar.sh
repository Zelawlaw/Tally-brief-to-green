#!/bin/bash
set -euo pipefail

HOST="${1:-http://localhost:9000}"
MAX_RETRIES=60

echo "Waiting for SonarQube at $HOST ..."
for i in $(seq 1 "$MAX_RETRIES"); do
  if curl -s -o /dev/null -w "%{http_code}" "$HOST/api/system/status" | grep -q 200; then
    echo "SonarQube is ready (attempt $i)."
    exit 0
  fi
  printf "."
  sleep 2
done
echo ""
echo "SonarQube did not become ready after $MAX_RETRIES attempts."
exit 1
