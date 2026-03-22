import test from "node:test";
import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { recordPrompt, summarizeSession } from "../plugin/scripts/lib/store.mjs";

async function withTempStore(run) {
  const dataHome = await mkdtemp(join(tmpdir(), "memory-mesh-server-"));
  try {
    await run(dataHome);
  } finally {
    await rm(dataHome, { recursive: true, force: true });
  }
}

function parseFramedMessage(output) {
  const separator = "\r\n\r\n";
  const headerEnd = output.indexOf(separator);
  assert.notEqual(headerEnd, -1, "missing MCP header separator");

  const headers = output
    .slice(0, headerEnd)
    .split("\r\n")
    .reduce((acc, line) => {
      const [key, ...rest] = line.split(":");
      acc[key.toLowerCase()] = rest.join(":").trim();
      return acc;
    }, {});

  const body = output.slice(headerEnd + separator.length, headerEnd + separator.length + Number(headers["content-length"]));
  return JSON.parse(body);
}

test("mcp-server responds before stdin close exits the process", async () => {
  await withTempStore(async (dataHome) => {
    const cwd = process.cwd();
    await recordPrompt({ sessionId: "server-memory", cwd, prompt: "Persist MCP responses" }, { dataHome });
    await summarizeSession({ sessionId: "server-memory", cwd }, { dataHome });

    const child = spawn(process.execPath, ["plugin/scripts/mcp-server.mjs"], {
      cwd,
      env: {
        ...process.env,
        MEMORY_MESH_HOME: dataHome,
      },
      stdio: ["pipe", "pipe", "pipe"],
    });

    const request = JSON.stringify({
      jsonrpc: "2.0",
      id: 7,
      method: "tools/call",
      params: {
        name: "get_memory",
        arguments: {
          id: "server-memory",
        },
      },
    });

    let stdout = "";
    let stderr = "";
    child.stdout.setEncoding("utf8");
    child.stderr.setEncoding("utf8");
    child.stdout.on("data", (chunk) => {
      stdout += chunk;
    });
    child.stderr.on("data", (chunk) => {
      stderr += chunk;
    });

    child.stdin.end(`Content-Length: ${Buffer.byteLength(request)}\r\n\r\n${request}`);

    const exitCode = await new Promise((resolve, reject) => {
      child.on("error", reject);
      child.on("close", resolve);
    });

    assert.equal(exitCode, 0, stderr);
    const response = parseFramedMessage(stdout);
    assert.equal(response.id, 7);
    assert.match(response.result.content[0].text, /Persist MCP responses/);
  });
});
