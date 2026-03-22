# Claude Code Memory Plugin

`claude-code-memory` is a Claude Code plugin marketplace repo that adds persistent memory to a project.

It supports:

- a zero-dependency local backend for single-user memory
- a `mem9` backend for multi-agent shared memory across Claude Code and Codex

The plugin stores prompt and tool activity as local session journals, turns them into compact memory snapshots on `Stop`, injects recent project memory on `SessionStart`, and exposes memory search/store through an MCP server.

## Features

- Local persistent memory under `~/.claude-code-memory`
- Optional `mem9` remote backend for shared team memory
- Automatic context injection on new or compacted Claude Code sessions
- Stop-time memory summarization from prompts and tool usage
- Session-start context that prefers durable decisions and constraints over noisy worklogs
- MCP tools for `search_memories`, `list_recent_memories`, `get_memory`, `store_memory`, `remember_decision`, `remember_constraint`, and `remember_handoff`
- Zero runtime dependencies beyond Node.js

## Install

In Claude Code:

```text
/plugin marketplace add baiyuqing/otto
/plugin install claude-code-memory
```

Restart Claude Code after installation.

## Backends

### Local backend

This is the default mode.

```text
CLAUDE_CODE_MEMORY_BACKEND=local
```

Storage lives under:

```text
~/.claude-code-memory/
  sessions/
  memories/
```

You can override this with `CLAUDE_CODE_MEMORY_HOME`.

### mem9 shared backend

Use this when multiple agents should share one memory pool.

Claude Code plugin hooks still build local session summaries, but the final memory is pushed to `mem9` so other agents can retrieve it.

Set these environment variables in Claude Code:

```json
{
  "env": {
    "CLAUDE_CODE_MEMORY_BACKEND": "mem9",
    "MEM9_API_URL": "https://api.mem9.ai",
    "MEM9_API_KEY": "your-api-key",
    "CLAUDE_CODE_MEMORY_AGENT_ID": "claude-code",
    "CLAUDE_CODE_MEMORY_TEAM_ID": "platform"
  }
}
```

If you use the older tenant-scoped API instead of API keys, set `MEM9_TENANT_ID` instead of `MEM9_API_KEY`.

## How It Works

The plugin uses Claude Code hooks:

- `SessionStart` injects recent project memory, with durable team facts first and recent worklogs second.
- `UserPromptSubmit` records the user prompt into a local session journal.
- `PostToolUse` records tool activity, changed files, and shell commands.
- `Stop` compacts the current session into a reusable memory snapshot.

Memory is grouped by project using Git metadata when available, so worktrees from the same repository share the same project key.

## Claude Code + Codex Shared Memory

For a multi-agent team setup:

- Claude Code uses the plugin hooks to auto-load recent project memory and auto-store session summaries.
- Codex points its MCP configuration at [`plugin/scripts/mcp-server.mjs`](./plugin/scripts/mcp-server.mjs).
- Both sides use the same backend and the same `MEM9_API_KEY` or `MEM9_TENANT_ID`.
- Distinguish agents with `CLAUDE_CODE_MEMORY_AGENT_ID`, for example `claude-code` and `codex`.

Codex can use:

- `search_memories`
- `list_recent_memories`
- `get_memory`
- `store_memory`
- `remember_decision`
- `remember_constraint`
- `remember_handoff`

That gives Codex an explicit write path even though it does not use Claude Code hook lifecycle events, and it makes durable team facts easier to separate from routine worklogs.

## Shared Memory Notes

- Shared retrieval is project-scoped by Git common directory, not worktree-scoped.
- Stored tags include project, workspace, agent, and memory kind.
- Context injection prefers durable memory types such as `decision` and `constraint`, then fills with recent `handoff` or `session-summary` worklogs.
- `mem9` mode is a shared backend; local mode remains available for private memory.

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
