---
kind: observability.log-aggregator
category: observability
version: 1.0.0
description: Centralized log aggregation with Loki and Promtail.
ports:
  - name: storage
    portType: pvc-spec
    direction: input
  - name: log-endpoint
    portType: log-endpoint
    direction: output
parameters:
  - name: retentionDays
    type: int
    default: "7"
    description: Log retention period in days.
  - name: lokiVersion
    type: string
    default: "2.9.4"
    description: Loki container image version.
  - name: promtailVersion
    type: string
    default: "2.9.4"
    description: Promtail container image version.
requires: []
provides:
  - log-endpoint
---

# observability.log-aggregator

Deploys Loki (log storage) as a StatefulSet and Promtail (log collector) as a
DaemonSet for centralized log aggregation.

## Composition Notes

- **Optionally** wire `storage` from a storage block for persistent Loki data.
- Outputs `log-endpoint` (Loki push API URL) for consumption by dashboards.
- Most complex block: introduces DaemonSet as a new resource type.

## Kubernetes Resources Created

- StatefulSet: `{cluster}-{name}-loki` with Loki container
- DaemonSet: `{cluster}-{name}-promtail` with Promtail container
- Service: `{cluster}-{name}-loki` (port 3100)
- ConfigMap: `{cluster}-{name}-loki-config` (Loki configuration)
- ConfigMap: `{cluster}-{name}-promtail-config` (Promtail configuration)
- ServiceAccount: `{cluster}-{name}-promtail` (for node log access)
