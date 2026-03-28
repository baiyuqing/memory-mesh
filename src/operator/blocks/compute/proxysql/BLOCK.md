---
kind: compute.proxysql
category: compute
version: 1.0.0
description: ProxySQL connection pooler and query router for MySQL.
ports:
  - name: upstream-dsn
    portType: dsn
    direction: input
    required: true
  - name: dsn
    portType: dsn
    direction: output
  - name: metrics
    portType: metrics-endpoint
    direction: output
parameters:
  - name: maxConnections
    type: int
    default: "1024"
    description: Maximum frontend connections.
  - name: defaultHostgroup
    type: int
    default: "0"
    description: Default hostgroup for queries.
  - name: monitorUsername
    type: string
    default: "monitor"
    description: Username for backend health checks.
  - name: multiplexing
    type: string
    default: "true"
    description: Enable connection multiplexing.
requires:
  - "datastore.mysql"
provides:
  - dsn
  - metrics-endpoint
---

# compute.proxysql

Runs ProxySQL as a Deployment in front of a MySQL engine block. Provides
connection pooling, query routing, and read/write splitting.

## Composition Notes

- **Must** wire `upstream-dsn` from a MySQL engine block's `dsn` output.
- Outputs its own `dsn` for downstream consumers.
- **Optionally** wire `metrics` to a monitoring block.

## Kubernetes Resources Created

- Deployment: `{cluster}-{name}` with ProxySQL container
- Service: `{cluster}-{name}` (port 6033 for MySQL traffic)
- ConfigMap: `{cluster}-{name}-config` with proxysql.cnf
