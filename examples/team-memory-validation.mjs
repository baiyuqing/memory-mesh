#!/usr/bin/env node

import assert from "node:assert/strict";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { pathToFileURL } from "node:url";
import { callTool } from "../plugin/scripts/lib/mcp-tools.mjs";
import { recordPrompt, recordToolUse, renderContextBlock, summarizeSession } from "../plugin/scripts/lib/store.mjs";

function isMainModule() {
  return process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href;
}

function heading(title) {
  process.stdout.write(`\n=== ${title} ===\n`);
}

async function withAgent(agentId, run) {
  const previous = process.env.CLAUDE_CODE_MEMORY_AGENT_ID;
  process.env.CLAUDE_CODE_MEMORY_AGENT_ID = agentId;
  try {
    return await run();
  } finally {
    if (previous === undefined) {
      delete process.env.CLAUDE_CODE_MEMORY_AGENT_ID;
    } else {
      process.env.CLAUDE_CODE_MEMORY_AGENT_ID = previous;
    }
  }
}

function rememberEnv(keys) {
  return Object.fromEntries(keys.map((key) => [key, process.env[key]]));
}

function restoreEnv(snapshot) {
  for (const [key, value] of Object.entries(snapshot)) {
    if (value === undefined) {
      delete process.env[key];
    } else {
      process.env[key] = value;
    }
  }
}

export async function runTeamMemoryValidation(options = {}) {
  const cwd = options.cwd || process.cwd();
  const scenarioName = options.scenarioName || "branch-migration";
  const cleanup = options.cleanup !== false;
  const dataHome = options.dataHome || (await mkdtemp(join(tmpdir(), "claude-code-memory-demo-")));
  const envSnapshot = rememberEnv([
    "CLAUDE_CODE_MEMORY_HOME",
    "CLAUDE_CODE_MEMORY_BACKEND",
    "CLAUDE_CODE_MEMORY_TEAM_ID",
    "CLAUDE_CODE_MEMORY_AGENT_ID",
  ]);

  process.env.CLAUDE_CODE_MEMORY_HOME = dataHome;
  process.env.CLAUDE_CODE_MEMORY_BACKEND = options.backend || "local";
  process.env.CLAUDE_CODE_MEMORY_TEAM_ID = options.teamId || "memory-demo";

  try {
    await withAgent("claude-code", async () => {
      await callTool("remember_decision", {
        cwd,
        title: "Main branch ownership",
        decision: "The main branch now hosts the claude-code-memory project.",
        tags: ["scenario:branch-migration"],
      });
    });

    await withAgent("codex", async () => {
      await callTool("remember_constraint", {
        cwd,
        title: "Legacy code safety",
        constraint: "The old mysqlbench code must stay on the mysqlbench branch and should not be rewritten from the plugin worktree.",
        tags: ["scenario:branch-migration"],
      });
      await callTool("remember_handoff", {
        cwd,
        title: "Next implementation step",
        handoff: "Next step is to wire Claude Code and Codex to the same mem9 backend and verify shared retrieval on session start.",
        tags: ["scenario:branch-migration"],
      });
    });

    await withAgent("claude-code", async () => {
      const sessionId = `demo-${scenarioName}-session`;
      await recordPrompt({
        sessionId,
        cwd,
        prompt: "Tighten the MCP memory toolset for structured team collaboration.",
      });
      await recordToolUse({
        sessionId,
        cwd,
        toolName: "Edit",
        toolInput: { file_path: "plugin/scripts/lib/mcp-tools.mjs" },
        toolResponse: { ok: true },
      });
      await recordToolUse({
        sessionId,
        cwd,
        toolName: "Bash",
        toolInput: { command: "npm test" },
        toolResponse: { code: 0 },
      });
      await summarizeSession({ sessionId, cwd });
    });

    const decisions = await withAgent("claude-code", async () =>
      callTool("list_recent_memories", {
        cwd,
        memoryType: "decision",
      }),
    );
    const handoffs = await withAgent("codex", async () =>
      callTool("search_memories", {
        cwd,
        query: "shared retrieval on session start",
        memoryType: "handoff",
      }),
    );
    const context = await withAgent("claude-code", async () => renderContextBlock({ cwd }));

    assert.match(decisions.content[0].text, /Type=decision/);
    assert.match(decisions.content[0].text, /Main branch ownership/);
    assert.match(handoffs.content[0].text, /Type=handoff/);
    assert.match(handoffs.content[0].text, /shared retrieval on session start/i);
    assert.match(context, /Durable team memory:/);
    assert.match(context, /Main branch ownership/);
    assert.match(context, /Legacy code safety/);
    assert.match(context, /Recent shared worklog:/);
    assert.match(context, /\[handoff\]/);
    assert.match(context, /\[session-summary\]/);

    return {
      dataHome,
      decisions: decisions.content[0].text,
      handoffs: handoffs.content[0].text,
      context,
    };
  } finally {
    restoreEnv(envSnapshot);
    if (cleanup && !options.dataHome) {
      await rm(dataHome, { recursive: true, force: true });
    }
  }
}

if (isMainModule()) {
  const result = await runTeamMemoryValidation();

  heading("Decision Query");
  process.stdout.write(`${result.decisions}\n`);

  heading("Handoff Query");
  process.stdout.write(`${result.handoffs}\n`);

  heading("SessionStart Context");
  process.stdout.write(`${result.context}\n`);

  heading("Result");
  process.stdout.write("Validation example passed.\n");
}
