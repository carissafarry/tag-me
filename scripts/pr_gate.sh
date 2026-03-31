#!/usr/bin/env bash
set -euo pipefail

THRESHOLD="${1:-70}"
API_DIR="apps/api"
COVERAGE_FILE="coverage.out"

echo "==> Running Go tests with coverage..."
cd "$API_DIR"

go test ./... -coverpkg=./... -coverprofile="$COVERAGE_FILE" >/tmp/tagme_go_test.log 2>&1 || {
  echo "STATUS=TEST_FAIL"
  echo "COVERAGE=0"
  echo "==> go test failed"
  cat /tmp/tagme_go_test.log
  exit 1
}

COVERAGE_LINE="$(go tool cover -func="$COVERAGE_FILE" | grep total:)"
COVERAGE_VALUE="$(echo "$COVERAGE_LINE" | awk '{print $3}' | tr -d '%')"

echo "==> Coverage line: $COVERAGE_LINE"
echo "==> Coverage value: $COVERAGE_VALUE%"

awk -v cov="$COVERAGE_VALUE" -v threshold="$THRESHOLD" 'BEGIN {
  if (cov + 0 >= threshold + 0) exit 0;
  exit 1;
}'
RESULT=$?

if [ $RESULT -eq 0 ]; then
  echo "STATUS=PASS"
  echo "COVERAGE=$COVERAGE_VALUE"
  exit 0
else
  echo "STATUS=LOW_COVERAGE"
  echo "COVERAGE=$COVERAGE_VALUE"
  exit 2
fi