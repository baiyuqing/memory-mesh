# src/core — Domain Logic

## Purpose

Core business logic for the ottoplus platform. This package defines domain models, validation rules, and business operations that are **independent of any infrastructure**.

## Rules

- **No external dependencies** — Do not import Kubernetes, AWS SDK, HTTP frameworks, or database drivers here.
- **No side effects** — Functions should be pure where possible. No network calls, no file I/O.
- **Testable in isolation** — All code here must be testable with `go test` alone, no Docker or K8s required.

## Key Concepts

- `Cluster` — Domain model representing a composable infrastructure environment (blocks, wires, parameters).
- `Engine` — Pluggable interface for database engine types (PostgreSQL, MySQL, Redis, etc.).
- Validation logic — Spec validation, resource limits, version compatibility checks.
- State machine — Cluster lifecycle phases: Pending → Provisioning → Running → Updating → Failed → Deleting.

## Conventions

- One type per file where practical.
- File naming: `database-cluster.go`, `engine.go`, `validation.go`.
- Tests: `database-cluster_test.go`, etc.
