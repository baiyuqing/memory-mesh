import test from "node:test";
import assert from "node:assert/strict";
import { getAgentId, getBackend, getRole, getTeamId } from "../plugin/scripts/lib/config.mjs";

function withEnv(patch, run) {
  const keys = Object.keys(patch);
  const previous = Object.fromEntries(keys.map((key) => [key, process.env[key]]));

  for (const [key, value] of Object.entries(patch)) {
    if (value === undefined) {
      delete process.env[key];
    } else {
      process.env[key] = value;
    }
  }

  try {
    return run();
  } finally {
    for (const [key, value] of Object.entries(previous)) {
      if (value === undefined) {
        delete process.env[key];
      } else {
        process.env[key] = value;
      }
    }
  }
}

test("Memory Mesh env vars are preferred over legacy aliases", () => {
  withEnv(
    {
      MEMORY_MESH_BACKEND: "mem9",
      MEMORY_MESH_AGENT_ID: "mesh-agent",
      MEMORY_MESH_TEAM_ID: "mesh-team",
      CLAUDE_CODE_MEMORY_BACKEND: "local",
      CLAUDE_CODE_MEMORY_AGENT_ID: "legacy-agent",
      CLAUDE_CODE_MEMORY_TEAM_ID: "legacy-team",
    },
    () => {
      assert.equal(getBackend(), "mem9");
      assert.equal(getAgentId(), "mesh-agent");
      assert.equal(getTeamId(), "mesh-team");
    },
  );
});

test("getRole reads MEMORY_MESH_ROLE and falls back to empty string", () => {
  withEnv({ MEMORY_MESH_ROLE: "pm" }, () => {
    assert.equal(getRole(), "pm");
  });
  withEnv({ MEMORY_MESH_ROLE: undefined }, () => {
    assert.equal(getRole(), "");
  });
  assert.equal(getRole({ role: "tech-lead" }), "tech-lead");
});

test("Legacy Claude Code env vars still work as fallback", () => {
  withEnv(
    {
      MEMORY_MESH_BACKEND: undefined,
      MEMORY_MESH_AGENT_ID: undefined,
      MEMORY_MESH_TEAM_ID: undefined,
      CLAUDE_CODE_MEMORY_BACKEND: "mem9",
      CLAUDE_CODE_MEMORY_AGENT_ID: "legacy-agent",
      CLAUDE_CODE_MEMORY_TEAM_ID: "legacy-team",
    },
    () => {
      assert.equal(getBackend(), "mem9");
      assert.equal(getAgentId(), "legacy-agent");
      assert.equal(getTeamId(), "legacy-team");
    },
  );
});
