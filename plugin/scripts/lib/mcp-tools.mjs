import { formatMemory, getMemoryById, listRecentMemories, searchMemories, storeMemory } from "./store.mjs";

export function getToolDefinitions() {
  return [
    {
      name: "search_memories",
      description: "Search stored Claude Code memory summaries for the current project or all projects.",
      inputSchema: {
        type: "object",
        properties: {
          query: {
            type: "string",
            description: "Search text to match against stored memory.",
          },
          projectKey: {
            type: "string",
            description: "Optional project key override. Defaults to the current project when cwd is passed.",
          },
          cwd: {
            type: "string",
            description: "Optional working directory used to infer the project key.",
          },
          limit: {
            type: "number",
            minimum: 1,
            maximum: 20,
          },
        },
        required: ["query"],
      },
    },
    {
      name: "list_recent_memories",
      description: "List recent memory summaries for the current project or all projects.",
      inputSchema: {
        type: "object",
        properties: {
          projectKey: {
            type: "string",
          },
          cwd: {
            type: "string",
          },
          limit: {
            type: "number",
            minimum: 1,
            maximum: 20,
          },
        },
      },
    },
    {
      name: "get_memory",
      description: "Fetch the full text for a stored memory by ID.",
      inputSchema: {
        type: "object",
        properties: {
          id: {
            type: "string",
            description: "The memory ID returned by search_memories or list_recent_memories.",
          },
        },
        required: ["id"],
      },
    },
    {
      name: "store_memory",
      description: "Persist a team-shared memory item for the current project. Useful for Codex or explicit durable notes.",
      inputSchema: {
        type: "object",
        properties: {
          content: {
            type: "string",
            description: "The memory content to persist.",
          },
          cwd: {
            type: "string",
            description: "Optional working directory used to infer the project key.",
          },
          title: {
            type: "string",
          },
          tags: {
            type: "array",
            items: {
              type: "string",
            },
          },
          memoryType: {
            type: "string",
            description: "Optional memory type label such as explicit, decision, constraint, or worklog.",
          },
          metadata: {
            type: "object",
          },
        },
        required: ["content"],
      },
    },
  ];
}

function textResult(text) {
  return {
    content: [
      {
        type: "text",
        text,
      },
    ],
  };
}

function renderCompactList(memories) {
  if (memories.length === 0) {
    return "No memories found.";
  }

  return memories
    .map((memory, index) => {
      const files = memory.filesChanged.slice(0, 3).join(", ");
      const commands = memory.commands.slice(0, 2).join(" | ");
      const details = [
        `ID=${memory.id}`,
        `Updated=${memory.updatedAt}`,
        files ? `Files=${files}` : "",
        commands ? `Commands=${commands}` : "",
      ]
        .filter(Boolean)
        .join(" | ");

      return `${index + 1}. ${memory.title}\n   ${memory.summary}\n   ${details}`;
    })
    .join("\n\n");
}

export async function callTool(name, args = {}) {
  if (name === "search_memories") {
    const memories = await searchMemories(args);
    return textResult(renderCompactList(memories));
  }

  if (name === "list_recent_memories") {
    const memories = await listRecentMemories(args);
    return textResult(renderCompactList(memories));
  }

  if (name === "get_memory") {
    const memory = await getMemoryById(args.id);
    return textResult(memory ? formatMemory(memory) : `Memory not found: ${args.id}`);
  }

  if (name === "store_memory") {
    const result = await storeMemory(args);
    return textResult(result ? `Stored shared memory.\n\n${result.summary || result.content || ""}` : "Nothing was stored.");
  }

  throw new Error(`Unknown tool: ${name}`);
}
