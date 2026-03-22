import { getAgentId, getRole, getTeamId } from "./config.mjs";
import { MAX_CONTEXT_MEMORIES, MAX_SEARCH_RESULTS } from "./constants.mjs";
import { getProjectContext } from "./project.mjs";

function nowIso() {
  return new Date().toISOString();
}

function truncate(text, limit) {
  if (!text) {
    return "";
  }
  return text.length > limit ? `${text.slice(0, limit - 1)}…` : text;
}

function unique(values) {
  return [...new Set(values.filter(Boolean))];
}

function normalizeMemoryType(value, fallback = "") {
  return typeof value === "string" && value.trim() ? value.trim() : fallback;
}

function parseMetadata(raw) {
  if (!raw) {
    return {};
  }
  if (typeof raw === "object") {
    return raw;
  }
  try {
    return JSON.parse(raw);
  } catch {
    return {};
  }
}

function buildConfig(options = {}) {
  const apiUrl = options.mem9ApiUrl || process.env.MEM9_API_URL || "https://api.mem9.ai";
  const apiKey = options.mem9ApiKey || process.env.MEM9_API_KEY || "";
  const tenantId = options.mem9TenantId || process.env.MEM9_TENANT_ID || "";
  const agentId = getAgentId(options);

  if (!apiKey && !tenantId) {
    throw new Error("mem9 backend requires MEM9_API_KEY or MEM9_TENANT_ID");
  }

  return {
    apiUrl: apiUrl.replace(/\/$/, ""),
    apiKey,
    tenantId,
    agentId,
  };
}

function buildBaseUrl(config) {
  if (config.apiKey) {
    return `${config.apiUrl}/v1alpha2/mem9s`;
  }
  return `${config.apiUrl}/v1alpha1/mem9s/${config.tenantId}`;
}

function buildHeaders(config) {
  const headers = {
    "Content-Type": "application/json",
    "X-Mnemo-Agent-Id": config.agentId,
  };
  if (config.apiKey) {
    headers["X-API-Key"] = config.apiKey;
  }
  return headers;
}

async function request(path, init = {}, options = {}) {
  const config = buildConfig(options);
  const response = await fetch(`${buildBaseUrl(config)}${path}`, {
    ...init,
    headers: {
      ...buildHeaders(config),
      ...init.headers,
    },
  });

  if (!response.ok) {
    throw new Error(`mem9 request failed (${response.status}): ${await response.text()}`);
  }

  if (response.status === 204) {
    return null;
  }

  return response.json();
}

function buildProjectTags(project, options = {}, extraTags = [], agentId = getAgentId(options)) {
  return unique([
    `project:${project.projectKey}`,
    `workspace:${project.workspaceLabel}`,
    `agent:${agentId}`,
    getTeamId(options) ? `team:${getTeamId(options)}` : "",
    getRole(options) ? `role:${getRole(options)}` : "",
    ...extraTags,
  ]);
}

function inferMemoryType(memory, metadata) {
  return normalizeMemoryType(
    memory.memory_type,
    normalizeMemoryType(
      metadata.memoryType,
      (memory.tags || []).find((tag) => tag.startsWith("kind:"))?.slice("kind:".length) || "explicit",
    ),
  );
}

function normalizeMemory(memory, input = {}, options = {}) {
  const metadata = parseMetadata(memory.metadata);
  const project = input.cwd ? getProjectContext(input.cwd) : null;
  const projectKey = metadata.projectKey || project?.projectKey || "";
  const projectLabel = metadata.projectLabel || project?.projectLabel || projectKey || "unknown";
  const workspaceLabel = metadata.workspaceLabel || project?.workspaceLabel || projectLabel;
  const content = memory.content || "";

  return {
    id: memory.id,
    agentId: memory.agent_id || memory.source || metadata.agentId || "",
    projectKey,
    projectLabel,
    workspaceLabel,
    cwd: metadata.cwd || input.cwd || "",
    gitRoot: metadata.gitRoot || "",
    startedAt: metadata.startedAt || memory.created_at || nowIso(),
    updatedAt: memory.updated_at || memory.created_at || nowIso(),
    promptCount: metadata.promptCount || 0,
    toolEventCount: metadata.toolEventCount || 0,
    memoryType: inferMemoryType(memory, metadata),
    title: metadata.title || truncate(content.split("\n")[0] || content, 80),
    request: metadata.request || truncate(content, 200),
    latestPrompt: metadata.latestPrompt || truncate(content, 200),
    summary: content,
    keyActions: metadata.keyActions || [],
    filesChanged: metadata.filesChanged || [],
    filesRead: metadata.filesRead || [],
    commands: metadata.commands || [],
    tools: metadata.tools || [],
    tags: memory.tags || [],
    metadata,
  };
}

