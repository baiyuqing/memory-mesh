import test from "node:test";
import assert from "node:assert/strict";
import { runTeamMemoryValidation } from "../examples/team-memory-validation.mjs";

test("team memory validation example passes and renders the expected context", async () => {
  const result = await runTeamMemoryValidation();

  assert.match(result.decisions, /Type=decision/);
  assert.match(result.handoffs, /Type=handoff/);
  assert.match(result.context, /Durable memory:/);
  assert.match(result.context, /Recent activity:/);
});
