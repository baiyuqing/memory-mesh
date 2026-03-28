---
kind: gateway.pgbouncer
category: gateway
version: 1.0.0
description: PgBouncer connection pooler for PostgreSQL.
ports:
  - name: upstream-dsn
    portType: dsn
    direction: input
    required: true
  - name: upstream-credential
    portType: credential
    direction: input
  - name: dsn
    portType: dsn
    direction: output
  - name: metrics
    portType: metrics-endpoint
    direction: output
parameters:
  - name: poolMode
    type: string
    default: "transaction"
    description: "Pool mode: session, transaction, or statement."
  - name: maxClientConnections
    type: int
    default: "500"
    description: Maximum number of client connections.
  - name: defaultPoolSize
    type: int
    default: "20"
    description: Default pool size per user/database pair.
requires:
  - "datastore.postgresql"
provides:
  - dsn
  - metrics-endpoint
---

# gateway.pgbouncer

Runs PgBouncer as a Deployment in front of a PostgreSQL engine block.
Provides connection pooling to reduce backend connection pressure.

## Composition Notes

- **Must** wire `upstream-dsn` from a PostgreSQL engine block's `dsn` output.
- Outputs its own `dsn` — downstream consumers connect through PgBouncer instead of directly.
- **Optionally** wire `metrics` to a monitoring block.

## Kubernetes Resources Created

- Deployment: `{cluster}-{name}` with PgBouncer container
- Service: `{cluster}-{name}` for client connections
- ConfigMap: `{cluster}-{name}-config` with pgbouncer.ini
- Secret: `{cluster}-{name}-userlist` with auth credentials
