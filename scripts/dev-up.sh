#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
CLUSTER_NAME="ottoplus-dev"

echo "==> Creating k3d cluster: ${CLUSTER_NAME}"
if k3d cluster list | grep -q "${CLUSTER_NAME}"; then
  echo "    Cluster already exists, skipping creation."
else
  k3d cluster create --config "${PROJECT_DIR}/deploy/k3d-config.yaml"
fi

echo "==> Waiting for cluster to be ready..."
kubectl wait --for=condition=Ready nodes --all --timeout=120s

echo "==> Creating ottoplus namespace"
kubectl create namespace ottoplus --dry-run=client -o yaml | kubectl apply -f -

echo "==> Deploying LocalStack"
kubectl apply -f "${PROJECT_DIR}/deploy/localstack/k8s-manifest.yaml"

echo "==> Installing CRDs"
kubectl apply -f "${PROJECT_DIR}/deploy/crds/"

echo "==> Waiting for all components to be ready..."
"${SCRIPT_DIR}/wait-ready.sh"

echo ""
echo "==> Local dev environment is ready!"
echo "    Cluster:    ${CLUSTER_NAME}"
echo "    LocalStack: kubectl port-forward -n localstack svc/localstack 4566:4566"
echo "    CRDs:       kubectl get crd databaseclusters.ottoplus.io"
echo ""
echo "    Try: kubectl apply -f - <<EOF"
echo "    apiVersion: ottoplus.io/v1alpha1"
echo "    kind: DatabaseCluster"
echo "    metadata:"
echo "      name: my-db"
echo "      namespace: ottoplus"
echo "    spec:"
echo "      engine: postgresql"
echo "      replicas: 1"
echo "    EOF"
