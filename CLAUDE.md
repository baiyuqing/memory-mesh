# CLAUDE.md

## Project

Memory Mesh — a Claude Code plugin for shared persistent memory across AI coding agents (Claude Code, Codex). Zero dependencies beyond Node.js ≥18.

## Structure

```
plugin/
  scripts/
    cli.mjs              # Hook entry points (setup, session-start, observe, summarize)
    mcp-server.mjs       # Stdio MCP server exposing memory tools
    lib/
      config.mjs         # Env var config (backend, agentId, teamId, role)
      constants.mjs      # Memory types, limits, tool classifications
      local-store.mjs    # File-based storage, context injection, roster
      mem9-client.mjs    # Remote mem9 HTTP backend
      mcp-tools.mjs      # MCP tool definitions and handlers
      project.mjs        # Git project context detection
      store.mjs          # Storage facade (routes local ↔ mem9)
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
- Context injection selects up to 5 memories from 20 scanned: 3 durable (goals first) + 2 activity
- Team roster is auto-derived from memory authors, not from a registry
- Local sessions are always the source of truth; mem9 is a sync target
