import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { callTool, getToolDefinitions } from "../plugin/scripts/lib/mcp-tools.mjs";
import { recordPrompt, summarizeSession } from "../plugin/scripts/lib/store.mjs";

async function withTempStore(run) {
  const dataHome = await mkdtemp(join(tmpdir(), "claude-code-memory-tools-"));
  const previous = process.env.CLAUDE_CODE_MEMORY_HOME;
  process.env.CLAUDE_CODE_MEMORY_HOME = dataHome;

  try {
    await run(dataHome);
  } finally {
    if (previous === undefined) {
      delete process.env.CLAUDE_CODE_MEMORY_HOME;
    } else {
      process.env.CLAUDE_CODE_MEMORY_HOME = previous;
    }
    await rm(dataHome, { recursive: true, force: true });
  }
}

test("tool definitions expose the expected MCP surface", () => {
  const names = getToolDefinitions().map((tool) => tool.name);
  assert.deepEqual(names, ["search_memories", "list_recent_memories", "get_memory", "store_memory"]);
});

test("get_memory returns a formatted memory body", async () => {
  await withTempStore(async () => {
    const cwd = process.cwd();
    await recordPrompt({ sessionId: "memory-1", cwd, prompt: "Remember plugin installation steps" });
    await summarizeSession({ sessionId: "memory-1", cwd });

    const result = await callTool("get_memory", { id: "memory-1" });
    assert.match(result.content[0].text, /Remember plugin installation steps/);
    assert.match(result.content[0].text, /Summary:/);
  });
});

test("store_memory persists a local explicit memory", async () => {
  await withTempStore(async () => {
    const result = await callTool("store_memory", {
      cwd: process.cwd(),
      content: "The shared deploy workflow uses a single release branch.",
      memoryType: "decision",
    });

    assert.match(result.content[0].text, /Stored shared memory/);

    const recall = await callTool("search_memories", {
      cwd: process.cwd(),
      query: "release branch",
    });

    assert.match(recall.content[0].text, /shared deploy workflow/i);
  });
});
