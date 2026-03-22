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
  const impl = await resolveBackend(options);
  return impl.listRecentMemories(input, options);
}

export async function searchMemories(input = {}, options = {}) {
  const impl = await resolveBackend(options);
  return impl.searchMemories(input, options);
}

export async function getMemoryById(id, options = {}) {
  const impl = await resolveBackend(options);
  return impl.getMemoryById(id, options);
}

export async function storeMemory(input = {}, options = {}) {
  const impl = await resolveBackend(options);
  return impl.storeMemory(
    {
      ...input,
      agentId: input.agentId || getAgentId(options),
    },
    options,
  );
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
