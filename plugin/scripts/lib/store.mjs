import { getAgentId, getBackend } from "./config.mjs";
import { MAX_CONTEXT_MEMORIES, MAX_CONTEXT_SCAN_MEMORIES } from "./constants.mjs";
import * as localStore from "./local-store.mjs";

export const ensureStore = localStore.ensureStore;
export const formatMemory = localStore.formatMemory;

const BUILTIN_BACKENDS = {
  local: () => localStore,
  mem9: () => import("./mem9-client.mjs"),
};

const backendCache = new Map();

async function resolveBackend(options = {}) {
  const name = getBackend(options);

  if (backendCache.has(name)) {
    return backendCache.get(name);
  }

  let mod;
  if (BUILTIN_BACKENDS[name]) {
    mod = await BUILTIN_BACKENDS[name]();
  } else {
    // Treat as a module path (relative or absolute)
    mod = await import(name);
  }

  const required = ["listRecentMemories", "searchMemories", "getMemoryById", "storeMemory"];
  const missing = required.filter((fn) => typeof mod[fn] !== "function");
  if (missing.length > 0) {
    throw new Error(`Backend "${name}" is missing required exports: ${missing.join(", ")}`);
  }

  backendCache.set(name, mod);
  return mod;
}

export async function recordPrompt(input, options = {}) {
  return localStore.recordPrompt(
    {
      ...input,
      agentId: input.agentId || getAgentId(options),
    },
    options,
  );
}

export async function recordToolUse(input, options = {}) {
  return localStore.recordToolUse(
    {
      ...input,
      agentId: input.agentId || getAgentId(options),
    },
    options,
  );
}

export async function summarizeSession(input, options = {}) {
  const memory = await localStore.summarizeSession(
    {
      ...input,
      agentId: input.agentId || getAgentId(options),
    },
    options,
  );

  if (!memory) {
    return null;
  }

  const backend = getBackend(options);
  if (backend !== "local") {
    try {
      const impl = await resolveBackend(options);
      const remote = await impl.storeMemory(memory, options);
      return {
        ...memory,
        remote,
      };
    } catch (error) {
      return {
        ...memory,
        remoteError: error instanceof Error ? error.message : String(error),
      };
    }
  }

  return memory;
}

export async function listRecentMemories(input = {}, options = {}) {
  if (!isRemoteBackend(options)) {
    return localStore.listRecentMemories(input, options);
  }

  const impl = await resolveBackend(options);
  const [localR, remoteR] = await Promise.allSettled([
    localStore.listRecentMemories(input, options),
    impl.listRecentMemories(input, options),
  ]);

  return mergeMemories(
    localR.status === "fulfilled" ? localR.value : [],
    remoteR.status === "fulfilled" ? remoteR.value : [],
    input.limit || MAX_CONTEXT_SCAN_MEMORIES,
  );
}

export async function searchMemories(input = {}, options = {}) {
  if (!isRemoteBackend(options)) {
    return localStore.searchMemories(input, options);
  }

  const impl = await resolveBackend(options);
  const [localR, remoteR] = await Promise.allSettled([
    localStore.searchMemories(input, options),
    impl.searchMemories(input, options),
  ]);

  return mergeMemories(
    localR.status === "fulfilled" ? localR.value : [],
    remoteR.status === "fulfilled" ? remoteR.value : [],
    input.limit || MAX_CONTEXT_SCAN_MEMORIES,
  );
}

export async function getMemoryById(id, options = {}) {
  // Always try local first for verbatim fidelity
  const local = await localStore.getMemoryById(id, options);
  if (local) {
    return local;
  }

  if (!isRemoteBackend(options)) {
    return null;
  }

  // Fall back to remote for cross-agent memories
  try {
    const impl = await resolveBackend(options);
    return await impl.getMemoryById(id, options);
  } catch {
    return null;
  }
}

function isRemoteBackend(options = {}) {
  return getBackend(options) !== "local";
}

function mergeMemories(localMemories, remoteMemories, limit) {
  const seen = new Map();
  for (const m of localMemories) seen.set(m.id, m);
  for (const m of remoteMemories) {
    const localId = m.metadata?.localId;
    if (!seen.has(m.id) && !(localId && seen.has(localId))) {
      seen.set(m.id, m);
    }
  }
  return [...seen.values()]
    .sort((a, b) => (b.updatedAt || "").localeCompare(a.updatedAt || ""))
    .slice(0, limit);
}

export async function storeMemory(input = {}, options = {}) {
  const enriched = {
    ...input,
    agentId: input.agentId || getAgentId(options),
  };

  // Always write locally (verbatim source of truth)
  const local = await localStore.storeMemory(enriched, options);

  if (!isRemoteBackend(options)) {
    return local;
  }

  // Sync to remote for cross-agent sharing, attach localId for dedup
  try {
    const impl = await resolveBackend(options);
    const remote = await impl.storeMemory(
      { ...enriched, metadata: { ...enriched.metadata, localId: local.id } },
      options,
    );
    return { ...local, remote };
  } catch (error) {
    return {
      ...local,
      remoteError: error instanceof Error ? error.message : String(error),
    };
  }
}

export async function renderContextBlock(input = {}, options = {}) {
  const requestedLimit = input.limit || MAX_CONTEXT_MEMORIES;
  const memories = await listRecentMemories(
    {
      cwd: input.cwd,
      projectKey: input.projectKey,
      limit: Math.max(requestedLimit, MAX_CONTEXT_SCAN_MEMORIES),
    },
    options,
  );

  return localStore.renderContextFromMemories(memories, {
    ...input,
    limit: requestedLimit,
  });
}
