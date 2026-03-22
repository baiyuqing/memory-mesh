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

export function getProjectContext(cwd = process.cwd()) {
  const gitRoot = normalizePath(cwd, runGit(cwd, ["rev-parse", "--show-toplevel"]));
  const gitCommonDir = normalizePath(cwd, runGit(cwd, ["rev-parse", "--git-common-dir"]));

  let projectKey = basename(cwd);
  if (gitCommonDir) {
    projectKey = basename(gitCommonDir) === ".git" ? basename(dirname(gitCommonDir)) : basename(gitCommonDir);
  } else if (gitRoot) {
    projectKey = basename(gitRoot);
  }

  const workspaceLabel = basename(cwd);
  const projectLabel = gitRoot ? basename(gitRoot) : workspaceLabel;

  return {
    cwd,
    gitRoot,
    gitCommonDir,
    projectKey,
    projectLabel,
    workspaceLabel,
  };
}

