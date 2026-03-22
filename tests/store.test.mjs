import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { getMemoryById, listRecentMemories, recordPrompt, recordToolUse, renderContextBlock, searchMemories, storeMemory, summarizeSession } from "../plugin/scripts/lib/store.mjs";
import { getProjectContext } from "../plugin/scripts/lib/project.mjs";

async function withTempStore(run) {
  const dataHome = await mkdtemp(join(tmpdir(), "memory-mesh-"));
  try {
    await run(dataHome);
  } finally {
    await rm(dataHome, { recursive: true, force: true });
  }
}

test("summarizeSession persists a searchable memory snapshot", async () => {
  await withTempStore(async (dataHome) => {
    const cwd = process.cwd();
    await recordPrompt(
      {
        sessionId: "session-1",
        cwd,
        prompt: "Build the claude code memory plugin MVP",
      },
      { dataHome },
    );
    await recordToolUse(
      {
        sessionId: "session-1",
        cwd,
        toolName: "Write",
        toolInput: { file_path: "plugin/scripts/cli.mjs" },
        toolResponse: { ok: true },
      },
      { dataHome },
    );
    await recordToolUse(
      {
        sessionId: "session-1",
        cwd,
        toolName: "Bash",
        toolInput: { command: "npm test" },
        toolResponse: { code: 0 },
      },
      { dataHome },
    );

    const memory = await summarizeSession({ sessionId: "session-1", cwd }, { dataHome });
    assert.equal(memory.id, "session-1");
    assert.match(memory.summary, /Build the claude code memory plugin MVP/);
    assert.deepEqual(memory.filesChanged, ["plugin/scripts/cli.mjs"]);
    assert.deepEqual(memory.commands, ["npm test"]);

    const stored = await getMemoryById("session-1", { dataHome });
    assert.equal(stored.id, "session-1");
  });
});

test("renderContextBlock scopes memories to the current project", async () => {
  await withTempStore(async (dataHome) => {
    const cwd = process.cwd();

    await recordPrompt({ sessionId: "same-project", cwd, prompt: "Investigate memory injection" }, { dataHome });
    await summarizeSession({ sessionId: "same-project", cwd }, { dataHome });

    const otherCwd = join(tmpdir(), "some-other-project");
    await recordPrompt({ sessionId: "other-project", cwd: otherCwd, prompt: "Unrelated work" }, { dataHome });
    await summarizeSession({ sessionId: "other-project", cwd: otherCwd }, { dataHome });

    const context = await renderContextBlock({ cwd }, { dataHome });
    assert.match(context, /Investigate memory injection/);
    assert.doesNotMatch(context, /Unrelated work/);
  });
});

test("searchMemories returns relevant recent memories", async () => {
  await withTempStore(async (dataHome) => {
    const cwd = process.cwd();

    await recordPrompt({ sessionId: "a", cwd, prompt: "Add MCP search tools" }, { dataHome });
    await summarizeSession({ sessionId: "a", cwd }, { dataHome });

    await recordPrompt({ sessionId: "b", cwd, prompt: "Refactor CSS theme" }, { dataHome });
    await summarizeSession({ sessionId: "b", cwd }, { dataHome });

    const results = await searchMemories({ cwd, query: "search tools" }, { dataHome });
    assert.equal(results.length, 1);
    assert.equal(results[0].id, "a");

    const recent = await listRecentMemories({ cwd, limit: 2 }, { dataHome });
    assert.equal(recent.length, 2);
  });
});

test("local store keeps typed memories searchable and filterable", async () => {
  await withTempStore(async (dataHome) => {
    const cwd = process.cwd();
    const expectedProjectKey = getProjectContext(cwd).projectKey;

    await storeMemory(
      {
        id: "decision-1",
        cwd,
        content: "Use mem9 as the shared team memory backend.",
        memoryType: "decision",
        tags: ["area:memory"],
      },
      { dataHome, agentId: "codex" },
    );
    await storeMemory(
      {
        id: "handoff-1",
        cwd,
        content: "Next agent should wire Codex config to the MCP server.",
        memoryType: "handoff",
      },
      { dataHome, agentId: "claude-code" },
    );

    const decisions = await listRecentMemories({ cwd, memoryType: "decision" }, { dataHome });
    assert.equal(decisions.length, 1);
    assert.equal(decisions[0].id, "decision-1");
    assert.ok(decisions[0].tags.includes("kind:decision"));
    assert.ok(decisions[0].tags.includes(`project:${expectedProjectKey}`));
    assert.ok(decisions[0].tags.includes("agent:codex"));
    assert.ok(decisions[0].tags.includes("area:memory"));

    const handoffSearch = await searchMemories({ cwd, query: "next agent", memoryType: "handoff" }, { dataHome });
    assert.equal(handoffSearch.length, 1);
    assert.equal(handoffSearch[0].id, "handoff-1");
  });
});

