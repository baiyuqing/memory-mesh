# ottoplus

A Database-as-a-Service (DBaaS) platform with a Kubernetes-native control plane and composable "Lego block" architecture. Designed for AI-agent-friendly local development.

## Architecture

```
┌──────────────────────────────────────────────────────┐
│  Control Plane                                       │
│  ┌──────────┐  ┌────────────────────────────────┐    │
│  │ REST API │  │ K8s Operator                    │    │
│  │ /v1/*    │  │ watches DatabaseCluster CRDs    │    │
│  └──────────┘  │ reconciles blocks in topo order │    │
│                └────────────────────────────────┘    │
├──────────────────────────────────────────────────────┤
│  Block Layer (composable, pluggable)                 │
│                                                      │
│  storage ─── engine ─── proxy ─── monitoring         │
│  local-pv    postgresql  pgbouncer  metrics-exporter │
│  ebs         mysql       proxysql   log-aggregator   │
│              redis                  health-dashboard  │
│                                                      │
│  auth ────── backup ──── networking  integration     │
│  mtls        s3-backup   ingress     stripe          │
│  password-               service-    slack-notifier  │
│  rotation                mesh                        │
├──────────────────────────────────────────────────────┤
│  Infrastructure                                      │
│  k3d (local K8s) + LocalStack (S3, SQS, IAM)        │
└──────────────────────────────────────────────────────┘
```

## Quick Start

```bash
# Prerequisites: docker, k3d, kubectl

# Start local environment (k3d cluster + LocalStack)
make dev-up

# Seed test data
make seed

# Run unit tests
make test

# Tear down
make dev-down
```

## Composable Block System

Blocks are self-contained units that wire together via typed ports. A database cluster is a composition of blocks:

```yaml
apiVersion: ottoplus.io/v1alpha1
kind: DatabaseCluster
metadata:
  name: my-app-db
spec:
  blocks:
    composition:
      - kind: storage.local-pv
        name: storage
        parameters:
          size: "10Gi"

      - kind: engine.postgresql
        name: db
        parameters:
          version: "16"
          replicas: "3"
        inputs:
          storage: storage/pvc-spec         # inline dependency

      - kind: proxy.pgbouncer
        name: pooler
        parameters:
          poolMode: transaction
        inputs:
          upstream-dsn: db/dsn              # pooler depends on db

      - kind: monitoring.metrics-exporter
        name: metrics
        inputs:
          metrics-input: db/metrics         # auto-scraped
```

Or use the shorthand for common setups:

```yaml
spec:
  engine: postgresql
  version: "16"
  replicas: 3
  storage: "10Gi"
  backup:
    enabled: true
    schedule: "0 2 * * *"
    destination: "s3://backups/my-app-db"
```

### Port Types

Blocks connect through typed ports. A wire is valid only when port types match.

| Port Type | Description |
|-----------|-------------|
| `dsn` | Database connection string |
| `pvc-spec` | PersistentVolumeClaim specification |
| `metrics-endpoint` | Prometheus-compatible metrics URL |
| `http-endpoint` | HTTP service endpoint |
| `event-stream` | Event stream URL |
| `tls-cert` | TLS certificate bundle |
| `credential` | Database credential reference |
| `log-endpoint` | Log collection URL |
| `ingress-url` | Externally-accessible URL |
| `dashboard-url` | Web UI dashboard URL |

### Blocks (16)

| Category | Block | Description |
|----------|-------|-------------|
| engine | `engine.postgresql` | PostgreSQL via StatefulSet |
| engine | `engine.mysql` | MySQL via StatefulSet |
| engine | `engine.redis` | Redis with optional persistence |
| proxy | `proxy.pgbouncer` | Connection pooler for PostgreSQL |
| proxy | `proxy.proxysql` | Connection pooler for MySQL |
| storage | `storage.local-pv` | Local PersistentVolume |
| storage | `storage.ebs` | AWS EBS via StorageClass |
| backup | `backup.s3-backup` | S3 backups via CronJob |
| monitoring | `monitoring.metrics-exporter` | Prometheus scrape config |
| monitoring | `monitoring.log-aggregator` | Loki + Promtail |
| monitoring | `monitoring.health-dashboard` | Status dashboard |
| auth | `auth.mtls` | Self-signed mTLS certificates |
| auth | `auth.password-rotation` | Credential rotation via CronJob |
| networking | `networking.ingress` | K8s Ingress with optional TLS |
| integration | `integration.stripe` | Stripe webhook receiver |
| integration | `integration.slack-notifier` | Slack alert notifications |

Each block has a `BLOCK.md` with YAML frontmatter (machine-readable descriptor) and markdown body (AI-readable context).

## API

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/v1/blocks` | List all registered blocks |
| GET | `/v1/blocks/{kind}` | Get block descriptor |
| POST | `/v1/compositions/validate` | Validate a composition |
| POST | `/v1/compositions/auto-wire` | Auto-wire unambiguous port matches |
| POST | `/v1/compositions/topology` | Get topological sort + dependency graph |
| GET | `/healthz` | Health check |

## Project Structure

```
src/
  core/           # Domain logic — no infra deps, pure Go
    block/        # Block, Port, Composition, Registry, AutoWire
  api/            # REST API server
  operator/       # K8s operator + block runtime implementations
    blocks/       # 16 block implementations (each with BLOCK.md)
    reconciler/   # Composition expansion (shorthand → explicit)
deploy/
  crds/           # DatabaseCluster CRD
  k3d-config.yaml
  localstack/     # LocalStack K8s manifests
scripts/          # dev-up, dev-down, seed, wait-ready
```

## Development

```bash
make help          # Show all targets
make test          # Unit tests (no infra needed)
make lint          # go vet + golangci-lint
make build         # Build binaries
make fmt           # Format code
```

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Language | Go |
| Orchestration | Kubernetes (k3d for local dev) |
| Cloud Simulation | LocalStack (S3, SQS, IAM) |
| Operator | controller-runtime v0.19.0 |
| API | net/http with ServeMux |
| CRD | `DatabaseCluster` (ottoplus.io/v1alpha1) |

## License

MIT
