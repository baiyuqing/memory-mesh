import { createHash } from "node:crypto";
import { mkdir, readFile, readdir, rename, rm, stat, writeFile } from "node:fs/promises";
import { existsSync } from "node:fs";
import { join } from "node:path";
import {
  ACTIVITY_MEMORY_TYPES,
  CHANGE_TOOLS,
  DEFAULT_DATA_HOME,
  DURABLE_MEMORY_TYPES,
  IMPORTANT_TOOLS,
  MAX_CONTEXT_ACTIVITY_MEMORIES,
  MAX_CONTEXT_DURABLE_MEMORIES,
  MAX_CONTEXT_MEMORIES,
  MAX_PROMPT_LENGTH,
  MAX_SEARCH_RESULTS,
  MAX_VALUE_PREVIEW,
  READ_TOOLS,
} from "./constants.mjs";
import { getAgentId, getTeamId } from "./config.mjs";
import { getProjectContext } from "./project.mjs";

function nowIso() {
  return new Date().toISOString();
}

function hashId(value) {
  return createHash("sha1").update(String(value)).digest("hex");
}

function truncate(text, limit) {
  if (!text) {
    return "";
  }
  return text.length > limit ? `${text.slice(0, limit - 1)}…` : text;
}

function stringifyPreview(value, limit = MAX_VALUE_PREVIEW) {
  if (value === undefined || value === null) {
    return "";
  }

  const text = typeof value === "string" ? value : JSON.stringify(value);
  return truncate(text, limit);
}

function unique(values) {
  return [...new Set(values.filter(Boolean))];
}

function normalizeMemoryType(value, fallback = "") {
  return typeof value === "string" && value.trim() ? value.trim() : fallback;
}

function normalizePrompt(prompt) {
  const text = typeof prompt === "string" ? prompt.trim() : "";
  return truncate(text || "[non-text prompt]", MAX_PROMPT_LENGTH);
}

function buildPaths(options = {}) {
  const dataHome = options.dataHome || process.env.CLAUDE_CODE_MEMORY_HOME || DEFAULT_DATA_HOME;
  return {
    dataHome,
    sessionsDir: join(dataHome, "sessions"),
    memoriesDir: join(dataHome, "memories"),
  };
}

export async function ensureStore(options = {}) {
  const paths = buildPaths(options);
  await mkdir(paths.sessionsDir, { recursive: true });
  await mkdir(paths.memoriesDir, { recursive: true });
  return paths;
}

async function writeJsonAtomic(path, value) {
  const tempPath = `${path}.${process.pid}.${Date.now()}.tmp`;
  await writeFile(tempPath, JSON.stringify(value, null, 2));
  await rename(tempPath, path);
}

async function withLock(targetPath, work) {
  const lockPath = `${targetPath}.lock`;

  for (let attempt = 0; attempt < 50; attempt += 1) {
    try {
      await mkdir(lockPath);
      try {
        return await work();
      } finally {
        await rm(lockPath, { recursive: true, force: true });
      }
    } catch (error) {
      if (error?.code !== "EEXIST") {
        throw error;
      }
      await new Promise((resolve) => setTimeout(resolve, 20));
    }
  }

  throw new Error(`Timed out waiting for lock: ${lockPath}`);
}

async function readJsonOrNull(path) {
  if (!existsSync(path)) {
    return null;
  }
  const raw = await readFile(path, "utf8");
  return JSON.parse(raw);
}

function sessionFile(paths, sessionId) {
  return join(paths.sessionsDir, `${hashId(sessionId)}.json`);
}

function memoryFile(paths, sessionId) {
  return join(paths.memoriesDir, `${hashId(sessionId)}.json`);
}

function extractStrings(value, include) {
  const results = [];

  function walk(current, key = "", depth = 0) {
    if (depth > 5 || current === null || current === undefined) {
      return;
    }

    if (typeof current === "string") {
      if (include(key, current)) {
        results.push(current);
      }
      return;
    }

    if (Array.isArray(current)) {
      for (const item of current) {
        walk(item, key, depth + 1);
      }
      return;
    }

    if (typeof current === "object") {
      for (const [childKey, childValue] of Object.entries(current)) {
        walk(childValue, childKey, depth + 1);
      }
    }
  }

  walk(value);
  return unique(results);
}

