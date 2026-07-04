#!/bin/bash
set -euo pipefail

HOST="${1:-http://localhost:9000}"
PROJECT="${2:-tally}"
AUTH="${3:-admin:admin}"
GATE_NAME="Standard Gate"

echo "Bootstrapping SonarQube at $HOST ..."

# Check if quality gate already exists
EXISTING=$(curl -s -u "$AUTH" "$HOST/api/qualitygates/show?name=$(echo "$GATE_NAME" | jq -sRr @uri)")
GATE_ID=$(echo "$EXISTING" | jq -r '.id // empty')

if [ -n "$GATE_ID" ]; then
  echo "Quality gate '$GATE_NAME' already exists (id=$GATE_ID)."
else
  echo "Creating quality gate '$GATE_NAME' ..."
  GATE_ID=$(curl -s -u "$AUTH" -X POST "$HOST/api/qualitygates/create" \
    -d "name=$GATE_NAME" | jq -r '.id')
  echo "Quality gate created (id=$GATE_ID)."

  curl -s -u "$AUTH" -X POST "$HOST/api/qualitygates/create_condition" \
    -d "gateId=$GATE_ID" -d "metric=coverage" -d "op=LT" -d "error=100" > /dev/null
  echo "  + coverage < 100% = ERROR"

  curl -s -u "$AUTH" -X POST "$HOST/api/qualitygates/create_condition" \
    -d "gateId=$GATE_ID" -d "metric=code_smells" -d "op=GT" -d "error=0" > /dev/null
  echo "  + code_smells > 0 = ERROR"

  curl -s -u "$AUTH" -X POST "$HOST/api/qualitygates/create_condition" \
    -d "gateId=$GATE_ID" -d "metric=duplicated_lines_density" -d "op=GT" -d "error=0" > /dev/null
  echo "  + duplicated_lines_density > 0 = ERROR"

  curl -s -u "$AUTH" -X POST "$HOST/api/qualitygates/create_condition" \
    -d "gateId=$GATE_ID" -d "metric=bugs" -d "op=GT" -d "error=0" > /dev/null
  echo "  + bugs > 0 = ERROR"

  curl -s -u "$AUTH" -X POST "$HOST/api/qualitygates/create_condition" \
    -d "gateId=$GATE_ID" -d "metric=vulnerabilities" -d "op=GT" -d "error=0" > /dev/null
  echo "  + vulnerabilities > 0 = ERROR"

  curl -s -u "$AUTH" -X POST "$HOST/api/qualitygates/create_condition" \
    -d "gateId=$GATE_ID" -d "metric=security_hotspots_reviewed" -d "op=LT" -d "error=100" > /dev/null
  echo "  + security_hotspots_reviewed < 100% = ERROR"

  curl -s -u "$AUTH" -X POST "$HOST/api/qualitygates/create_condition" \
    -d "gateId=$GATE_ID" -d "metric=sqale_rating" -d "op=GT" -d "error=1" > /dev/null
  echo "  + maintainability_rating > A = ERROR"

  curl -s -u "$AUTH" -X POST "$HOST/api/qualitygates/create_condition" \
    -d "gateId=$GATE_ID" -d "metric=security_rating" -d "op=GT" -d "error=1" > /dev/null
  echo "  + security_rating > A = ERROR"

  curl -s -u "$AUTH" -X POST "$HOST/api/qualitygates/create_condition" \
    -d "gateId=$GATE_ID" -d "metric=reliability_rating" -d "op=GT" -d "error=1" > /dev/null
  echo "  + reliability_rating > A = ERROR"
fi

# Set as default
curl -s -u "$AUTH" -X POST "$HOST/api/qualitygates/set_as_default" \
  -d "gateName=$GATE_NAME" > /dev/null
echo "Set as default quality gate."

# Associate with project
curl -s -u "$AUTH" -X POST "$HOST/api/qualitygates/select" \
  -d "gateName=$GATE_NAME&projectKey=$PROJECT" > /dev/null
echo "Associated with project '$PROJECT'."

# Create project if it doesn't exist
PROJ_EXISTS=$(curl -s -u "$AUTH" "$HOST/api/projects/search?projects=$PROJECT" | jq -r '.components | length')
if [ "$PROJ_EXISTS" -eq 0 ]; then
  curl -s -u "$AUTH" -X POST "$HOST/api/projects/create" \
    -d "project=$PROJECT" -d "name=Tally" > /dev/null
  echo "Project '$PROJECT' created."
else
  echo "Project '$PROJECT' already exists."
fi

# Generate a CI token for the scanner
TOKEN=$(curl -s -u "$AUTH" -X POST "$HOST/api/user_tokens/generate" \
  -d "name=tally-ci" | jq -r '.token // empty')
if [ -n "$TOKEN" ]; then
  echo ""
  echo "=== Tally CI token (set this in your environment) ==="
  echo "export SONAR_TOKEN=$TOKEN"
else
  echo ""
  echo "NOTE: Could not auto-generate token (password may have been changed)."
  echo "Generate one manually at $HOST (My Account → Security → Generate Token)"
  echo "Then set: export SONAR_TOKEN=<token>"
fi

echo ""
echo "Bootstrap complete."
