#!/usr/bin/env bash
set -euo pipefail

echo "==> Seeding test data..."

# Create a sample Cluster resource
kubectl apply -f - <<EOF
apiVersion: ottoplus.io/v1alpha1
kind: Cluster
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
kind: Cluster
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
echo "    kubectl get oc -n ottoplus"
