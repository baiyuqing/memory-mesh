# src/core — Domain Logic

## Purpose

Core business logic for the DBaaS platform. This package defines domain models, validation rules, and business operations that are **independent of any infrastructure**.

## Rules

- **No external dependencies** — Do not import Kubernetes, AWS SDK, HTTP frameworks, or database drivers here.
- **No side effects** — Functions should be pure where possible. No network calls, no file I/O.
- **Testable in isolation** — All code here must be testable with `go test` alone, no Docker or K8s required.

## Key Concepts

- `DatabaseCluster` — Domain model representing a managed database instance (engine, replicas, version, resources, config).
- `Engine` — Pluggable interface for database engine types (PostgreSQL, MySQL, Redis, etc.).
- Validation logic — Spec validation, resource limits, version compatibility checks.
- State machine — Cluster lifecycle phases: Pending → Provisioning → Running → Updating → Failed → Deleting.

## Conventions

- One type per file where practical.
- File naming: `database-cluster.go`, `engine.go`, `validation.go`.
- Tests: `database-cluster_test.go`, etc.
