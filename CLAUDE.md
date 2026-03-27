# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

Memory Mesh — context relay for AI coding agents. Preserves goals, decisions, constraints, and handoffs across agent sessions so the next agent picks up where the last left off. Zero dependencies beyond Node.js ≥18.

## Commands

- `npm test` — run all tests (node:test, ~0.6s)
- `node --test tests/store.test.mjs` — run a single test file
- `npm run demo:team-memory` — run the team memory validation example

## Conventions

- Use `git commit --signoff` for all commits
- Zero runtime dependencies — no npm install needed
- All source is ESM (`"type": "module"`)
- Tests use `node:test` + `node:assert/strict`, no external test framework
- Tests use a `withTempStore(fn)` helper that creates an isolated temp directory and sets `MEMORY_MESH_HOME` — follow this pattern for new tests
- Follow existing patterns when adding new memory types or MCP tools (see `remember_decision` in `mcp-tools.mjs` as template)

## Architecture

### Entry points

- `plugin/scripts/cli.mjs` — Hook entry points (subcommands: `setup`, `session-start`, `observe`, `summarize`)
- `plugin/scripts/mcp-server.mjs` — Stdio MCP server exposing memory tools to Codex and other MCP clients

### Hook lifecycle

Setup → SessionStart (inject context) → UserPromptSubmit (record) → PostToolUse (record) → Stop (summarize)

Hook config lives in `plugin/hooks/hooks.json`. Each hook invokes `cli.mjs` with the appropriate subcommand.

### Storage layer (`plugin/scripts/lib/`)

- **store.mjs** — Storage facade. Dual-write: writes go to both local and remote backend; reads merge both sources with local winning on duplicates. Backend is resolved from `MEMORY_MESH_BACKEND` env var (`local` | `mem9` | custom module path).
- **local-store.mjs** — File-based storage under `~/.memory-mesh/` (override with `MEMORY_MESH_HOME`). Handles session journaling, memory persistence, and context injection formatting.
- **mem9-client.mjs** — HTTP client for the remote mem9 backend. Supports API key or tenant ID auth.
- **config.mjs** — Reads `MEMORY_MESH_BACKEND`, `MEMORY_MESH_AGENT_ID`, `MEMORY_MESH_TEAM_ID` (also supports legacy `CLAUDE_CODE_MEMORY_*` prefix). Priority: function args > env vars > defaults.
- **constants.mjs** — Memory type taxonomy, context selection limits, tool classification sets.
- **mcp-tools.mjs** — MCP tool definitions and `callTool()` dispatch.
- **project.mjs** — Derives project key from git remote URL, git root, or directory name. Worktrees from the same repo share a project key.

### Memory model

- **Durable types** (persist across sessions): `goal` > `decision` > `constraint` > `ownership` > `runbook` > `api-contract`
- **Activity types** (session-scoped): `session-summary`, `worklog`, `handoff`
- Context injection selects up to 5 from the 20 most recent: 3 durable (goals first) + 2 activity
- Injected as lightweight refs (~500 tokens) — type + title + 80-char hint + ID; agents call `get_memory` for full details

### Dual-write semantics

- Local stores verbatim content (source of truth, no compression)
- mem9 enables cross-agent sharing (may compress/deduplicate)
- If mem9 is unreachable, local still works — failure tolerant by design
