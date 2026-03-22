import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { getMemoryById, listRecentMemories, recordPrompt, recordToolUse, renderContextBlock, searchMemories, summarizeSession } from "../plugin/scripts/lib/store.mjs";

async function withTempStore(run) {
  const dataHome = await mkdtemp(join(tmpdir(), "claude-code-memory-"));
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

