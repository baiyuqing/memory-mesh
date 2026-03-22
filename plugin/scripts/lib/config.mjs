export function getBackend(options = {}) {
  if (options.backend) {
    return options.backend;
  }

  if (process.env.CLAUDE_CODE_MEMORY_BACKEND) {
    return process.env.CLAUDE_CODE_MEMORY_BACKEND;
  }

  if (process.env.MEM9_API_KEY || process.env.MEM9_TENANT_ID) {
    return "mem9";
  }

  return "local";
}

export function getAgentId(options = {}) {
  return (
    options.agentId ||
    process.env.CLAUDE_CODE_MEMORY_AGENT_ID ||
    process.env.MEM9_AGENT_ID ||
    "claude-code"
  );
}

export function getTeamId(options = {}) {
  return options.teamId || process.env.CLAUDE_CODE_MEMORY_TEAM_ID || "";
}

