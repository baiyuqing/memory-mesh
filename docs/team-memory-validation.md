# Team Memory Validation Example

This repository now includes a runnable validation example for the shared-memory design.

## What It Validates

The example models a small multi-agent team working on this repository:

- Claude Code records a durable `decision`
- Codex records a durable `constraint`
- Codex records a `handoff`
- Claude Code records a normal session summary worklog

Then a new session loads project context and should show:

1. durable team memory first
2. recent shared worklog second
3. correct type filtering for search and listing

## Scenario

The scenario uses this repo's current branch layout:

- Decision: `main` now hosts the `Memory Mesh` project
- Constraint: old `mysqlbench` code must remain on the `mysqlbench` branch
- Handoff: next step is wiring Claude Code and Codex to the same `mem9` backend
- Worklog: Claude Code tightened the MCP memory toolset and ran `npm test`

## Run It

From the repo root:

```bash
node examples/team-memory-validation.mjs
```

Or:

```bash
npm run demo:team-memory
```

The script uses a temporary local store, so it is safe to run repeatedly and does not need live `mem9` credentials.

## Expected Signals

The script should print:

- a `decision` query result that includes `Main branch ownership`
- a `handoff` query result that includes `shared retrieval on session start`
- a `SessionStart Context` block with:
  - `Durable team memory:`
  - `Main branch ownership`
  - `Legacy code safety`
  - `Recent shared worklog:`
  - one `[handoff]` entry
  - one `[session-summary]` entry
- a final `Validation example passed.`

## Why This Example Matters

This is the shortest example that proves the design is doing the right thing:

- typed durable memory is not lost
- shared handoff notes are searchable independently
- session-start context is not dominated by raw worklogs
- the same project can accumulate memory from multiple agents

## Real Cross-Client Follow-Up

After the local smoke check passes, you can do the real integration check with live clients:

1. Configure Claude Code and Codex with the same `MEM9_API_KEY` or `MEM9_TENANT_ID`
2. Keep `MEMORY_MESH_TEAM_ID` the same across both clients
3. Use different `MEMORY_MESH_AGENT_ID` values such as `claude-code` and `codex`
4. From Codex, call `remember_decision`, `remember_constraint`, or `remember_handoff`
5. Start a fresh Claude Code session in the same repo
6. Confirm the injected context surfaces the durable items before the recent worklog
