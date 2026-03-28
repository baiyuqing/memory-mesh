---
kind: monitoring.health-dashboard
category: monitoring
version: 1.0.0
description: Lightweight health dashboard aggregating metrics, logs, and events.
ports:
  - name: metrics
    portType: metrics-endpoint
    direction: input
  - name: logs
    portType: log-endpoint
    direction: input
  - name: events
    portType: event-stream
    direction: input
  - name: dashboard-url
    portType: dashboard-url
    direction: output
  - name: http-endpoint
    portType: http-endpoint
    direction: output
parameters:
  - name: refreshInterval
    type: int
    default: "30"
    description: Dashboard refresh interval in seconds.
  - name: title
    type: string
    default: "OttoPlus Health"
    description: Dashboard title.
  - name: port
    type: int
    default: "8080"
    description: Dashboard HTTP port.
requires: []
provides:
  - dashboard-url
  - http-endpoint
---

# monitoring.health-dashboard

Deploys a lightweight status dashboard that aggregates signals from metrics,
log, and event sources into a single pane of glass.

## Composition Notes

- All input ports are **optional** — dashboard degrades gracefully.
- Multi-input aggregation + multi-output pattern.
- Outputs both `dashboard-url` and `http-endpoint` for consumption.

## Kubernetes Resources Created

- Deployment: `{cluster}-{name}` with dashboard container
- Service: `{cluster}-{name}` (configurable port, default 8080)
- ConfigMap: `{cluster}-{name}-config` (dashboard configuration)
