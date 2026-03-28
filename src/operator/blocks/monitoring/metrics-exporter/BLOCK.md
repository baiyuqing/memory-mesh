---
kind: monitoring.metrics-exporter
category: monitoring
version: 1.0.0
description: Prometheus metrics exporter sidecar for database engines.
ports:
  - name: scrape-target
    portType: metrics-endpoint
    direction: input
    required: true
parameters:
  - name: port
    type: int
    default: "9187"
    description: Metrics exporter listen port.
  - name: scrapeInterval
    type: string
    default: "15s"
    description: Prometheus scrape interval annotation.
requires:
  - "engine.*"
provides: []
---

# monitoring.metrics-exporter

Deploys a Prometheus exporter that scrapes metrics from the wired engine
block. Adds Prometheus annotations to the engine pods for auto-discovery.

## Composition Notes

- **Must** wire `scrape-target` from an engine block's `metrics` output.
- Adds `prometheus.io/scrape`, `prometheus.io/port` annotations to target pods.

## Kubernetes Resources Created

- ServiceMonitor: `{cluster}-{name}` (if Prometheus Operator is installed)
- Alternatively, pod annotations for Prometheus auto-discovery
