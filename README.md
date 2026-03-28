# ottoplus

A composable local development environment platform for AI agents. Uses Kubernetes-native orchestration and a "Lego block" architecture to let AI agents provision, wire, and manage complex infrastructure stacks locally.

## Current Status

**Phase 1: Minimum Viable Loop.** The project has a working API server, a Kubernetes operator, and three registered blocks. The system can validate compositions, auto-wire blocks, and reconcile a Cluster CR end-to-end.

### What Works Today

| Component | Status |
|-----------|--------|
| API server (`cmd/api`) | Builds and runs. Serves block catalog, composition validation, auto-wiring, topology. |
| Operator (`cmd/operator`) | Builds. Watches `Cluster` CRDs, expands shorthand to compositions, reconciles blocks in dependency order. |
| Block: `storage.local-pv` | Outputs PVC spec for engine blocks. No-op reconciliation. |
| Block: `datastore.postgresql` | Creates StatefulSet, ConfigMap, Service, headless Service. |
| Block: `gateway.pgbouncer` | Creates Deployment, ConfigMap, Service. |
| Unit tests | `src/core/block/`, `src/api/`, `src/operator/reconciler/` |

### What Does NOT Work Yet

- The remaining 13 blocks are implemented but **not registered** in the operator. They compile but are not wired into the startup path.
- No integration or e2e tests. The `tests/` directory does not exist yet.
- `src/shared/` directory does not exist yet.
- The operator requires a running k3d cluster with the CRD installed to actually reconcile. Without K8s it will fail to start.
- Dev-only insecure defaults: `POSTGRES_HOST_AUTH_METHOD=trust`, PgBouncer `auth_type=any`. These are intentional for local development but must not be used in any other context.

## Quick Start

### API server only (no Kubernetes required)

```bash
make demo
```

This builds and starts the API server on `:8080`. In another terminal, verify:

```bash
# Health check — expect {"status":"ok"}
curl -s http://localhost:8080/healthz | jq .

# List registered blocks — expect 3 blocks
curl -s http://localhost:8080/v1/blocks | jq '.blocks | length'

# Validate the sample composition — expect {"isValid":true}
curl -s -X POST http://localhost:8080/v1/compositions/validate \
  -H 'Content-Type: application/json' \
  -d @deploy/examples/sample-composition.json | jq .

# Get topology order — expect nodes: [storage, db, pooler]
curl -s -X POST http://localhost:8080/v1/compositions/topology \
  -H 'Content-Type: application/json' \
  -d @deploy/examples/sample-composition.json | jq '.nodes[].name'
```

**Success criteria:**
- `/healthz` returns `{"status":"ok"}`
- `/v1/blocks` returns exactly 3 blocks: `storage.local-pv`, `datastore.postgresql`, `gateway.pgbouncer`
- `/v1/compositions/validate` returns `{"isValid":true}` for the sample composition
- `/v1/compositions/topology` returns nodes in order: `storage`, `db`, `pooler`

### Full stack (requires Docker + k3d + kubectl)

```bash
make dev-up                                       # k3d cluster + LocalStack + CRD
make build                                        # Build api-server and operator binaries
kubectl apply -f deploy/examples/sample-cluster.yaml  # Create sample Cluster CR
./bin/operator                                     # Run operator (separate terminal)
kubectl get clusters.ottoplus.io -n ottoplus       # Check status
```

**Success criteria:**
- `kubectl get clusters.ottoplus.io -n ottoplus` shows `demo-pg` with Phase=Running
- `kubectl get statefulsets -n ottoplus` shows `demo-pg-db`
- `kubectl get deployments -n ottoplus` shows `demo-pg-pooler`
- `kubectl get svc -n ottoplus` shows `demo-pg-db`, `demo-pg-db-headless`, `demo-pg-pooler`

## Architecture

```
┌──────────────────────────────────────────────────────┐
│  Control Plane                                       │
│  ┌──────────┐  ┌────────────────────────────────┐    │
│  │ REST API │  │ K8s Operator                    │    │
│  │ /v1/*    │  │ watches Cluster CRDs            │    │
│  └──────────┘  │ reconciles blocks in topo order │    │
│                └────────────────────────────────┘    │
├──────────────────────────────────────────────────────┤
│  Block Layer (Phase 1: 3 blocks registered)          │
│                                                      │
│  storage.local-pv                                    │
│  datastore.postgresql                                │
│  gateway.pgbouncer                                   │
├──────────────────────────────────────────────────────┤
│  Local Infrastructure                                │
│  k3d (K8s) + LocalStack (S3, SQS, IAM)              │
└──────────────────────────────────────────────────────┘
```

## Project Structure

```
cmd/
  api/             # API server entry point
  operator/        # Operator entry point
src/
  core/            # Domain logic — pure Go, no infra deps
    block/         # Block, Port, Composition, Registry, AutoWire
  api/             # REST API server (handlers, middleware)
  operator/        # K8s operator
    blocks/        # Block runtime implementations
    reconciler/    # Shorthand expansion to Composition
deploy/
  crds/            # Cluster CRD (v1alpha1)
  examples/        # Sample Cluster CR and composition JSON
  k3d-config.yaml
  localstack/      # LocalStack K8s manifests
scripts/           # dev-up, dev-down, seed, wait-ready
```

## Development

```bash
make help          # Show all targets
make build         # Build api-server and operator binaries
make test          # Unit tests (core + api + reconciler)
make demo          # Build and run API server locally
make lint          # go vet + golangci-lint
make fmt           # Format code
make dev-up        # Create k3d cluster + LocalStack
make dev-down      # Tear down
```

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Language | Go 1.24 |
| Orchestration | Kubernetes (k3d for local dev) |
| Cloud Simulation | LocalStack (S3, SQS, IAM) |
| Operator | controller-runtime v0.19.0 |
| API | net/http |
| CRD | ottoplus.io/v1alpha1 |

## License

MIT