function buildSearchParams(input = {}, options = {}) {
  const params = new URLSearchParams();
  const limit = String(input.limit || MAX_SEARCH_RESULTS);
  params.set("limit", limit);

  if (input.query?.trim()) {
    params.set("q", input.query.trim());
  }

  const projectKey = input.projectKey || (input.cwd ? getProjectContext(input.cwd).projectKey : "");
  const tags = unique([
    projectKey ? `project:${projectKey}` : "",
    getTeamId(options) ? `team:${getTeamId(options)}` : "",
    input.memoryType ? `kind:${input.memoryType}` : "",
  ]);

  if (tags.length > 0) {
    params.set("tags", tags.join(","));
  }

  return params;
}

export async function listRecentMemories(input = {}, options = {}) {
  const params = buildSearchParams(
    {
      ...input,
      limit: input.limit || MAX_CONTEXT_MEMORIES,
    },
    options,
  );
  const response = await request(`/memories?${params.toString()}`, {}, options);
  return (response?.memories || []).map((memory) => normalizeMemory(memory, input, options));
}

export async function searchMemories(input = {}, options = {}) {
  const params = buildSearchParams(input, options);
  const response = await request(`/memories?${params.toString()}`, {}, options);
  return (response?.memories || []).map((memory) => normalizeMemory(memory, input, options));
}

export async function getMemoryById(id, options = {}) {
  if (!id) {
    return null;
  }
  const response = await request(`/memories/${id}`, {}, options);
  return normalizeMemory(response, {}, options);
}

export async function storeMemory(input = {}, options = {}) {
  const project = getProjectContext(input.cwd || process.cwd());
  const memoryType = normalizeMemoryType(input.memoryType, "explicit");
  const agentId = input.agentId || getAgentId(options);
  const tags = unique([
    ...buildProjectTags(project, options, [`kind:${memoryType}`], agentId),
    ...(input.tags || []),
  ]);
  const metadata = {
    ...input.metadata,
    backend: "mem9",
    title: input.title || truncate(input.summary || input.content || "", 80),
    request: input.request || truncate(input.content || input.summary || "", 200),
    latestPrompt: input.latestPrompt || truncate(input.content || input.summary || "", 200),
    projectKey: input.projectKey || project.projectKey,
    projectLabel: input.projectLabel || project.projectLabel,
    workspaceLabel: input.workspaceLabel || project.workspaceLabel,
    cwd: input.cwd || project.cwd,
    gitRoot: project.gitRoot,
    agentId,
    promptCount: input.promptCount || 0,
    toolEventCount: input.toolEventCount || 0,
    filesChanged: input.filesChanged || [],
    filesRead: input.filesRead || [],
    commands: input.commands || [],
    tools: input.tools || [],
    keyActions: input.keyActions || [],
    memoryType,
    startedAt: input.startedAt || nowIso(),
  };

  const content = truncate(input.summary || input.content || input.latestPrompt || input.request || "", 2000);
  if (!content) {
    return null;
  }

  await request(
    "/memories",
    {
      method: "POST",
      body: JSON.stringify({
        content,
        agent_id: agentId,
        tags,
        metadata,
      }),
    },
    options,
  );

  return {
    accepted: true,
    summary: content,
    tags,
    metadata,
  };
}
