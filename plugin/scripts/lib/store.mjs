import { getAgentId, getBackend } from "./config.mjs";
import * as localStore from "./local-store.mjs";
import * as mem9Client from "./mem9-client.mjs";

export const ensureStore = localStore.ensureStore;
export const formatMemory = localStore.formatMemory;

function backendImpl(options = {}) {
  return getBackend(options) === "mem9" ? mem9Client : localStore;
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

  if (getBackend(options) === "mem9") {
    try {
      const remote = await mem9Client.storeMemory(memory, options);
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
  return backendImpl(options).listRecentMemories(input, options);
}

export async function searchMemories(input = {}, options = {}) {
  return backendImpl(options).searchMemories(input, options);
}

export async function getMemoryById(id, options = {}) {
  return backendImpl(options).getMemoryById(id, options);
}

export async function storeMemory(input = {}, options = {}) {
  return backendImpl(options).storeMemory(
    {
      ...input,
      agentId: input.agentId || getAgentId(options),
    },
    options,
  );
}

export async function renderContextBlock(input = {}, options = {}) {
  const memories = await listRecentMemories(
    {
      cwd: input.cwd,
      projectKey: input.projectKey,
      limit: input.limit,
      memoryType: input.memoryType,
    },
    options,
  );

  return localStore.renderContextFromMemories(memories, input);
}
