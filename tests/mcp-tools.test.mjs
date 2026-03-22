import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { callTool, getToolDefinitions } from "../plugin/scripts/lib/mcp-tools.mjs";
import { recordPrompt, summarizeSession } from "../plugin/scripts/lib/store.mjs";

async function withTempStore(run) {
  const dataHome = await mkdtemp(join(tmpdir(), "memory-mesh-tools-"));
  const previous = process.env.MEMORY_MESH_HOME;
  process.env.MEMORY_MESH_HOME = dataHome;

  try {
    await run(dataHome);
  } finally {
    if (previous === undefined) {
      delete process.env.MEMORY_MESH_HOME;
    } else {
      process.env.MEMORY_MESH_HOME = previous;
    }
    await rm(dataHome, { recursive: true, force: true });
  }
}

test("tool definitions expose the expected MCP surface", () => {
  const names = getToolDefinitions().map((tool) => tool.name);
  assert.deepEqual(names, [
    "search_memories",
    "list_recent_memories",
    "get_memory",
    "store_memory",
    "remember_decision",
    "remember_constraint",
    "remember_handoff",
    "remember_goal",
  ]);
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

test("remember_goal stores a durable goal memory", async () => {
  await withTempStore(async () => {
    await callTool("remember_goal", {
      cwd: process.cwd(),
      goal: "Ship v2 API with full backward compatibility by end of Q3.",
      title: "Ship v2 API",
    });

    const goals = await callTool("list_recent_memories", {
      cwd: process.cwd(),
      memoryType: "goal",
    });
    assert.match(goals.content[0].text, /Type=goal/);
    assert.match(goals.content[0].text, /Ship v2 API/);
  });
});

test("remember_decision and remember_handoff create typed memories", async () => {
  await withTempStore(async () => {
    await callTool("remember_decision", {
      cwd: process.cwd(),
      decision: "Use structured memory types so durable team context beats noisy worklogs.",
      title: "Structured memory policy",
    });
    await callTool("remember_handoff", {
      cwd: process.cwd(),
      handoff: "Next agent should wire the new memory tools into their workflow.",
    });

    const decisions = await callTool("list_recent_memories", {
      cwd: process.cwd(),
      memoryType: "decision",
    });
    assert.match(decisions.content[0].text, /Type=decision/);
    assert.match(decisions.content[0].text, /Structured memory policy/);

    const handoff = await callTool("search_memories", {
      cwd: process.cwd(),
      query: "next agent",
      memoryType: "handoff",
    });
    assert.match(handoff.content[0].text, /Type=handoff/);
  });
});
