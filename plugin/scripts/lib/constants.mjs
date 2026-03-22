import { homedir } from "node:os";
import { join } from "node:path";

export const PLUGIN_NAME = "memory-mesh";
export const PLUGIN_VERSION = "0.2.0";
export const DEFAULT_DATA_HOME = join(homedir(), ".memory-mesh");
export const LEGACY_DATA_HOME = join(homedir(), ".claude-code-memory");
export const HOOK_OK = {
  continue: true,
  suppressOutput: true,
};
export const MAX_PROMPT_LENGTH = 2000;
export const MAX_VALUE_PREVIEW = 4000;
export const MAX_CONTEXT_MEMORIES = 5;
export const MAX_CONTEXT_SCAN_MEMORIES = 20;
export const MAX_CONTEXT_DURABLE_MEMORIES = 3;
export const MAX_CONTEXT_ACTIVITY_MEMORIES = 2;
export const MAX_SEARCH_RESULTS = 10;
export const MAX_CONTEXT_HINT_LENGTH = 80;
export const DURABLE_MEMORY_TYPES = new Set([
  "goal",
  "decision",
  "constraint",
  "ownership",
  "runbook",
  "api-contract",
]);
export const ACTIVITY_MEMORY_TYPES = new Set([
  "session-summary",
  "worklog",
  "handoff",
]);
export const IMPORTANT_TOOLS = new Set([
  "Bash",
  "Edit",
  "MultiEdit",
  "Write",
  "NotebookEdit",
  "Grep",
  "Glob",
  "Read",
  "LS",
  "Task",
]);
export const CHANGE_TOOLS = new Set([
  "Edit",
  "MultiEdit",
  "Write",
  "NotebookEdit",
  "CreateFile",
  "DeleteFile",
  "RenameFile",
]);
export const READ_TOOLS = new Set([
  "Read",
  "Glob",
  "Grep",
  "LS",
  "ListDir",
]);