function extractPaths(toolInput, toolResponse) {
  const matcher = (key, value) => {
    const lowerKey = key.toLowerCase();
    return /(path|file|files|target|targets)/.test(lowerKey) && /[/.]/.test(value);
  };

  return unique([
    ...extractStrings(toolInput, matcher),
    ...extractStrings(toolResponse, matcher),
  ]).slice(0, 20);
}

function extractCommand(toolInput) {
  const commands = extractStrings(toolInput, (key) => /^(command|cmd|bash_command)$/.test(key.toLowerCase()));
  return commands[0] ? truncate(commands[0], 240) : null;
}

function classifyPaths(toolName, filePaths) {
  if (CHANGE_TOOLS.has(toolName)) {
    return {
      filesChanged: filePaths,
      filesRead: [],
    };
  }

  if (READ_TOOLS.has(toolName)) {
    return {
      filesChanged: [],
      filesRead: filePaths,
    };
  }

  return {
    filesChanged: [],
    filesRead: filePaths,
  };
}

function interestingEvents(session) {
  return session.tools.filter((tool) => IMPORTANT_TOOLS.has(tool.toolName)).slice(-8);
}

function buildActionLine(tool) {
  const parts = [tool.toolName];
  if (tool.command) {
    parts.push(`ran \`${tool.command}\``);
  }
  if (tool.filesChanged.length > 0) {
    parts.push(`changed ${tool.filesChanged.slice(0, 3).join(", ")}`);
  }
  if (tool.filesRead.length > 0 && tool.filesChanged.length === 0) {
    parts.push(`looked at ${tool.filesRead.slice(0, 3).join(", ")}`);
  }
  if (parts.length === 1 && tool.inputPreview) {
    parts.push(truncate(tool.inputPreview, 120));
  }
  return parts.join(" ");
}

function buildSharedTags(project, options = {}, extraTags = [], agentId = getAgentId(options)) {
  return unique([
    `project:${project.projectKey}`,
    `workspace:${project.workspaceLabel}`,
    `agent:${agentId}`,
    getTeamId(options) ? `team:${getTeamId(options)}` : "",
    ...extraTags,
  ]);
}

function memoryTypeFromTags(tags = []) {
  const tag = tags.find((value) => value.startsWith("kind:"));
  return tag ? tag.slice("kind:".length) : "";
}

export function getMemoryType(memory) {
  return normalizeMemoryType(
    memory?.memoryType,
    normalizeMemoryType(memory?.metadata?.memoryType, memoryTypeFromTags(memory?.tags || []) || "explicit"),
  );
}

function requestedMemoryTypes(input = {}) {
  return unique([
    normalizeMemoryType(input.memoryType),
    ...(Array.isArray(input.memoryTypes) ? input.memoryTypes.map((value) => normalizeMemoryType(value)) : []),
  ]);
}

function filterByMemoryTypes(memories, input = {}) {
  const allowed = new Set(requestedMemoryTypes(input));
  if (allowed.size === 0) {
    return memories;
  }
  return memories.filter((memory) => allowed.has(getMemoryType(memory)));
}

function buildSummary(session, options = {}) {
  const firstPrompt = session.prompts[0]?.text || "";
  const latestPrompt = session.prompts.at(-1)?.text || firstPrompt || "Continue work";
  const events = interestingEvents(session);
  const filesChanged = unique(events.flatMap((tool) => tool.filesChanged)).slice(0, 10);
  const filesRead = unique(events.flatMap((tool) => tool.filesRead)).slice(0, 10);
  const commands = unique(events.map((tool) => tool.command).filter(Boolean)).slice(0, 6);
  const tools = unique(events.map((tool) => tool.toolName)).slice(0, 10);
  const keyActions = events.map(buildActionLine);

  let summary = `Latest intent: ${latestPrompt}`;
  if (filesChanged.length > 0) {
    summary += `. Changed ${filesChanged.slice(0, 4).join(", ")}`;
  }
  if (commands.length > 0) {
    summary += `. Ran ${commands.slice(0, 2).join(" | ")}`;
  }
  if (tools.length > 0) {
    summary += `. Main tools: ${tools.join(", ")}`;
  }

  return {
    title: truncate(latestPrompt, 80),
    request: truncate(firstPrompt, 200),
    latestPrompt: truncate(latestPrompt, 200),
    summary,
    keyActions,
    filesChanged,
    filesRead,
    commands,
    tools,
    tags: buildSharedTags(
      {
        projectKey: session.projectKey,
        workspaceLabel: session.workspaceLabel,
      },
      options,
      ["kind:session-summary"],
      session.agentId || getAgentId(options),
    ),
  };
}

