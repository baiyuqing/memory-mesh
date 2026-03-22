#!/usr/bin/env node

import { HOOK_OK } from "./lib/constants.mjs";
import { ensureStore, recordPrompt, recordToolUse, renderContextBlock, summarizeSession } from "./lib/store.mjs";

async function readStdinJson() {
  if (process.stdin.isTTY) {
    return {};
  }

  const chunks = [];
  for await (const chunk of process.stdin) {
    chunks.push(chunk);
  }

  const text = Buffer.concat(chunks).toString("utf8").trim();
  return text ? JSON.parse(text) : {};
}

function normalizeInput(raw) {
  return {
    sessionId: raw.session_id ?? raw.id ?? raw.sessionId,
    cwd: raw.cwd ?? process.cwd(),
    prompt: raw.prompt,
    toolName: raw.tool_name ?? raw.toolName,
    toolInput: raw.tool_input ?? raw.toolInput,
    toolResponse: raw.tool_response ?? raw.toolResponse,
  };
}

function printJson(value) {
  process.stdout.write(`${JSON.stringify(value)}\n`);
}

async function handleSetup() {
  await ensureStore();
  printJson(HOOK_OK);
}

async function handleSessionStart(input) {
  const additionalContext = await renderContextBlock({ cwd: input.cwd });
  printJson({
    hookSpecificOutput: {
      hookEventName: "SessionStart",
      additionalContext,
    },
  });
}

async function handleSessionInit(input) {
  await recordPrompt(input);
  printJson(HOOK_OK);
}

async function handleObserve(input) {
  await recordToolUse(input);
  printJson(HOOK_OK);
}

async function handleSummarize(input) {
  await summarizeSession(input);
  printJson(HOOK_OK);
}

async function main() {
  const [, , command, subcommand] = process.argv;
  const raw = await readStdinJson();
  const input = normalizeInput(raw);

  try {
    if (command === "setup") {
      await handleSetup();
      return;
    }

    if (command === "hook" && subcommand === "session-start") {
      await handleSessionStart(input);
      return;
    }

    if (command === "hook" && subcommand === "session-init") {
      await handleSessionInit(input);
      return;
    }

    if (command === "hook" && subcommand === "observe") {
      await handleObserve(input);
      return;
    }

    if (command === "hook" && subcommand === "summarize") {
      await handleSummarize(input);
      return;
    }

    throw new Error(`Unknown command: ${command || ""} ${subcommand || ""}`.trim());
  } catch (error) {
    if (command === "hook" && subcommand === "session-start") {
      printJson({
        hookSpecificOutput: {
          hookEventName: "SessionStart",
          additionalContext: "",
        },
      });
      return;
    }

    printJson(HOOK_OK);
    process.stderr.write(`${error instanceof Error ? error.stack : String(error)}\n`);
  }
}

await main();
