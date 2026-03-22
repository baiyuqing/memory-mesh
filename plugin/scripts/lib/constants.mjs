import { homedir } from "node:os";
import { join } from "node:path";

export const PLUGIN_NAME = "claude-code-memory";
export const PLUGIN_VERSION = "0.1.0";
export const DEFAULT_DATA_HOME = join(homedir(), ".claude-code-memory");
export const HOOK_OK = {
  continue: true,
  suppressOutput: true,
};
export const MAX_PROMPT_LENGTH = 2000;
export const MAX_VALUE_PREVIEW = 4000;
export const MAX_CONTEXT_MEMORIES = 5;
export const MAX_SEARCH_RESULTS = 10;
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

