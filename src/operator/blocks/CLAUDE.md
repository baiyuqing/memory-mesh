# src/operator/blocks — Block Runtime Implementations

## Purpose

Contains `BlockRuntime` implementations that bridge domain-level block definitions to Kubernetes resource management. Each subdirectory is one block.

## Structure

```
blocks/
  runtime.go                     # BlockRuntime interface + RuntimeRegistry
  engine/
    postgresql/                  # BLOCK.md + Go implementation
    mysql/
    redis/
  proxy/
    pgbouncer/
    proxysql/
  backup/
    s3-backup/
  monitoring/
    metrics-exporter/
    log-aggregator/
    health-dashboard/
  auth/
    password-rotation/
    mtls/
  storage/
    local-pv/
    ebs/
  integration/
    stripe/
    slack-notifier/
  networking/
    ingress/
    service-mesh/
```

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
