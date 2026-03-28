# CLAUDE.md — Project Harness

> This file provides context and constraints for Claude Code when working in this repository.

## Project Overview

**ottoplus** — A composable local development environment platform for AI agents. Kubernetes-native control plane with a pluggable block architecture. AI agents describe what infrastructure they need, and the platform provisions and wires it together.

### Tech Stack

| Layer | Technology |
|-------|-----------|
| Language | **Go** |
| Orchestration | **Kubernetes** (k3d for local dev) |
| Cloud Simulation | **LocalStack** (S3, SQS, IAM) |
| Operator Framework | **controller-runtime** (sigs.k8s.io/controller-runtime) |
| API | REST (net/http or chi) with OpenAPI |
| CRD | `Cluster` (ottoplus.io/v1alpha1) |

### Architecture

- **Control Plane** (`src/api/`, `src/operator/`) — API server + K8s operator that watches `Cluster` CRDs and reconciles desired state.
- **Block Layer** (`src/operator/blocks/`) — Pluggable infrastructure components (engines, proxies, storage, monitoring, auth, networking, integrations).
- **Core Domain** (`src/core/`) — Pure business logic with zero infrastructure dependencies.
- **Local Dev** — One-command setup via `make dev-up` (k3d + LocalStack). Ephemeral, data loss acceptable.

### Composable Block Architecture ("Lego Blocks")

The system is built from composable blocks that wire together via typed ports:

- **Block** — Self-contained unit with `Descriptor` (kind, ports, parameters, requires, provides). Defined in `src/core/block/`.
- **Port** — Typed connection point (input/output). Blocks connect when port types match (e.g. `dsn`, `pvc-spec`, `metrics-endpoint`).
- **Wire** — Explicit connection from one block's output port to another's input port. Can be auto-wired when unambiguous.
- **Composition** — A set of BlockRefs + Wires that form a complete environment stack.
- **BlockRuntime** — Infrastructure-aware implementation (in `src/operator/blocks/`) that reconciles K8s resources.
- **BLOCK.md** — Per-block manifest with YAML frontmatter (machine-readable) + markdown body (AI-readable).

Block categories: `datastore`, `compute`, `storage`, `observability`, `security`, `networking`.

The CRD supports both **shorthand** (flat `engine`/`replicas` fields, auto-expanded) and **explicit composition** (`spec.blocks` with composition + wires + inline inputs).

### AI-Agent-Friendly Design

- `CLAUDE.md` per module for self-describing context.
- `BLOCK.md` per block with YAML frontmatter for machine-readable descriptors.
- `Makefile` as single entry point (`make help`).
- Idempotent operations throughout (API, operator reconciliation, scripts).
- Fast feedback loops: unit tests run without infra, integration tests under 60s.
- Observable: structured JSON logs, health endpoints, OpenTelemetry traces.

## Code Conventions

### General

- Use clear, descriptive names. Avoid abbreviations unless universally understood (`id`, `url`, `http` are fine; `mgr`, `svc`, `ctx` are not).
- One concept per file. If a file is doing two unrelated things, split it.
- Keep functions short (< 40 lines as a guideline). Extract when complexity grows, not preemptively.
- Prefer composition over inheritance.
- No dead code. Remove unused imports, variables, and functions — don't comment them out.

### Naming

| Element       | Convention         | Example              |
|---------------|--------------------|----------------------|
| Files         | kebab-case         | `database-cluster.go` |
| Directories   | kebab-case         | `api-handlers/`       |
| Constants     | PascalCase (exported) or camelCase (unexported) | `MaxRetryCount` / `defaultTimeout` |
| Functions     | camelCase (Go idiom) | `getUserByID`        |
| Types/Structs | PascalCase         | `Cluster`             |
| Booleans      | is/has/should prefix | `isActive`, `hasPermission` |

### Directory Structure

