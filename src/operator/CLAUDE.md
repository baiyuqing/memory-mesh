# src/operator — Kubernetes Operator

## Purpose

Kubernetes operator that watches `DatabaseCluster` CRDs and reconciles the desired state with actual infrastructure. This is the core automation engine of the data plane.

## Rules

- **Reconciliation loop must be idempotent** — Running reconcile multiple times with the same input must produce the same result.
- **Use controller-runtime** — Built on `sigs.k8s.io/controller-runtime` for the reconciler framework.
- **Status updates** — Always update `.status` fields to reflect current state after reconciliation.
- **No direct cloud calls in reconciler** — Use interfaces defined in `core/` for cloud operations (S3 backups, etc.). Implementations live in `shared/` or dedicated adapter packages.

## Key Components

- `controller.go` — Main reconciler implementing `reconcile.Reconciler`.
- `manager.go` — Operator manager setup, scheme registration, leader election.
- `predicates.go` — Event filters (ignore status-only updates, etc.).

## Reconciliation Flow

```
Watch DatabaseCluster CR
  → Validate spec (delegate to core/)
  → Determine desired state (StatefulSet, Service, ConfigMap)
  → Compare with actual state in cluster
  → Create/Update/Delete K8s resources as needed
  → Update DatabaseCluster status
```

## Conventions

- One controller per CRD.
- Use `ctrl.Log` for structured logging.
- Requeue with backoff on transient errors, fail fast on permanent errors.
