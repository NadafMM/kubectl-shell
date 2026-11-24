import assert from "node:assert/strict";
import test from "node:test";
import { parsePolicy, promptLabel, ruleFor } from "./policy.ts";

test("prod-* requires the context name and guards the prompt", () => {
  const parsed = parsePolicy(
    JSON.stringify({
      contexts: { "prod-*": { mode: "guarded", confirm: "typed", deny: ["exec"] } },
    }),
  );
  assert.equal("error" in parsed, false);
  if ("error" in parsed) return;
  const rule = ruleFor(parsed, "prod-main");
  assert.equal(rule.confirm, "typed");
  assert.equal(rule.mode, "guarded");
  assert.deepEqual(rule.deny, ["exec"]);
  assert.equal(ruleFor(parsed, "staging").confirm, "default");
  assert.equal(promptLabel(true, "prod-main:default$ "), "[GUARDED] prod-main:default$ ");
});
