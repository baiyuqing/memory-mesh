---
kind: security.mtls
category: security
version: 1.0.0
description: Self-signed mTLS certificate provisioner. Generates CA, server, and client certificates stored as Kubernetes Secrets.
ports:
  - name: tls-cert
    portType: tls-cert
    direction: output
parameters:
  - name: commonName
    type: string
    default: "ottoplus.local"
    required: true
    description: Common name for the CA certificate.
  - name: validityDays
    type: int
    default: "365"
    description: Certificate validity in days.
  - name: keySize
    type: int
    default: "2048"
    description: RSA key size in bits.
  - name: organization
    type: string
    default: "ottoplus"
    description: Organization name for certificates.
requires: []
provides:
  - tls-cert
---

# security.mtls

Provisions self-signed mTLS certificates for secure inter-service communication.
Generates a CA certificate, server certificate, and client certificate, stored
as Kubernetes Secrets.

## Composition Notes

- No inputs required — acts as a root block in the dependency graph.
- Outputs `tls-cert` for consumption by blocks needing TLS (e.g., `networking.ingress`).
- Certificates are self-signed; not suitable for production without a proper CA.

## Kubernetes Resources Created

- Secret: `{cluster}-{name}-ca` (CA certificate + key)
- Secret: `{cluster}-{name}-server` (server certificate + key)
- Secret: `{cluster}-{name}-client` (client certificate + key)
- ConfigMap: `{cluster}-{name}-config` (certificate metadata)