```
src/
  core/        # Domain logic — no infra deps, pure Go, testable in isolation
  api/         # Control plane API server (REST/gRPC handlers, middleware)
  operator/    # K8s operator (reconciler for Cluster CRD)
  shared/      # Shared utilities (logging, errors, config)
deploy/
  k3d-config.yaml          # Local K8s cluster config
  localstack/               # LocalStack K8s manifests
  crds/                     # Custom Resource Definitions
tests/
  unit/        # Fast tests, no infra needed
  integration/ # Requires k3d + LocalStack
  e2e/         # Full system tests
scripts/       # dev-up, dev-down, seed, wait-ready
docs/          # Architecture and local dev guides
```

### Comments & Documentation

- Don't add comments for obvious code. Only comment *why*, not *what*.
- Exported functions and types must have Go doc comments (`// FuncName does...`).
- Keep README and docs up to date when behavior changes.

## Git Workflow

### Branching

- `main` — production-ready, protected. Never push directly.
- `feat/<name>` — new features
- `fix/<name>` — bug fixes
- `chore/<name>` — refactoring, tooling, config
- `docs/<name>` — documentation only

### Commits

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <short summary>

<optional body>
```

Types: `feat`, `fix`, `refactor`, `chore`, `docs`, `test`, `ci`, `perf`

Rules:
- Summary line ≤ 72 characters, imperative mood ("add", not "added" or "adds")
- One logical change per commit. Don't bundle unrelated changes.
- Never commit secrets, credentials, or `.env` files.

### Pull Requests

- Keep PRs focused — one feature or fix per PR.
- PR title follows the same Conventional Commits format.
- PR body must include: Summary (what & why), Test plan.
- All CI checks must pass before merge.
- Squash merge to `main` by default.

## Architecture Principles

### Design

1. **Separation of Concerns** — Business logic must not depend on infrastructure. Use dependency injection or ports/adapters.
2. **Explicit over Implicit** — Prefer explicit configuration and explicit error handling over magic/convention.
3. **Fail Fast** — Validate inputs at system boundaries. Propagate errors clearly; don't swallow them.
4. **Minimal Surface Area** — Export only what consumers need. Keep internal implementation private.

### Dependencies

- Evaluate before adding. Every dependency is a liability.
- Pin versions. Use lockfiles.
- Prefer well-maintained, widely-used libraries. Avoid single-maintainer packages for critical paths.
- No unnecessary wrappers around standard library features.

### Security

- Never hardcode secrets. Use environment variables or a secrets manager.
- Validate and sanitize all external input (user input, API responses, file contents).
- Use parameterized queries for database access — never string concatenation.
- Apply principle of least privilege for service accounts and API keys.
- Keep dependencies up to date; monitor for known vulnerabilities.

### Error Handling

- Use typed/structured errors where the language supports it.
- Log errors with context (what operation, what input, what failed).
- Don't expose internal error details to end users.
- Handle the error or propagate it — never silently ignore.

### Testing

- Write tests for business logic and edge cases. Don't test framework internals.
- Tests should be deterministic — no flaky tests, no dependency on external services.
- Test file naming: `<module>_test.go`.
- Use test fixtures and factories, not raw inline data in every test.

## Claude-Specific Instructions

### Memory & Knowledge Persistence

- At the end of each development session, persist any new learnings, decisions, or context to this file (or sub-directory CLAUDE.md files) and commit them to the repo.
- Examples of what to persist: architecture decisions, user preferences, tech stack choices, gotchas discovered, patterns adopted.
- Keep memory entries concise and actionable — not a diary, but a reference.

### When Working in This Repo

- Always read relevant files before making changes.
- Follow the conventions defined above — do not introduce new patterns without discussion.
- When unsure about architecture decisions, ask before implementing.
- Prefer small, incremental changes over large rewrites.
- Run existing tests/linters before committing if available.
- Do not create new config files (.eslintrc, tsconfig, etc.) unless explicitly asked.

### What NOT to Do

- Don't add features beyond what was requested.
- Don't refactor code that isn't related to the current task.
- Don't add comments to code you didn't change.
- Don't create README or documentation files unless asked.
- Don't install new dependencies without confirming with the user first.
