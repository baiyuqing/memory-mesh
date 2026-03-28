# CLAUDE.md — Project Harness

> This file provides context and constraints for Claude Code when working in this repository.

## Project Overview

**ottoplus** — Project type and tech stack are TBD. This harness defines general engineering standards that apply regardless of the specific technology chosen.

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
| Files         | kebab-case         | `user-service.ts`    |
| Directories   | kebab-case         | `api-handlers/`      |
| Constants     | UPPER_SNAKE_CASE   | `MAX_RETRY_COUNT`    |
| Functions     | camelCase or snake_case (match language idiom) | `getUserById` / `get_user_by_id` |
| Types/Classes | PascalCase         | `UserProfile`        |
| Booleans      | is/has/should prefix | `isActive`, `hasPermission` |

### Directory Structure (general guidance)

```
src/           # Application source code
  core/        # Core domain logic (no external dependencies)
  infra/       # Infrastructure: DB, HTTP, messaging adapters
  api/         # API layer: routes, handlers, middleware
  shared/      # Shared utilities used across modules
tests/         # Test files mirroring src/ structure
docs/          # Project documentation
scripts/       # Build, deploy, and dev scripts
```

### Comments & Documentation

- Don't add comments for obvious code. Only comment *why*, not *what*.
- Public APIs must have docstrings/JSDoc.
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
- Test file naming: `<module>.test.<ext>` or `<module>_test.<ext>` (match language idiom).
- Use test fixtures and factories, not raw inline data in every test.

## Claude-Specific Instructions

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
