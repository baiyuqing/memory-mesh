#!/usr/bin/env bash
set -euo pipefail

echo "==> Seeding test data..."

# Create a sample DatabaseCluster resource
kubectl apply -f - <<EOF
apiVersion: ottoplus.io/v1alpha1
kind: DatabaseCluster
metadata:
  name: seed-pg
  namespace: ottoplus
spec:
  engine: postgresql
  replicas: 1
  version: "16"
  resources:
    cpu: "250m"
    memory: "512Mi"
    storage: "1Gi"
---
apiVersion: ottoplus.io/v1alpha1
kind: DatabaseCluster
metadata:
  name: seed-mysql
  namespace: ottoplus
spec:
  engine: mysql
  replicas: 2
  version: "8.0"
  resources:
    cpu: "500m"
    memory: "1Gi"
    storage: "5Gi"
EOF

echo "==> Seed data created."
echo "    kubectl get dbc -n ottoplus"
