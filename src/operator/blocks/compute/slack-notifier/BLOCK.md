---
kind: compute.slack-notifier
category: compute
version: 1.0.0
description: Slack webhook notifier for cluster events and alerts.
ports:
  - name: event-stream
    portType: event-stream
    direction: input
  - name: metrics
    portType: metrics-endpoint
    direction: input
  - name: http-endpoint
    portType: http-endpoint
    direction: output
parameters:
  - name: webhookSecretName
    type: string
    required: true
    description: Name of the K8s Secret containing the Slack webhook URL.
  - name: channel
    type: string
    default: "#alerts"
    description: Slack channel for notifications.
  - name: severity
    type: string
    default: "warning"
    description: "Minimum severity level: info, warning, critical."
  - name: throttleSeconds
    type: int
    default: "60"
    description: Minimum seconds between notifications.
requires: []
provides:
  - http-endpoint
---

# compute.slack-notifier

Deploys a lightweight notification relay that forwards cluster events and
metric alerts to a Slack channel via webhook.

## Composition Notes

- **Optionally** wire `event-stream` from monitoring blocks.
- **Optionally** wire `metrics` from any block exposing metrics.
- Outputs `http-endpoint` for health checking the notifier itself.
- Fan-in pattern: accepts multiple optional inputs.

## Kubernetes Resources Created

- Deployment: `{cluster}-{name}` with notifier container
- Service: `{cluster}-{name}` (port 8080)
- ConfigMap: `{cluster}-{name}-config` with notification settings
