#!/usr/bin/env node

import { PLUGIN_NAME, PLUGIN_VERSION } from "./lib/constants.mjs";
import { callTool, getToolDefinitions } from "./lib/mcp-tools.mjs";

let buffer = Buffer.alloc(0);
let pendingMessages = 0;
let stdinEnded = false;

function sendMessage(message) {
  const payload = Buffer.from(JSON.stringify(message), "utf8");
  process.stdout.write(`Content-Length: ${payload.length}\r\n\r\n`);
  process.stdout.write(payload);
}

function sendResponse(id, result) {
  sendMessage({
    jsonrpc: "2.0",
    id,
    result,
  });
}

function sendError(id, code, message) {
  sendMessage({
    jsonrpc: "2.0",
    id,
    error: {
      code,
      message,
    },
  });
}

async function handleMessage(message) {
  if (!message || typeof message !== "object") {
    return;
  }

  const { id, method, params } = message;

  if (method === "notifications/initialized") {
    return;
  }

  if (method === "ping") {
    sendResponse(id, {});
    return;
  }

  if (method === "initialize") {
    sendResponse(id, {
      protocolVersion: params?.protocolVersion || "2024-11-05",
      capabilities: {
        tools: {},
      },
      serverInfo: {
        name: PLUGIN_NAME,
        version: PLUGIN_VERSION,
      },
    });
    return;
  }

  if (method === "tools/list") {
    sendResponse(id, {
      tools: getToolDefinitions(),
    });
    return;
  }

  if (method === "tools/call") {
    try {
      const result = await callTool(params?.name, params?.arguments || {});
      sendResponse(id, result);
    } catch (error) {
      sendError(id, -32000, error instanceof Error ? error.message : String(error));
    }
    return;
  }

  if (id !== undefined) {
    sendError(id, -32601, `Method not found: ${method}`);
  }
}

function maybeExit() {
  if (stdinEnded && pendingMessages === 0) {
    process.exit(0);
  }
}

function processBuffer() {
  while (true) {
    const headerEnd = buffer.indexOf("\r\n\r\n");
    if (headerEnd === -1) {
      return;
    }

    const headerText = buffer.slice(0, headerEnd).toString("utf8");
    const headers = Object.fromEntries(
      headerText
        .split("\r\n")
        .map((line) => line.split(":"))
        .filter((parts) => parts.length >= 2)
        .map(([key, ...value]) => [key.trim().toLowerCase(), value.join(":").trim()]),
    );

    const length = Number(headers["content-length"]);
    if (!Number.isFinite(length)) {
      buffer = Buffer.alloc(0);
      return;
    }

    const bodyStart = headerEnd + 4;
    const bodyEnd = bodyStart + length;
    if (buffer.length < bodyEnd) {
      return;
    }

    const body = buffer.slice(bodyStart, bodyEnd).toString("utf8");
    buffer = buffer.slice(bodyEnd);

    let message;
    try {
      message = JSON.parse(body);
    } catch (error) {
      continue;
    }

    pendingMessages += 1;
    Promise.resolve(handleMessage(message))
      .finally(() => {
        pendingMessages -= 1;
        maybeExit();
      });
  }
}

process.stdin.on("data", (chunk) => {
  buffer = Buffer.concat([buffer, chunk]);
  processBuffer();
});

process.stdin.on("end", () => {
  stdinEnded = true;
  maybeExit();
});
