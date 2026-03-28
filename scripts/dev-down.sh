#!/usr/bin/env bash
set -euo pipefail

CLUSTER_NAME="ottoplus-dev"

echo "==> Deleting k3d cluster: ${CLUSTER_NAME}"
if k3d cluster list | grep -q "${CLUSTER_NAME}"; then
  k3d cluster delete "${CLUSTER_NAME}"
  echo "    Cluster deleted."
else
  echo "    Cluster not found, nothing to delete."
fi

echo "==> Local dev environment torn down."
