# src/core/block — Block Abstraction Layer

## Purpose

Defines the fundamental composable unit ("Lego block") of the ottoplus platform. This package is **pure domain logic** — no Kubernetes, no cloud SDKs, no I/O.

## Key Abstractions

| Type | File | Description |
|------|------|-------------|
| `Block` | block.go | Interface: `Descriptor()` + `ValidateParameters()` |
| `Descriptor` | block.go | Machine-readable manifest (kind, ports, parameters, requires, provides) |
| `Port` | block.go | Typed connection point (input/output) with a `PortType` for matching |
| `Registry` | registry.go | In-memory catalog of all known blocks |
| `Composition` | composition.go | A set of `BlockRef`s wired together via `Wire`s |

## How Blocks Compose

1. Each block exposes **Ports** (typed input/output endpoints).
2. A **Wire** connects one block's output port to another's input port when `PortType` matches.
3. **AutoWire** automatically connects unambiguous matches (1 output → 1 input of same type).
4. **TopologicalSort** orders blocks by dependency for sequential reconciliation.
5. **Validate** checks: block exists, params valid, wires type-match, required ports wired.

## Port Types (convention)

| PortType | Meaning |
|----------|---------|
| `dsn` | Database connection string |
| `pvc-spec` | PersistentVolumeClaim specification |
| `metrics-endpoint` | Prometheus-compatible metrics URL |
| `tls-cert` | TLS certificate bundle |
| `s3-path` | S3 bucket path |

## Rules

- No infrastructure imports. This package must compile and test with `go test` alone.
- All types are JSON/YAML serializable for CRD integration.
- Block implementations live in `src/operator/blocks/`, not here.
