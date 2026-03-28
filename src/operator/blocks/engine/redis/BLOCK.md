---
kind: engine.redis
category: engine
version: 1.0.0
description: Redis in-memory data store managed as a Kubernetes StatefulSet. Supports standalone and replica modes.
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
    required: false
parameters:
  - name: version
    type: string
    default: "7.2"
    required: true
    description: Redis major version (6.2, 7.0, 7.2).
  - name: replicas
    type: int
    default: "1"
    required: true
    description: Number of replicas (1 = standalone, >1 = primary + replicas).
  - name: maxMemory
    type: string
    default: "256mb"
    description: maxmemory setting.
  - name: maxMemoryPolicy
    type: string
    default: "allkeys-lru"
    description: "Eviction policy: noeviction, allkeys-lru, volatile-lru, etc."
  - name: persistence
    type: string
    default: "none"
    description: "Persistence mode: none, rdb, aof, rdb+aof."
requires: []
provides:
  - dsn
  - metrics-endpoint
---

# engine.redis

Runs Redis as a StatefulSet. When replicas > 1, replica 0 is primary and
replicas 1..N are read replicas using Redis replication.

## Composition Notes

- Storage input is **optional**. Without it, Redis runs purely in-memory (data lost on restart — fine for caching and local dev).
- With a storage block wired, enables RDB/AOF persistence to disk.
- Wire `dsn` output to proxy or application blocks.
- Wire `metrics` to a monitoring block for redis_exporter scraping.

## Kubernetes Resources Created

- StatefulSet: `{cluster}-{name}` with Redis container
- Service: `{cluster}-{name}` for client connections (port 6379)
- Service (headless): `{cluster}-{name}-headless` for pod DNS
- ConfigMap: `{cluster}-{name}-config` with redis.conf
- Optional: PVC if storage block is wired
