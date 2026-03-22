import { basename, dirname, isAbsolute, resolve } from "node:path";
import { spawnSync } from "node:child_process";

function runGit(cwd, args) {
  const result = spawnSync("git", args, {
    cwd,
    encoding: "utf8",
  });

  if (result.status !== 0) {
    return null;
  }

  const value = result.stdout.trim();
  return value || null;
}

function normalizePath(cwd, value) {
  if (!value) {
    return null;
  }
  return isAbsolute(value) ? value : resolve(cwd, value);
}

export function parseRemoteRepository(remoteUrl) {
  const value = typeof remoteUrl === "string" ? remoteUrl.trim().replace(/\/+$/, "") : "";
  if (!value) {
    return null;
  }

  const match = value.match(/[:/]([^/:/]+)\/([^/]+?)(?:\.git)?$/);
  if (!match) {
    return null;
  }

  const [, owner, repo] = match;
  return {
    owner,
    repo,
    slug: `${owner}/${repo}`,
  };
}

export function getProjectContext(cwd = process.cwd()) {
  const gitRoot = normalizePath(cwd, runGit(cwd, ["rev-parse", "--show-toplevel"]));
  const gitCommonDir = normalizePath(cwd, runGit(cwd, ["rev-parse", "--git-common-dir"]));
  const remoteOriginUrl = runGit(cwd, ["config", "--get", "remote.origin.url"]);
  const remoteRepository = parseRemoteRepository(remoteOriginUrl);

  let projectKey = basename(cwd);
  if (remoteRepository?.repo) {
    projectKey = remoteRepository.repo;
  } else if (gitCommonDir) {
    projectKey = basename(gitCommonDir) === ".git" ? basename(dirname(gitCommonDir)) : basename(gitCommonDir);
  } else if (gitRoot) {
    projectKey = basename(gitRoot);
  }

  const workspaceLabel = basename(cwd);
  const projectLabel = remoteRepository?.repo || (gitRoot ? basename(gitRoot) : workspaceLabel);

  return {
    cwd,
    gitRoot,
    gitCommonDir,
    remoteOriginUrl,
    remoteRepository,
    projectKey,
    projectLabel,
    workspaceLabel,
  };
}
