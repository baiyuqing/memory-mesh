# Task Plan

## Current Task: Build Claude Code Memory Plugin MVP

- [x] Inspect local context and choose a minimal plugin architecture that matches Claude Code plugin hooks.
- [x] Scaffold an isolated marketplace/plugin repository with task tracking and package metadata.
- [x] Implement hook handlers for setup, session start, prompt capture, tool observation, and stop-time summarization.
- [x] Implement a lightweight MCP search server over the stored memory snapshots.
- [x] Add tests for storage, summarization, context injection, and MCP tool behavior.
- [x] Run verification, review the diff for scope, and push the branch.

## Assumptions

- Target format is a Claude Code plugin marketplace repo with a single installable plugin.
- MVP should avoid external runtime services and native dependencies.
- Memory storage should be local-only and readable without a database server.

## Review

- Built a Claude Code marketplace repo with a single installable plugin: `claude-code-memory`.
- Implemented hook handlers for setup, session start context injection, prompt capture, tool observation, and stop-time summarization using local JSON storage.
- Added a lightweight stdio MCP server with `search_memories`, `list_recent_memories`, and `get_memory`.
- Scoped memory by Git common directory so separate worktrees from the same repository share memory.
- Verification:
- `npm test`
- Manual hook flow via `node plugin/scripts/cli.mjs hook ...`
- Manual MCP stdio request against `node plugin/scripts/mcp-server.mjs`
