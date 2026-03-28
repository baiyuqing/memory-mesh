---
kind: networking.ingress
category: networking
version: 1.0.0
description: Kubernetes Ingress for external HTTP access with optional TLS.
ports:
  - name: upstream-http
    portType: http-endpoint
    direction: input
    required: true
  - name: tls-cert
    portType: tls-cert
    direction: input
  - name: ingress-url
    portType: ingress-url
    direction: output
parameters:
  - name: host
    type: string
    required: true
    description: Hostname for the Ingress rule.
  - name: path
    type: string
    default: "/"
    description: URL path prefix.
  - name: ingressClassName
    type: string
    default: "nginx"
    description: Ingress class name.
  - name: tlsEnabled
    type: string
    default: "false"
    description: Enable TLS termination.
requires:
  - "datastore.*"
provides:
  - ingress-url
---

# networking.ingress

Creates a Kubernetes Ingress resource for external HTTP access to services.
Supports optional TLS termination via the `tls-cert` input port.

## Composition Notes

- **Must** wire `upstream-http` from a block providing `http-endpoint`.
- **Optionally** wire `tls-cert` from `security.mtls` for TLS termination.
- Outputs `ingress-url` with the externally-accessible URL.

## Kubernetes Resources Created

- Ingress: `{cluster}-{name}` with host-based routing
