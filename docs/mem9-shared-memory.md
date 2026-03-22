# mem9 Shared Memory

This plugin can run against `mem9` so Claude Code and Codex share one project memory pool.

## Architecture

- Claude Code hooks record prompts and tool activity locally.
- On `Stop`, the plugin compacts that work into a memory summary.
- In `mem9` mode, the summary is pushed to the configured `mem9` tenant or API key space.
- On `SessionStart`, Claude Code pulls a wider recent sample and prefers durable memory types before recent worklogs.
- Codex connects to the same MCP server and uses `search_memories`, `store_memory`, and focused remember tools against the same backend.

## Required Environment

```text
MEMORY_MESH_BACKEND=mem9
MEM9_API_URL=https://api.mem9.ai
MEM9_API_KEY=...
MEMORY_MESH_AGENT_ID=claude-code
MEMORY_MESH_TEAM_ID=platform
```

Alternative compatibility mode:

```text
MEMORY_MESH_BACKEND=mem9
MEM9_API_URL=https://api.mem9.ai
MEM9_TENANT_ID=...
MEMORY_MESH_AGENT_ID=claude-code
```

## Agent IDs

Use different agent IDs for each client:

- Claude Code: `claude-code`
- Codex: `codex`
- Other agents: whatever you standardize on

The memory records include agent tags and metadata so search results show who learned what.

## Shared Retrieval Scope

The plugin stores tags such as:

- `project:<repo>`
- `workspace:<worktree>`
- `agent:<agent-id>`
- `team:<team-id>` when configured
- `kind:<memory-kind>`

Search and context injection are project-scoped, so worktrees from the same repository can see each other's shared memory.

## Codex Usage

Point Codex's MCP configuration at [`plugin/scripts/mcp-server.mjs`](../plugin/scripts/mcp-server.mjs) and set the same `mem9` environment variables, but use:

```text
MEMORY_MESH_AGENT_ID=codex
```

Then Codex can:

- search existing team memory
- fetch a specific memory by ID
- store explicit durable notes with `store_memory`
- store durable decisions with `remember_decision`
- store hard constraints with `remember_constraint`
- leave resumable baton-passing notes with `remember_handoff`

## Recommended Team Pattern

- Auto-capture worklogs from Claude Code hooks.
- Use `remember_decision` and `remember_constraint` for durable team truths.
- Use `remember_handoff` for agent-to-agent continuation notes.
- Keep `store_memory` as the generic escape hatch when the note type does not fit the focused tools.
- Keep agent IDs stable.
- Keep one team ID per engineering group or repo domain.
- Use the same `MEM9_API_KEY` or tenant across all participating agents.
- Legacy `CLAUDE_CODE_MEMORY_*` environment variables remain supported for compatibility.
