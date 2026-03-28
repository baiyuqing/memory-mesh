# ottoplus

A composable local development environment platform for AI agents. Uses Kubernetes-native orchestration and a "Lego block" architecture to let AI agents provision, wire, and manage complex infrastructure stacks locally.

## The Problem

AI agents writing code need local infrastructure — databases, caches, message queues, monitoring, auth, networking. Setting up these environments is manual, fragile, and hard for agents to reason about. ottoplus makes it composable and machine-readable so agents can self-serve.

## How It Works

Infrastructure components are **blocks** that connect through typed **ports**. An agent describes what it needs as a composition, and the operator reconciles the actual Kubernetes resources.

```yaml
apiVersion: ottoplus.io/v1alpha1
kind: Cluster
metadata:
  name: my-app
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
          storage: storage/pvc-spec

      - kind: proxy.pgbouncer
        name: pooler
        inputs:
          upstream-dsn: db/dsn

      - kind: monitoring.metrics-exporter
        name: metrics
        inputs:
          metrics-input: db/metrics
```

The `inputs` field makes the dependency graph readable inline — no need to cross-reference a separate wiring section. Blocks auto-wire when port types match unambiguously.

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
│  Block Layer (composable, pluggable)                 │
│                                                      │
│  storage     engine       proxy        monitoring    │
│  local-pv    postgresql   pgbouncer    metrics       │
│  ebs         mysql        proxysql     log-aggregator│
│              redis                     dashboard     │
│                                                      │
│  auth        backup       networking   integration   │
│  mtls        s3-backup    ingress      stripe        │
│  password-                service-mesh slack-notifier │
│  rotation                                            │
├──────────────────────────────────────────────────────┤
│  Local Infrastructure                                │
│  k3d (K8s) + LocalStack (S3, SQS, IAM)              │
└──────────────────────────────────────────────────────┘
```

## Quick Start

```bash
# Prerequisites: docker, k3d, kubectl
make dev-up     # k3d cluster + LocalStack, one command
make seed       # Seed example compositions
make test       # Unit tests (no infra needed)
make dev-down   # Tear down
```

## Blocks

Each block is a self-contained unit with:
- A `Descriptor` (kind, typed ports, parameters, requires/provides)
- A `BLOCK.md` with YAML frontmatter (machine-readable) + markdown body (AI-readable)
- A `BlockRuntime` that reconciles Kubernetes resources

### Available Blocks (16)

| Category | Block | What It Provisions |
|----------|-------|--------------------|
| engine | `engine.postgresql` | PostgreSQL StatefulSet + Services |
| engine | `engine.mysql` | MySQL StatefulSet + Services |
| engine | `engine.redis` | Redis with optional persistence |
| proxy | `proxy.pgbouncer` | PostgreSQL connection pooler |
| proxy | `proxy.proxysql` | MySQL connection pooler |
| storage | `storage.local-pv` | Local PersistentVolume |
| storage | `storage.ebs` | AWS EBS StorageClass |
| backup | `backup.s3-backup` | S3 backups via CronJob |
| monitoring | `monitoring.metrics-exporter` | Prometheus scrape config |
| monitoring | `monitoring.log-aggregator` | Loki + Promtail |
| monitoring | `monitoring.health-dashboard` | Status dashboard |
| auth | `auth.mtls` | Self-signed mTLS certificates |
| auth | `auth.password-rotation` | Credential rotation CronJob |
| networking | `networking.ingress` | K8s Ingress with optional TLS |
| integration | `integration.stripe` | Stripe webhook receiver |
| integration | `integration.slack-notifier` | Slack alert notifications |

### Port Types

Blocks connect through typed ports. A wire is valid only when port types match.

| Port Type | Meaning |
|-----------|---------|
| `dsn` | Connection string |
| `pvc-spec` | Storage claim specification |
| `metrics-endpoint` | Prometheus-compatible metrics URL |
| `http-endpoint` | HTTP service endpoint |
| `event-stream` | Event stream URL |
| `tls-cert` | TLS certificate bundle |
| `credential` | Credential reference |
| `log-endpoint` | Log collection URL |
| `ingress-url` | Externally-accessible URL |
| `dashboard-url` | Web UI dashboard URL |

### Adding a Block

1. Create `src/operator/blocks/<category>/<name>/`
2. Write `BLOCK.md` with YAML frontmatter descriptor
3. Implement the `BlockRuntime` interface in Go
4. Register in the operator startup

## API

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/v1/blocks` | List all registered blocks |
| GET | `/v1/blocks/{kind}` | Get block descriptor |
| POST | `/v1/compositions/validate` | Validate a composition |
| POST | `/v1/compositions/auto-wire` | Auto-wire unambiguous port matches |
| POST | `/v1/compositions/topology` | Get dependency graph (for visual editors) |
| GET | `/healthz` | Health check |

## AI-Agent-Friendly Design

- **Self-describing**: `CLAUDE.md` per module, `BLOCK.md` per block with machine-readable frontmatter
- **Discoverable**: `make help`, REST API for block catalog, topology endpoint for dependency graphs
- **Idempotent**: Every operation is safe to retry — API calls, reconciliation, scripts
- **Fast feedback**: Unit tests run without infrastructure, integration tests under 60s
- **Composable**: Agents describe what they need declaratively, the platform handles the rest

## Project Structure

```
src/
  core/           # Domain logic — pure Go, no infra deps
    block/        # Block, Port, Composition, Registry, AutoWire
  api/            # REST API server
  operator/       # K8s operator + block runtime implementations
    blocks/       # 16 block implementations (each with BLOCK.md)
    reconciler/   # Composition expansion
deploy/
  crds/           # Cluster CRD
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
| API | net/http |
| CRD | ottoplus.io/v1alpha1 |

## License

MIT
