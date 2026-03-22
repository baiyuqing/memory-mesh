import test from "node:test";
import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { getProjectContext, parseRemoteRepository } from "../plugin/scripts/lib/project.mjs";

test("parseRemoteRepository handles GitHub ssh and https remotes", () => {
  assert.deepEqual(parseRemoteRepository("https://github.com/baiyuqing/memory-mesh.git"), {
    owner: "baiyuqing",
    repo: "memory-mesh",
    slug: "baiyuqing/memory-mesh",
  });
  assert.deepEqual(parseRemoteRepository("git@github.com:baiyuqing/memory-mesh.git"), {
    owner: "baiyuqing",
    repo: "memory-mesh",
    slug: "baiyuqing/memory-mesh",
  });
});

test("getProjectContext prefers the origin repository name for the project key", () => {
  const remote = spawnSync("git", ["config", "--get", "remote.origin.url"], {
    cwd: process.cwd(),
    encoding: "utf8",
  });

  assert.equal(remote.status, 0);
  const repository = parseRemoteRepository(remote.stdout.trim());
  assert.ok(repository);

  const project = getProjectContext(process.cwd());
  assert.equal(project.projectKey, repository.repo);
  assert.equal(project.projectLabel, repository.repo);
  assert.equal(project.remoteRepository?.slug, repository.slug);
});