function stringifyMetadata(metadata) {
  if (!metadata) {
    return "";
  }
  if (typeof metadata === "string") {
    return metadata;
  }
  return JSON.stringify(metadata);
}

function memorySearchText(memory) {
  return [
    memory.projectKey,
    memory.projectLabel,
    memory.workspaceLabel,
    memory.agentId,
    getMemoryType(memory),
    memory.request,
    memory.latestPrompt,
    memory.summary,
    ...(memory.tags || []),
    ...memory.keyActions,
    ...memory.filesChanged,
    ...memory.filesRead,
    ...memory.commands,
    ...memory.tools,
    stringifyMetadata(memory.metadata),
  ]
    .filter(Boolean)
    .join("\n")
    .toLowerCase();
}

async function upsertSession(update, options = {}) {
  const paths = await ensureStore(options);
  const path = sessionFile(paths, update.sessionId);
  const project = getProjectContext(update.cwd || process.cwd());
  const agentId = update.agentId || getAgentId(options);

  return withLock(path, async () => {
    const current = (await readJsonOrNull(path)) || {
      sessionId: update.sessionId,
      agentId,
      projectKey: project.projectKey,
      projectLabel: project.projectLabel,
      workspaceLabel: project.workspaceLabel,
      cwd: update.cwd || project.cwd,
      gitRoot: project.gitRoot,
      startedAt: nowIso(),
      updatedAt: nowIso(),
      prompts: [],
      tools: [],
    };

    current.agentId = agentId;
    current.projectKey = project.projectKey;
    current.projectLabel = project.projectLabel;
    current.workspaceLabel = project.workspaceLabel;
    current.cwd = update.cwd || current.cwd;
    current.gitRoot = project.gitRoot;
    current.updatedAt = update.createdAt || nowIso();

    if (update.prompt) {
      current.prompts.push({
        text: normalizePrompt(update.prompt),
        createdAt: update.createdAt || nowIso(),
      });
    }

    if (update.toolName) {
      const filePaths = extractPaths(update.toolInput, update.toolResponse);
      const { filesChanged, filesRead } = classifyPaths(update.toolName, filePaths);
      current.tools.push({
        toolName: update.toolName,
        createdAt: update.createdAt || nowIso(),
        inputPreview: stringifyPreview(update.toolInput),
        responsePreview: stringifyPreview(update.toolResponse),
        command: extractCommand(update.toolInput),
        filesChanged,
        filesRead,
      });
    }

    await writeJsonAtomic(path, current);
    return current;
  });
}

export async function recordPrompt(input, options = {}) {
  if (!input.sessionId) {
    return null;
  }

  return upsertSession(
    {
      sessionId: input.sessionId,
      cwd: input.cwd,
      prompt: input.prompt,
      agentId: input.agentId,
      createdAt: input.createdAt,
    },
    options,
  );
}

export async function recordToolUse(input, options = {}) {
  if (!input.sessionId || !input.toolName) {
    return null;
  }

  return upsertSession(
    {
      sessionId: input.sessionId,
      cwd: input.cwd,
      toolName: input.toolName,
      toolInput: input.toolInput,
      toolResponse: input.toolResponse,
      agentId: input.agentId,
      createdAt: input.createdAt,
    },
    options,
  );
}

export async function summarizeSession(input, options = {}) {
  if (!input.sessionId) {
    return null;
  }

  const paths = await ensureStore(options);
  const sessionPath = sessionFile(paths, input.sessionId);
  const session = await readJsonOrNull(sessionPath);

  if (!session) {
    return null;
  }

  const summary = buildSummary(session, options);
  const memory = {
    id: session.sessionId,
    agentId: session.agentId || getAgentId(options),
    projectKey: session.projectKey,
    projectLabel: session.projectLabel,
    workspaceLabel: session.workspaceLabel,
    cwd: session.cwd,
    gitRoot: session.gitRoot,
    startedAt: session.startedAt,
    updatedAt: input.createdAt || nowIso(),
    promptCount: session.prompts.length,
    toolEventCount: session.tools.length,
    memoryType: "session-summary",
    metadata: {
      backend: "local",
      projectKey: session.projectKey,
      projectLabel: session.projectLabel,
      workspaceLabel: session.workspaceLabel,
      agentId: session.agentId || getAgentId(options),
      memoryType: "session-summary",
    },
    ...summary,
  };

  await writeJsonAtomic(memoryFile(paths, input.sessionId), memory);
  return memory;
}