test("renderContextBlock shows goals first and includes team roster", async () => {
  await withTempStore(async (dataHome) => {
    const cwd = process.cwd();

    await storeMemory(
      {
        id: "goal-1",
        cwd,
        content: "Ship v2 API with full backward compatibility by end of Q3.",
        title: "Ship v2 API",
        memoryType: "goal",
        updatedAt: "2026-03-20T00:00:00.000Z",
      },
      { dataHome, agentId: "alice", role: "pm" },
    );
    await storeMemory(
      {
        id: "decision-1",
        cwd,
        content: "Use PostgreSQL for the new service.",
        title: "Use PostgreSQL",
        memoryType: "decision",
        updatedAt: "2026-03-21T00:00:00.000Z",
      },
      { dataHome, agentId: "bob", role: "tech-lead" },
    );
    await storeMemory(
      {
        id: "handoff-1",
        cwd,
        content: "Auth module 80% complete, remaining: rate limiting.",
        title: "Auth progress",
        memoryType: "handoff",
        updatedAt: "2026-03-22T00:00:00.000Z",
      },
      { dataHome, agentId: "charlie", role: "dev" },
    );

    const context = await renderContextBlock({ cwd }, { dataHome });

    // Goals appear in the durable section with the goals label
    assert.match(context, /Team goals & durable memory:/);
    assert.match(context, /\[goal\]/);
    assert.match(context, /Ship v2 API/);

    // Goal appears before decision (goal first sorting)
    const goalIdx = context.indexOf("[goal]");
    const decisionIdx = context.indexOf("[decision]");
    assert.ok(goalIdx < decisionIdx, "goal should appear before decision");

    // Team roster is shown
    assert.match(context, /Team:/);
    assert.match(context, /alice \(pm\)/);
    assert.match(context, /bob \(tech-lead\)/);
    assert.match(context, /charlie \(dev\)/);
  });
});

test("renderContextBlock prioritizes durable memories ahead of raw worklogs", async () => {
  await withTempStore(async (dataHome) => {
    const cwd = process.cwd();

    await storeMemory(
      {
        id: "decision-ctx",
        cwd,
        content: "Always route shared team memory through mem9 for Claude Code and Codex.",
        title: "Shared backend decision",
        memoryType: "decision",
        updatedAt: "2026-03-21T00:00:00.000Z",
      },
      { dataHome },
    );
    await storeMemory(
      {
        id: "constraint-ctx",
        cwd,
        content: "Do not write team memory into the main worktree checkout.",
        title: "Worktree safety constraint",
        memoryType: "constraint",
        updatedAt: "2026-03-20T00:00:00.000Z",
      },
      { dataHome },
    );
    await storeMemory(
      {
        id: "handoff-ctx",
        cwd,
        content: "Current status: main now points at the plugin repo; next step is config polish.",
        title: "Branch migration handoff",
        memoryType: "handoff",
        updatedAt: "2026-03-22T01:00:00.000Z",
      },
      { dataHome },
    );

    await recordPrompt({ sessionId: "ctx-session", cwd, prompt: "Tighten the MCP toolset" }, { dataHome });
    await summarizeSession({ sessionId: "ctx-session", cwd, createdAt: "2026-03-22T02:00:00.000Z" }, { dataHome });

    const context = await renderContextBlock({ cwd }, { dataHome });
    assert.match(context, /Durable team memory:/);
    assert.match(context, /Shared backend decision/);
    assert.match(context, /Worktree safety constraint/);
    assert.match(context, /Recent shared worklog:/);
    assert.match(context, /Branch migration handoff/);
    assert.match(context, /\[decision\]/);
    assert.match(context, /\[session-summary\]/);
  });
});
