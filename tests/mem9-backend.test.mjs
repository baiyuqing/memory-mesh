import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { listRecentMemories, recordPrompt, storeMemory, summarizeSession } from "../plugin/scripts/lib/store.mjs";
import { getProjectContext } from "../plugin/scripts/lib/project.mjs";

async function withTempStore(run) {
  const dataHome = await mkdtemp(join(tmpdir(), "memory-mesh-mem9-"));
  const previousEnv = {
    MEMORY_MESH_HOME: process.env.MEMORY_MESH_HOME,
    MEMORY_MESH_BACKEND: process.env.MEMORY_MESH_BACKEND,
    MEMORY_MESH_AGENT_ID: process.env.MEMORY_MESH_AGENT_ID,
    CLAUDE_CODE_MEMORY_HOME: process.env.CLAUDE_CODE_MEMORY_HOME,
    CLAUDE_CODE_MEMORY_BACKEND: process.env.CLAUDE_CODE_MEMORY_BACKEND,
    MEM9_API_URL: process.env.MEM9_API_URL,
    MEM9_API_KEY: process.env.MEM9_API_KEY,
    MEM9_TENANT_ID: process.env.MEM9_TENANT_ID,
    CLAUDE_CODE_MEMORY_AGENT_ID: process.env.CLAUDE_CODE_MEMORY_AGENT_ID,
  };

  process.env.MEMORY_MESH_HOME = dataHome;
  process.env.MEMORY_MESH_BACKEND = "mem9";
  process.env.MEM9_API_URL = "https://mem9.example.test";
  delete process.env.MEM9_API_KEY;
  process.env.MEM9_TENANT_ID = "tenant-123";
  process.env.MEMORY_MESH_AGENT_ID = "codex";

  try {
    await run(dataHome);
  } finally {
    for (const [key, value] of Object.entries(previousEnv)) {
      if (value === undefined) {
        delete process.env[key];
      } else {
        process.env[key] = value;
      }
    }
    await rm(dataHome, { recursive: true, force: true });
  }
}

test("mem9 backend stores summarized sessions into the shared tenant API", async () => {
  await withTempStore(async () => {
    const expectedProjectKey = getProjectContext(process.cwd()).projectKey;
    const requests = [];
    const previousFetch = globalThis.fetch;
    globalThis.fetch = async (url, init = {}) => {
      requests.push({ url: String(url), init });
      return {
        ok: true,
        status: 202,
        async json() {
          return { status: "accepted" };
        },
        async text() {
          return JSON.stringify({ status: "accepted" });
        },
      };
    };

    try {
      const cwd = process.cwd();
      await recordPrompt({ sessionId: "shared-session", cwd, prompt: "Share architecture decisions across agents" });
      const memory = await summarizeSession({ sessionId: "shared-session", cwd });

      assert.equal(requests.length, 1);
      assert.match(requests[0].url, /https:\/\/mem9\.example\.test\/v1alpha1\/mem9s\/tenant-123\/memories/);
      assert.equal(requests[0].init.method, "POST");

      const body = JSON.parse(requests[0].init.body);
      assert.equal(body.agent_id, "codex");
      assert.match(body.content, /Share architecture decisions across agents/);
      assert.ok(body.tags.includes(`project:${expectedProjectKey}`));
      assert.equal(memory.remote.accepted, true);
    } finally {
      globalThis.fetch = previousFetch;
    }
  });
});

test("mem9 backend reads shared memories for the current project", async () => {
  await withTempStore(async () => {
    const project = getProjectContext(process.cwd());
    const previousFetch = globalThis.fetch;
    globalThis.fetch = async () => ({
      ok: true,
      status: 200,
      async json() {
        return {
          memories: [
            {
              id: "remote-1",
              content: "Codex fixed the build pipeline and standardized release tags.",
              tags: [`project:${project.projectKey}`, "team:platform", "kind:session-summary"],
              agent_id: "codex",
              metadata: {
                projectKey: project.projectKey,
                projectLabel: project.projectLabel,
                workspaceLabel: project.workspaceLabel,
                agentId: "codex",
                title: "Build pipeline stabilized",
                request: "Fix the build pipeline",
                latestPrompt: "Fix the build pipeline",
              },
              created_at: "2026-03-22T02:00:00.000Z",
              updated_at: "2026-03-22T02:00:00.000Z",
            },
          ],
          total: 1,
          limit: 5,
          offset: 0,
        };
      },
      async text() {
        return "";
      },
    });

    try {
      const memories = await listRecentMemories({ cwd: process.cwd(), limit: 5 }, { teamId: "platform" });
      assert.equal(memories.length, 1);
      assert.equal(memories[0].id, "remote-1");
      assert.equal(memories[0].agentId, "codex");
      assert.match(memories[0].summary, /build pipeline/i);
    } finally {
      globalThis.fetch = previousFetch;
    }
  });
});

test("mem9 backend preserves typed tags when storing explicit durable memory", async () => {
  await withTempStore(async () => {
    const expectedProjectKey = getProjectContext(process.cwd()).projectKey;
    const requests = [];
    const previousFetch = globalThis.fetch;
    globalThis.fetch = async (url, init = {}) => {
      requests.push({ url: String(url), init });
      return {
        ok: true,
        status: 202,
        async json() {
          return { status: "accepted" };
        },
        async text() {
          return JSON.stringify({ status: "accepted" });
        },
      };
    };

    try {
      await storeMemory({
        cwd: process.cwd(),
        content: "The team must keep old mysqlbench code on its own branch.",
        memoryType: "constraint",
        tags: ["area:git"],
      });

      assert.equal(requests.length, 1);
      const body = JSON.parse(requests[0].init.body);
      assert.ok(body.tags.includes("kind:constraint"));
      assert.ok(body.tags.includes(`project:${expectedProjectKey}`));
      assert.ok(body.tags.includes("agent:codex"));
      assert.ok(body.tags.includes("area:git"));
      assert.equal(body.metadata.memoryType, "constraint");
    } finally {
      globalThis.fetch = previousFetch;
    }
  });
});