async function loadMemoryFiles(options = {}) {
  const paths = await ensureStore(options);
  const fileNames = await readdir(paths.memoriesDir);
  const memories = [];

  for (const fileName of fileNames) {
    const path = join(paths.memoriesDir, fileName);
    const fileStat = await stat(path);
    if (!fileStat.isFile()) {
      continue;
    }
    const memory = await readJsonOrNull(path);
    if (memory) {
      memories.push(memory);
    }
  }

  memories.sort((left, right) => right.updatedAt.localeCompare(left.updatedAt));
  return memories;
}

function scoreMemory(memory, query) {
  if (!query) {
    return 1;
  }

  const haystack = memorySearchText(memory);
  const tokens = query.toLowerCase().split(/\s+/).filter(Boolean);
  let score = 0;

  for (const token of tokens) {
    if (haystack.includes(token)) {
      score += 1;
    }
  }

  if (haystack.includes(query.toLowerCase())) {
    score += 2;
  }

  return score;
}

function filterByProject(memories, projectKey) {
  if (!projectKey) {
    return memories;
  }
  return memories.filter((memory) => memory.projectKey === projectKey);
}

export async function listRecentMemories(input = {}, options = {}) {
  const projectKey = input.projectKey || (input.cwd ? getProjectContext(input.cwd).projectKey : null);
  const limit = input.limit || MAX_CONTEXT_MEMORIES;
  const memories = filterByMemoryTypes(filterByProject(await loadMemoryFiles(options), projectKey), input);
  return memories.slice(0, limit);
}

export async function searchMemories(input = {}, options = {}) {
  const limit = input.limit || MAX_SEARCH_RESULTS;
  const projectKey = input.projectKey || (input.cwd ? getProjectContext(input.cwd).projectKey : null);
  const query = input.query?.trim() || "";

  const matches = filterByMemoryTypes(filterByProject(await loadMemoryFiles(options), projectKey), input)
    .map((memory) => ({
      memory,
      score: scoreMemory(memory, query),
    }))
    .filter((entry) => entry.score > 0)
    .sort((left, right) => right.score - left.score || right.memory.updatedAt.localeCompare(left.memory.updatedAt))
    .slice(0, limit)
    .map((entry) => entry.memory);

  return matches;
}

export async function getMemoryById(id, options = {}) {
  if (!id) {
    return null;
  }

  const paths = await ensureStore(options);
  return readJsonOrNull(memoryFile(paths, id));
}

export async function storeMemory(input = {}, options = {}) {
  const project = getProjectContext(input.cwd || process.cwd());
  const content = normalizePrompt(input.content || input.summary || "");
  const memoryType = normalizeMemoryType(input.memoryType, "explicit");
  const agentId = input.agentId || getAgentId(options);

  if (!content || content === "[non-text prompt]") {
    return null;
  }

  const memory = {
    id: input.id || `memory-${Date.now()}-${hashId(content).slice(0, 8)}`,
    agentId,
    projectKey: input.projectKey || project.projectKey,
    projectLabel: input.projectLabel || project.projectLabel,
    workspaceLabel: input.workspaceLabel || project.workspaceLabel,
    cwd: input.cwd || project.cwd,
    gitRoot: project.gitRoot,
    startedAt: input.startedAt || nowIso(),
    updatedAt: input.updatedAt || nowIso(),
    promptCount: input.promptCount || 0,
    toolEventCount: input.toolEventCount || 0,
    memoryType,
    title: truncate(input.title || content, 80),
    request: truncate(input.request || content, 200),
    latestPrompt: truncate(input.latestPrompt || content, 200),
    summary: truncate(input.summary || content, 1000),
    keyActions: unique(input.keyActions || []),
    filesChanged: unique(input.filesChanged || []),
    filesRead: unique(input.filesRead || []),
    commands: unique(input.commands || []),
    tools: unique(input.tools || []),
    tags: unique([
      ...buildSharedTags(project, options, [`kind:${memoryType}`], agentId),
      ...(input.tags || []),
    ]),
    metadata: {
      backend: "local",
      projectKey: input.projectKey || project.projectKey,
      projectLabel: input.projectLabel || project.projectLabel,
      workspaceLabel: input.workspaceLabel || project.workspaceLabel,
      agentId,
      memoryType,
      ...(input.metadata || {}),
    },
  };

  const paths = await ensureStore(options);
  await writeJsonAtomic(memoryFile(paths, memory.id), memory);
  return memory;
}

