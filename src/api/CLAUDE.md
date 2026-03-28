# src/api — Control Plane API

## Purpose

REST/gRPC API server for the DBaaS control plane. Handles user-facing requests for database provisioning, scaling, and lifecycle management.

## Rules

- **Thin layer** — API handlers should validate input and delegate to `core/` for business logic.
- **No business logic here** — This package translates HTTP/gRPC to domain operations and back.
- **OpenAPI spec** — All endpoints must be documented. Auto-generate from annotations where possible.
- **Idempotency** — All mutating endpoints must accept an `Idempotency-Key` header.
- **Structured errors** — Return consistent error responses with codes and messages.

## Key Components

- `server.go` — HTTP/gRPC server setup, middleware, routing.
- `handlers/` — Request handlers grouped by resource (clusters, backups, etc.).
- `middleware/` — Auth, logging, request ID, idempotency key tracking.

## Endpoints (planned)

| Method | Path | Description |
|--------|------|-------------|
| POST   | /v1/clusters | Create a new database cluster |
| GET    | /v1/clusters | List clusters |
| GET    | /v1/clusters/:id | Get cluster details |
| PATCH  | /v1/clusters/:id | Update cluster (scale, config) |
| DELETE | /v1/clusters/:id | Delete cluster |
| GET    | /healthz | Health check |
| GET    | /readyz | Readiness check |

## Conventions

- Handlers accept `(w http.ResponseWriter, r *http.Request)` or gRPC service interfaces.
- Return JSON with `Content-Type: application/json`.
- Log all requests with correlation ID.
