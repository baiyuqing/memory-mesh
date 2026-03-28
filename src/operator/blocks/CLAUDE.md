# src/operator/blocks — Block Runtime Implementations

## Purpose

Contains `BlockRuntime` implementations that bridge domain-level block definitions to Kubernetes resource management. Each subdirectory is one block.

## Structure

```
blocks/
  runtime.go                     # BlockRuntime interface + RuntimeRegistry
  datastore/
    postgresql/                  # BLOCK.md + Go implementation
    mysql/
    redis/
  gateway/
    pgbouncer/
    proxysql/
  storage/
    local-pv/
    ebs/
  observability/
    metrics-exporter/
    log-aggregator/
    health-dashboard/
  security/
    mtls/
    password-rotation/
  networking/
    ingress/
    service-mesh/
  integration/
    s3-backup/
    stripe/
    slack-notifier/
```

## Categories

| Category | What belongs here |
|----------|-------------------|
| `datastore` | Stateful data services — databases, caches, message brokers, search engines |
| `gateway` | Request-path middleware between clients and services — connection poolers, proxies |
| `storage` | Persistent volume provisioning — PVCs, StorageClasses, CSI drivers |
| `observability` | Metrics, logs, traces, dashboards, alerting |
| `security` | Certificates, credentials, secrets management, policy enforcement |
| `networking` | Ingress, service mesh, DNS, load balancing |
| `integration` | Adapters to external services/APIs — backups, webhooks, notifications |

## Adding a New Block

1. Create a directory under the appropriate category: `blocks/<category>/<name>/`
2. Create `BLOCK.md` with YAML frontmatter (machine-readable descriptor) and markdown body (AI-readable context).
3. Implement the `BlockRuntime` interface in a `.go` file.
4. Register the block in the operator's startup code.

## Rules

- Each block's `Reconcile` must be **idempotent** — safe to call repeatedly.
- Each block's `Delete` must clean up all owned K8s resources.
- Use owner references so garbage collection works if the parent CR is deleted.
- Block implementations should use server-side apply for K8s resource management.
- Every `BLOCK.md` must have the YAML frontmatter matching the Go `Descriptor()` return value.
