import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, rm, readdir, readFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import {
  getMemoryById,
  listRecentMemories,
  searchMemories,
  storeMemory,
} from "../plugin/scripts/lib/store.mjs";

async function withMem9Store(run) {
  const dataHome = await mkdtemp(join(tmpdir(), "memory-mesh-dual-"));
  const previousEnv = {
    MEMORY_MESH_HOME: process.env.MEMORY_MESH_HOME,
    MEMORY_MESH_BACKEND: process.env.MEMORY_MESH_BACKEND,
    MEMORY_MESH_AGENT_ID: process.env.MEMORY_MESH_AGENT_ID,
    CLAUDE_CODE_MEMORY_HOME: process.env.CLAUDE_CODE_MEMORY_HOME,
    CLAUDE_CODE_MEMORY_BACKEND: process.env.CLAUDE_CODE_MEMORY_BACKEND,
    MEM9_API_URL: process.env.MEM9_API_URL,
    MEM9_TENANT_ID: process.env.MEM9_TENANT_ID,
    CLAUDE_CODE_MEMORY_AGENT_ID: process.env.CLAUDE_CODE_MEMORY_AGENT_ID,
  };

  process.env.MEMORY_MESH_HOME = dataHome;
  process.env.MEMORY_MESH_BACKEND = "mem9";
  process.env.MEM9_API_URL = "https://mem9.example.test";
  process.env.MEM9_TENANT_ID = "tenant-dual";
  process.env.MEMORY_MESH_AGENT_ID = "agent-a";

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

function mockFetch(responses = []) {
  const requests = [];
  const previous = globalThis.fetch;
  let callIndex = 0;

  globalThis.fetch = async (url, init = {}) => {
    requests.push({ url: String(url), init });
    const resp = responses[callIndex] || responses[responses.length - 1] || { status: "accepted" };
    callIndex++;
    return {
      ok: true,
      status: resp.httpStatus || 200,
      async json() { return resp; },
      async text() { return JSON.stringify(resp); },
    };
  };

  return { requests, restore: () => { globalThis.fetch = previous; } };
}

test("storeMemory dual-writes to local and remote when backend=mem9", async () => {
  await withMem9Store(async (dataHome) => {
    const { requests, restore } = mockFetch([{ status: "accepted" }]);
    try {
      const result = await storeMemory({
        cwd: process.cwd(),
        content: "Use getSpaceId() from project.mjs to derive space isolation key. Function signature: getSpaceId(cwd) => string. Falls back to directory basename.",
        memoryType: "handoff",
        title: "Space isolation handoff",
      });

      // Remote was called
      assert.equal(requests.length, 1);
      assert.match(requests[0].url, /mem9\.example\.test/);
      const body = JSON.parse(requests[0].init.body);
      assert.ok(body.metadata.localId, "remote payload should include localId for dedup");

      // Local has full verbatim content
      assert.ok(result.id, "should have local memory ID");
      assert.ok(result.remote, "should have remote result");

      // Verify local file has complete content (not truncated)
      const memoriesDir = join(dataHome, "memories");
      const files = await readdir(memoriesDir);
      assert.ok(files.length >= 1, "local memory file should exist");
      const localFile = JSON.parse(await readFile(join(memoriesDir, files[0]), "utf8"));
      assert.match(localFile.summary, /getSpaceId/);
    } finally {
      restore();
    }
  });
});

test("listRecentMemories merges local and remote, local wins on dedup", async () => {
  await withMem9Store(async (dataHome) => {
    // Write a local memory first (via local backend directly)
    const localResult = await storeMemory(
      { cwd: process.cwd(), content: "Local verbatim content with full details", memoryType: "decision" },
      { dataHome, backend: "local" },
    );

    // Mock remote to return: same memory (truncated) + a cross-agent memory
    const { restore } = mockFetch([{
      memories: [
        {
          id: "remote-version",
          content: "Local verbatim...",
          tags: ["kind:decision"],
          agent_id: "agent-a",
          metadata: { localId: localResult.id, memoryType: "decision" },
          created_at: "2026-03-22T10:00:00Z",
          updated_at: "2026-03-22T10:00:00Z",
        },
        {
          id: "cross-agent-1",
          content: "Decision from another agent on a different machine",
          tags: ["kind:decision"],
          agent_id: "agent-b",
          metadata: { memoryType: "decision" },
          created_at: "2026-03-22T09:00:00Z",
          updated_at: "2026-03-22T09:00:00Z",
        },
      ],
      total: 2,
      limit: 20,
      offset: 0,
    }]);

    try {
      const memories = await listRecentMemories({ cwd: process.cwd(), limit: 20 });

      // Should have 2: local version (wins over remote duplicate) + cross-agent
      assert.equal(memories.length, 2);

      // Local version should win (has full content)
      const localMemory = memories.find((m) => m.id === localResult.id);
      assert.ok(localMemory, "local memory should be present by its original ID");
      assert.match(localMemory.summary, /full details/);

      // Cross-agent memory should also be present
      const crossAgent = memories.find((m) => m.id === "cross-agent-1");
      assert.ok(crossAgent, "cross-agent memory from remote should be included");
    } finally {
      restore();
    }
  });
});

test("listRecentMemories degrades gracefully when remote fails", async () => {
  await withMem9Store(async (dataHome) => {
    // Write a local memory
    await storeMemory(
      { cwd: process.cwd(), content: "Resilient local memory", memoryType: "goal" },
      { dataHome, backend: "local" },
    );

    // Mock remote to fail
    const previous = globalThis.fetch;
    globalThis.fetch = async () => { throw new Error("network timeout"); };

    try {
      const memories = await listRecentMemories({ cwd: process.cwd(), limit: 20 });
      assert.ok(memories.length >= 1, "should return local memories when remote fails");
      assert.match(memories[0].summary, /Resilient/);
    } finally {
      globalThis.fetch = previous;
    }
  });
});

test("getMemoryById prefers local, falls back to remote", async () => {
  await withMem9Store(async (dataHome) => {
    // Write a local memory
    const localResult = await storeMemory(
      { cwd: process.cwd(), content: "Full local content", memoryType: "decision" },
      { dataHome, backend: "local" },
    );

    const { requests, restore } = mockFetch([]);
    // Override with a smarter mock that returns 404 for unknown IDs
    const previousFetch = globalThis.fetch;
    globalThis.fetch = async (url, init = {}) => {
      requests.push({ url: String(url), init });
      const urlStr = String(url);
      if (urlStr.includes("remote-only-id")) {
        return {
          ok: true,
          status: 200,
          async json() {
            return {
              id: "remote-only-id",
              content: "Remote content",
              tags: ["kind:constraint"],
              agent_id: "agent-b",
              metadata: { memoryType: "constraint" },
              created_at: "2026-03-22T10:00:00Z",
              updated_at: "2026-03-22T10:00:00Z",
            };
          },
          async text() { return ""; },
        };
      }
      return { ok: false, status: 404, async json() { return null; }, async text() { return "not found"; } };
    };

    try {
      // Local ID: should NOT hit remote
      const local = await getMemoryById(localResult.id);
      assert.ok(local);
      assert.match(local.summary, /Full local/);
      assert.equal(requests.length, 0, "should not call remote when local has the memory");

      // Remote ID: should hit remote
      const remote = await getMemoryById("remote-only-id");
      assert.ok(remote);
      assert.equal(requests.length, 1, "should call remote for unknown ID");
    } finally {
      globalThis.fetch = previousFetch;
      restore();
    }
  });
});

test("backend=local does not dual-write or merge", async () => {
  const dataHome = await mkdtemp(join(tmpdir(), "memory-mesh-local-only-"));
  const savedEnv = {
    MEMORY_MESH_BACKEND: process.env.MEMORY_MESH_BACKEND,
    MEM9_API_KEY: process.env.MEM9_API_KEY,
    MEM9_TENANT_ID: process.env.MEM9_TENANT_ID,
  };
  delete process.env.MEMORY_MESH_BACKEND;
  delete process.env.MEM9_API_KEY;
  delete process.env.MEM9_TENANT_ID;

  const previous = globalThis.fetch;
  let fetchCalled = false;
  globalThis.fetch = async () => { fetchCalled = true; return { ok: true, status: 200, async json() { return {}; } }; };

  try {
    await storeMemory({ cwd: process.cwd(), content: "Local only", memoryType: "decision" }, { dataHome });
    assert.equal(fetchCalled, false, "fetch should never be called when backend=local");

    const memories = await listRecentMemories({ cwd: process.cwd() }, { dataHome });
    assert.equal(fetchCalled, false, "fetch should never be called for reads when backend=local");
    assert.ok(memories.length >= 1);
  } finally {
    globalThis.fetch = previous;
    for (const [key, value] of Object.entries(savedEnv)) {
      if (value !== undefined) process.env[key] = value;
    }
    await rm(dataHome, { recursive: true, force: true });
  }
});
