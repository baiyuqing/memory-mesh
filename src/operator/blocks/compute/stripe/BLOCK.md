---
kind: compute.stripe
category: compute
version: 1.0.0
description: Stripe payment API integration block. Deploys a webhook receiver and provides billing event streams.
ports:
  - name: webhook-endpoint
    portType: http-endpoint
    direction: output
  - name: billing-events
    portType: event-stream
    direction: output
  - name: dsn
    portType: dsn
    direction: input
    required: true
parameters:
  - name: apiKeySecret
    type: string
    required: true
    description: "Name of K8s Secret containing Stripe API key (key: STRIPE_API_KEY)."
  - name: webhookSecret
    type: string
    required: true
    description: "Name of K8s Secret containing Stripe webhook signing secret (key: STRIPE_WEBHOOK_SECRET)."
  - name: webhookPath
    type: string
    default: "/webhooks/stripe"
    description: Path for the webhook receiver endpoint.
  - name: replicas
    type: int
    default: "2"
    description: Number of webhook receiver replicas for HA.
  - name: events
    type: string
    default: "invoice.paid,invoice.payment_failed,customer.subscription.updated,customer.subscription.deleted"
    description: Comma-separated list of Stripe event types to subscribe to.
requires: []
provides:
  - http-endpoint
  - event-stream
---

# compute.stripe

Deploys a Stripe webhook receiver as a Kubernetes Deployment. Receives
Stripe events, validates signatures, and stores them in the wired database
for downstream processing.

## Composition Notes

- **Must** wire `dsn` from an engine block — events are persisted to a
  `stripe_events` table for reliable processing.
- Outputs `webhook-endpoint` (the URL to register with Stripe) and
  `billing-events` (a logical event stream other blocks can subscribe to).
- The block creates the `stripe_events` table via an init container that
  runs migrations on startup.

## New Port Types Introduced

| PortType | Meaning |
|----------|---------|
| `http-endpoint` | An HTTP URL that accepts incoming webhooks |
| `event-stream` | A logical event stream (backed by DB polling or message queue) |

## Kubernetes Resources Created

- Deployment: `{cluster}-{name}` with webhook receiver container
- Service: `{cluster}-{name}` (ClusterIP, port 8080)
- ConfigMap: `{cluster}-{name}-config` with event filter and webhook path
- Job (init): `{cluster}-{name}-migrate` runs DB migration on startup

## Security

- Stripe API key and webhook secret are **never** stored in ConfigMap —
  always referenced from existing K8s Secrets.
- Webhook signature validation is mandatory; unverified events are rejected.
- The block validates that the referenced Secrets exist during reconciliation.
