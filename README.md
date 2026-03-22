# Claude Code Memory Plugin

`claude-code-memory` is a minimal Claude Code plugin marketplace repo that adds persistent local memory to a project without requiring a background database or external service.

It stores prompt and tool activity as local session journals, turns them into compact memory snapshots on `Stop`, injects recent project memory on `SessionStart`, and exposes the stored summaries through an MCP server.

## Features

- Local-only persistent memory under `~/.claude-code-memory`
- Automatic context injection on new or compacted Claude Code sessions
- Stop-time memory summarization from prompts and tool usage
- MCP tools for `search_memories`, `list_recent_memories`, and `get_memory`
- Zero runtime dependencies beyond Node.js

## Install

In Claude Code:

```text
/plugin marketplace add baiyuqing/otto
/plugin install claude-code-memory
```

Restart Claude Code after installation.

## How It Works

The plugin uses Claude Code hooks:

- `SessionStart` injects recent memories for the current project.
- `UserPromptSubmit` records the user prompt into a local session journal.
- `PostToolUse` records tool activity, changed files, and shell commands.
- `Stop` compacts the current session into a reusable memory snapshot.

Memory is grouped by project using Git metadata when available, so worktrees from the same repository share the same project key.

## Local Storage

By default the plugin stores data in:

```text
~/.claude-code-memory/
  sessions/
  memories/
```

You can override this with `CLAUDE_CODE_MEMORY_HOME`.

## Development

```bash
npm test
```

The actual installable plugin lives in [`plugin/`](./plugin) and the marketplace manifest lives in [`.claude-plugin/marketplace.json`](./.claude-plugin/marketplace.json).
