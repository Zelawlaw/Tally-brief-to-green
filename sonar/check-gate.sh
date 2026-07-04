#!/bin/bash
set -euo pipefail

HOST="${1:-http://localhost:9000}"
PROJECT="${2:-tally}"
AUTH="admin:admin"
MAX_RETRIES=60
SLEEP=3

echo "Polling SonarQube quality gate for project '$PROJECT' ..."
for i in $(seq 1 "$MAX_RETRIES"); do
  RESP=$(curl -s -u "$AUTH" "$HOST/api/qualitygates/project_status?projectKey=$PROJECT")
  STATUS=$(echo "$RESP" | jq -r '.projectStatus.status // "NONE"')

  echo "  [$i/$MAX_RETRIES] status=$STATUS"

  if [ "$STATUS" = "OK" ]; then
    echo ""
    echo "=== Quality Gate PASSED ==="
    exit 0
  elif [ "$STATUS" = "ERROR" ]; then
    echo ""
    echo "=== Quality Gate FAILED ==="
    echo "Failing conditions:"
    echo "$RESP" | jq -r '.projectStatus.conditions[]? | select(.status=="ERROR") | "  \(.metricKey): actual=\(.actualValue) threshold=\(.errorThreshold)"'
    exit 1
  elif [ "$STATUS" = "NONE" ]; then
    echo "  Analysis not yet available. Has the scanner run?"
  fi
  sleep "$SLEEP"
done

echo ""
echo "Timeout waiting for quality gate after $MAX_RETRIES attempts."
exit 1
