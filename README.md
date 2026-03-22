# Memory Mesh

[![CI](https://github.com/baiyuqing/memory-mesh/actions/workflows/ci.yml/badge.svg)](https://github.com/baiyuqing/memory-mesh/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Node.js](https://img.shields.io/badge/Node.js-%3E%3D18-green.svg)](https://nodejs.org)
[![Zero Dependencies](https://img.shields.io/badge/dependencies-0-brightgreen.svg)](#)
[![MCP](https://img.shields.io/badge/MCP-compatible-blueviolet.svg)](#)

![Memory Mesh hero](./assets/memory-mesh-hero.svg)

`memory-mesh` is a Claude Code plugin marketplace repo for shared engineering memory across Claude Code and Codex.

It supports:

- a zero-dependency local backend for single-user memory
- a `mem9` backend for multi-agent shared memory across Claude Code and Codex

The plugin stores prompt and tool activity as local session journals, turns them into compact memory snapshots on `Stop`, injects recent project memory on `SessionStart`, and exposes memory search/store through an MCP server.

## Features

- Local persistent memory under `~/.memory-mesh`
- Optional `mem9` remote backend for shared team memory
- **Dual-write storage** — local preserves verbatim content (source of truth), mem9 enables cross-agent sharing
- **Lightweight context injection** — session start injects compact references (~500 tokens), agents pull full details on demand via `get_memory`
- Automatic context injection on new or compacted Claude Code sessions
- Stop-time memory summarization from prompts and tool usage
- Session-start context that prefers durable decisions and constraints over noisy worklogs
- MCP tools for `search_memories`, `list_recent_memories`, `get_memory`, `store_memory`, `remember_decision`, `remember_constraint`, `remember_handoff`, and `remember_goal`
- Zero runtime dependencies beyond Node.js

## Install

In Claude Code:

```text
/install baiyuqing/memory-mesh
/plugin install memory-mesh
```

Restart Claude Code after installation.

## Backends

### Local backend

This is the default mode.

```text
MEMORY_MESH_BACKEND=local
```

Storage lives under:

```text
~/.memory-mesh/
  sessions/
  memories/
```

You can override this with `MEMORY_MESH_HOME`.

### mem9 shared backend

Use this when multiple agents should share one memory pool.

Claude Code plugin hooks still build local session summaries, but the final memory is pushed to `mem9` so other agents can retrieve it.

Set these environment variables in Claude Code:

```json
{
  "env": {
    "MEMORY_MESH_BACKEND": "mem9",
    "MEM9_API_URL": "https://api.mem9.ai",
    "MEM9_API_KEY": "your-api-key",
    "MEMORY_MESH_AGENT_ID": "claude-code",
    "MEMORY_MESH_TEAM_ID": "platform"
  }
}
```

If you use the older tenant-scoped API instead of API keys, set `MEM9_TENANT_ID` instead of `MEM9_API_KEY`.
Legacy `CLAUDE_CODE_MEMORY_*` environment variables are still accepted for compatibility.

## How It Works

The plugin uses Claude Code hooks:

- `SessionStart` injects lightweight memory references (type + title + hint + ID), with durable team facts first and recent worklogs second.
- `UserPromptSubmit` records the user prompt into a local session journal.
- `PostToolUse` records tool activity, changed files, and shell commands.
- `Stop` compacts the current session into a reusable memory snapshot.

Memory is grouped by project using Git metadata when available, so worktrees from the same repository share the same project key.

### Context Injection Format

Session start injects compact references that agents can expand on demand:

```
Persistent memory for myproject

Team: alice (pm), bob (tech-lead), charlie (dev)

Team goals & durable memory:
  📌 [goal] by alice: Ship v2 API — Full backward compat by end of Q3 — ID: goal-1
  📌 [decision] by bob: Use PostgreSQL — Chose PG over DynamoDB for join support — ID: decision-1

Recent shared worklog:
  🔄 [handoff] by charlie: Auth progress — Auth module 80% complete, remaining: rate limiting — ID: handoff-1

Use `get_memory` with any ID above for full details, or `search_memories` to find more.
```

This design is inspired by [Anthropic's multi-agent research system](https://www.anthropic.com/engineering/multi-agent-research-system): pass lightweight references, let agents pull full details on demand.

### Dual-Write Storage

When `backend=mem9`, writes go to **both** local and mem9:

- **Local** preserves verbatim content (source of truth, no compression)
- **mem9** enables cross-agent sharing (may compress/deduplicate)
- **Reads** merge both sources, local wins on duplicates
- **Failure tolerant** — if mem9 is down, local still works

## Claude Code + Codex Shared Memory

For a multi-agent team setup:

- Claude Code uses the plugin hooks to auto-load recent project memory and auto-store session summaries.
- Codex points its MCP configuration at [`plugin/scripts/mcp-server.mjs`](./plugin/scripts/mcp-server.mjs).
- Both sides use the same backend and the same `MEM9_API_KEY` or `MEM9_TENANT_ID`.
- Distinguish agents with `MEMORY_MESH_AGENT_ID`, for example `claude-code` and `codex`.

Codex can use:

- `search_memories`
- `list_recent_memories`
- `get_memory`
- `store_memory`
- `remember_goal`
- `remember_decision`
- `remember_constraint`
- `remember_handoff`

That gives Codex an explicit write path even though it does not use Claude Code hook lifecycle events, and it makes durable team facts easier to separate from routine worklogs.

## Memory Types

| Type | Category | Priority | Use Case |
|------|----------|----------|----------|
| `goal` | Durable | Highest | North-star objectives set by PM/TL |
| `decision` | Durable | High | Architectural and design decisions |
| `constraint` | Durable | High | Non-negotiable rules and boundaries |
| `handoff` | Activity | Medium | Checkpoint when an agent leaves mid-task |
| `session-summary` | Activity | Low | Auto-generated session recap |
| `worklog` | Activity | Low | Incremental progress notes |

Context injection selects up to 3 durable (goals first) + 2 activity memories from the 20 most recent.

## Shared Memory Notes

- Shared retrieval is project-scoped by Git common directory, not worktree-scoped.
- Stored tags include project, workspace, agent, role, and memory kind.
- Dual-write ensures local verbatim fidelity + remote cross-agent sharing.
- Context injection prefers durable memory types such as `goal`, `decision` and `constraint`, then fills with recent `handoff` or `session-summary` worklogs.

More detail is in [`docs/mem9-shared-memory.md`](./docs/mem9-shared-memory.md).

## Development

```bash
npm test
```

```bash
npm run demo:team-memory
```

The validation example is documented in [`docs/team-memory-validation.md`](./docs/team-memory-validation.md).

The actual installable plugin lives in [`plugin/`](./plugin) and the marketplace manifest lives in [`.claude-plugin/marketplace.json`](./.claude-plugin/marketplace.json).
