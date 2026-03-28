---
kind: engine.postgresql
category: engine
version: 1.0.0
description: PostgreSQL database engine managed as a Kubernetes StatefulSet.
ports:
  - name: dsn
    portType: dsn
    direction: output
  - name: metrics
    portType: metrics-endpoint
    direction: output
  - name: storage
    portType: pvc-spec
    direction: input
    required: true
parameters:
  - name: version
    type: string
    default: "16"
    required: true
    description: PostgreSQL major version (14, 15, 16).
  - name: replicas
    type: int
    default: "1"
    required: true
    description: Number of replicas (1 = standalone, >1 = streaming replication).
  - name: maxConnections
    type: int
    default: "100"
    description: PostgreSQL max_connections setting.
  - name: sharedBuffers
    type: string
    default: "128MB"
    description: PostgreSQL shared_buffers setting.
requires:
  - "storage.*"
provides:
  - dsn
  - metrics-endpoint
---

# engine.postgresql

Runs PostgreSQL as a StatefulSet. Each replica gets its own PVC via the
wired storage block. Replica 0 is primary; replicas 1..N are streaming
replicas with read-only DSNs.

## Composition Notes

- **Must** be wired to a storage block (local-pv, ebs, or ceph).
- **Optionally** wire the `dsn` output to a proxy block (pgbouncer) for connection pooling.
- **Optionally** wire `metrics` to a monitoring block for Prometheus scraping.

## Kubernetes Resources Created

- StatefulSet: `{cluster}-{name}` with replica count from parameters
- Service (headless): `{cluster}-{name}-headless` for pod DNS
- Service (primary): `{cluster}-{name}` pointing to replica 0
- ConfigMap: `{cluster}-{name}-config` with postgresql.conf
- Secret: `{cluster}-{name}-credentials` with superuser password