export function formatMemory(memory) {
  const lines = [
    `ID: ${memory.id}`,
    `Project: ${memory.projectLabel} (${memory.workspaceLabel})`,
    `Agent: ${memory.agentId || "unknown"}`,
    `Type: ${getMemoryType(memory)}`,
    `Updated: ${memory.updatedAt}`,
    `Request: ${memory.request}`,
    `Summary: ${memory.summary}`,
  ];

  if (memory.tags?.length > 0) {
    lines.push(`Tags: ${memory.tags.join(", ")}`);
  }

  if (memory.keyActions?.length > 0) {
    lines.push("Key actions:");
    for (const action of memory.keyActions) {
      lines.push(`- ${action}`);
    }
  }

  if (memory.filesChanged?.length > 0) {
    lines.push(`Files changed: ${memory.filesChanged.join(", ")}`);
  }

  if (memory.commands?.length > 0) {
    lines.push(`Commands: ${memory.commands.join(" | ")}`);
  }

  return lines.join("\n");
}

function selectContextMemories(memories, limit = MAX_CONTEXT_MEMORIES) {
  const boundedLimit = Math.max(1, limit);
  const durableLimit = Math.min(MAX_CONTEXT_DURABLE_MEMORIES, boundedLimit);
  const activityLimit = Math.min(
    MAX_CONTEXT_ACTIVITY_MEMORIES,
    Math.max(boundedLimit - durableLimit, 0),
  );
  const chosenIds = new Set();

  const take = (entries, count) => {
    const selected = [];
    for (const memory of entries) {
      if (selected.length >= count || chosenIds.has(memory.id)) {
        continue;
      }
      selected.push(memory);
      chosenIds.add(memory.id);
    }
    return selected;
  };

  const durable = take(memories.filter((memory) => DURABLE_MEMORY_TYPES.has(getMemoryType(memory))), durableLimit);
  const activity = take(memories.filter((memory) => ACTIVITY_MEMORY_TYPES.has(getMemoryType(memory))), activityLimit);
  const fallback = take(memories, Math.max(boundedLimit - durable.length - activity.length, 0));

  return { durable, activity, fallback };
}

function renderContextSection(lines, title, memories) {
  if (memories.length === 0) {
    return;
  }

  lines.push(title);
  lines.push("");

  memories.forEach((memory, index) => {
    const agent = memory.agentId ? ` | by ${memory.agentId}` : "";
    lines.push(`${index + 1}. [${getMemoryType(memory)}] ${memory.updatedAt}${agent} | ${memory.title}`);
    lines.push(`   ${memory.summary}`);
    if (memory.filesChanged?.length > 0) {
      lines.push(`   Files: ${memory.filesChanged.slice(0, 4).join(", ")}`);
    }
    if (memory.commands?.length > 0) {
      lines.push(`   Commands: ${memory.commands.slice(0, 2).join(" | ")}`);
    }
    lines.push(`   Memory ID: ${memory.id}`);
    lines.push("");
  });
}

export function renderContextFromMemories(memories, input = {}) {
  if (memories.length === 0) {
    return "";
  }

  const project = input.cwd ? getProjectContext(input.cwd) : null;
  const { durable, activity, fallback } = selectContextMemories(memories, input.limit || MAX_CONTEXT_MEMORIES);
  const header = `Persistent memory for ${project?.projectLabel || memories[0].projectLabel}`;
  const lines = [header, ""];
  renderContextSection(lines, "Durable team memory:", durable);
  renderContextSection(lines, "Recent shared worklog:", activity);
  renderContextSection(lines, "Other recent memory:", fallback);

  lines.push("Use the claude-code-memory MCP tools for full details or search.");
  return lines.join("\n").trim();
}

export async function renderContextBlock(input = {}, options = {}) {
  const memories = await listRecentMemories(
    {
      cwd: input.cwd,
      projectKey: input.projectKey,
      limit: input.limit || MAX_CONTEXT_MEMORIES,
    },
    options,
  );

  return renderContextFromMemories(memories, input);
}
