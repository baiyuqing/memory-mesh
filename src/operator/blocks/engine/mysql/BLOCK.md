---
kind: engine.mysql
category: engine
version: 1.0.0
description: MySQL database engine managed as a Kubernetes StatefulSet.
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
    default: "8.0"
    required: true
    description: MySQL major version (5.7, 8.0, 8.4).
  - name: replicas
    type: int
    default: "1"
    required: true
    description: Number of replicas (1 = standalone, >1 = replication).
  - name: maxConnections
    type: int
    default: "151"
    description: max_connections setting.
  - name: innodbBufferPoolSize
    type: string
    default: "128M"
    description: innodb_buffer_pool_size setting.
requires:
  - "storage.*"
provides:
  - dsn
  - metrics-endpoint
---

# engine.mysql

Runs MySQL as a StatefulSet. Same interface pattern as engine.postgresql —
outputs `dsn` and `metrics`, requires `storage` input.

## Composition Notes

- **Must** wire to a storage block.
- **Optionally** wire `dsn` to a proxy block (ProxySQL) for connection pooling.
- **Optionally** wire `metrics` to a monitoring block.
- For replication (replicas > 1), uses MySQL Group Replication or async replication.

## Kubernetes Resources Created

- StatefulSet: `{cluster}-{name}` with MySQL container
- Service: `{cluster}-{name}` for client connections (port 3306)
- Service (headless): `{cluster}-{name}-headless` for pod DNS
- ConfigMap: `{cluster}-{name}-config` with my.cnf
