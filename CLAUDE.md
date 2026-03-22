# CLAUDE.md

## Project

Memory Mesh — context relay for AI coding agents. Preserves goals, decisions, constraints, and handoffs across agent sessions so the next agent picks up where the last left off. Zero dependencies beyond Node.js ≥18.

## Structure

```
plugin/
  scripts/
    cli.mjs              # Hook entry points (setup, session-start, observe, summarize)
    mcp-server.mjs       # Stdio MCP server exposing memory tools
    lib/
      config.mjs         # Env var config (backend, agentId, teamId)
      constants.mjs      # Memory types, limits, tool classifications
      local-store.mjs    # File-based storage, context injection
      mem9-client.mjs    # Remote mem9 HTTP backend
      mcp-tools.mjs      # MCP tool definitions and handlers
      project.mjs        # Git project context detection
      store.mjs          # Storage facade (dual-write local + mem9, merged reads)
  hooks/hooks.json       # Claude Code hook configuration
tests/                   # node --test, no framework
examples/                # Runnable demo scripts
```

## Commands

- `npm test` — run all tests (node:test, ~0.6s)
- `npm run demo:team-memory` — run the team memory validation example

## Conventions

- Use `git commit --signoff` for all commits
- Zero runtime dependencies — no npm install needed
- All source is ESM (`"type": "module"`)
- Tests use `node:test` + `node:assert/strict`, no external test framework
- Follow existing patterns when adding new memory types or MCP tools (see `remember_decision` as template)

## Architecture Notes

- Hook lifecycle: Setup → SessionStart (inject context) → UserPromptSubmit (record) → PostToolUse (record) → Stop (summarize)
- Memory types: `goal` > `decision` > `constraint` (durable) | `session-summary`, `handoff`, `worklog` (activity)
- Context injection: lightweight refs (~500 tokens) — type + title + 80-char hint + ID; agents use `get_memory` for full details
- Selection: up to 5 from 20 scanned — 3 durable (goals first) + 2 activity
- Dual-write: storeMemory writes local (verbatim) + mem9 (shared); reads merge both, local wins
- Local is always source of truth; mem9 is a sync target for cross-agent sharing
