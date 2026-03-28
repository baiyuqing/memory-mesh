#!/usr/bin/env bash
set -euo pipefail

TIMEOUT=${1:-120}
INTERVAL=5
ELAPSED=0

echo "    Waiting up to ${TIMEOUT}s for all components..."

# Wait for LocalStack
echo -n "    LocalStack: "
until kubectl -n localstack get pods -l app=localstack -o jsonpath='{.items[0].status.conditions[?(@.type=="Ready")].status}' 2>/dev/null | grep -q "True"; do
  if [ "$ELAPSED" -ge "$TIMEOUT" ]; then
    echo "TIMEOUT"
    echo "ERROR: LocalStack did not become ready within ${TIMEOUT}s"
    exit 1
  fi
  sleep "$INTERVAL"
  ELAPSED=$((ELAPSED + INTERVAL))
  echo -n "."
done
echo " Ready (${ELAPSED}s)"

# Verify CRD is installed
echo -n "    CRD:        "
if kubectl get crd clusters.ottoplus.io >/dev/null 2>&1; then
  echo " Installed"
else
  echo " MISSING"
  exit 1
fi

echo "    All components ready."
