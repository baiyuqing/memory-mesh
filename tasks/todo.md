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

## Current Task: Add Multi-Agent Shared Memory via mem9

- [x] Inspect `mem9-ai/mem9` to confirm whether it supports shared memory across agent clients.
- [x] Refactor the plugin memory layer so local storage remains intact while a remote backend can be selected.
- [x] Add `mem9` backend support for shared search/store from Claude Code hooks and MCP tools.
- [x] Add an explicit `store_memory` MCP tool so Codex can write shared memory without Claude hook lifecycles.
- [x] Document Claude Code + Codex shared-memory setup against the same `mem9` backend.
- [x] Add regression tests for `mem9` backend behavior and rerun the suite.

### Review

- Confirmed from the upstream `mem9` repository that it is explicitly designed for shared cloud memory across Claude Code and other agent clients.
- Added backend selection via `CLAUDE_CODE_MEMORY_BACKEND`, with `local` as the default and `mem9` as the shared remote option.
- Kept local session journaling for summary generation, then pushed summarized memory into `mem9` when the remote backend is enabled.
- Added `store_memory` to the MCP surface so Codex can persist durable team notes into the same project memory pool.
- Added documentation for shared setup and agent identity conventions in `README.md` and `docs/mem9-shared-memory.md`.
- Validation:
- `npm test`

## Current Task: Structure Team Memory for Durable Collaboration

- [x] Fix local-backend memory typing so explicit decisions and constraints can be filtered correctly.
- [x] Add focused MCP write tools for durable team facts and handoffs instead of relying on one generic shared note API.
- [x] Prioritize durable memories over raw worklogs when injecting session-start context.
- [x] Add regression coverage for typed memory filters, context prioritization, and the new MCP tools.
- [x] Run verification, review the diff, and commit the changes.

### Review

- Fixed local memory typing so `memoryType` now survives storage, filtering, and tag-based search.
- Added `remember_decision`, `remember_constraint`, and `remember_handoff` on top of the existing generic `store_memory` path.
- Changed session-start context injection to prefer durable memory types first, then recent shared worklogs, then fallback memory.
- Fixed tag merging so custom tags no longer drop required project, workspace, team, or agent scope tags.
- Validation:
- `npm test`
