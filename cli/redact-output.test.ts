import assert from "node:assert/strict";
import test from "node:test";
import { redactKubectlStdout, wantsRedactedStdout } from "./redact-output.ts";

test("wantsRedactedStdout needs env and revealing get secret", () => {
  const prev = process.env.KUBE_SHELL_REDACT_STDOUT;
  process.env.KUBE_SHELL_REDACT_STDOUT = "1";
  assert.equal(wantsRedactedStdout(["get", "secret", "token", "-o", "json"]), true);
  assert.equal(wantsRedactedStdout(["get", "pods"]), false);
  process.env.KUBE_SHELL_REDACT_STDOUT = prev;
  assert.equal(wantsRedactedStdout(["get", "secret", "token", "-o", "json"]), false);
});

test("redactKubectlStdout masks json data values", () => {
  const raw = JSON.stringify({ data: { password: "cGFzcw==" } });
  const out = redactKubectlStdout(["get", "secret", "token", "-o", "json"], raw);
  const parsed = JSON.parse(out.trim()) as { data: { password: string } };
  assert.equal(parsed.data.password, "***");
});

test("redactKubectlStdout masks yaml data values", () => {
  const raw = "apiVersion: v1\nkind: Secret\ndata:\n  password: cGFzcw==\n";
  const out = redactKubectlStdout(["get", "secret", "token", "-o", "yaml"], raw);
  assert.match(out, /password:\s*"\*\*\*"/);
  assert.doesNotMatch(out, /cGFzcw==/);
});

test("redactKubectlStdout leaves get pods unchanged", () => {
  const raw = "NAME\npod-a\n";
  assert.equal(redactKubectlStdout(["get", "pods"], raw), raw);
});

test("redactKubectlStdout masks list secret json items", () => {
  const raw = JSON.stringify({
    apiVersion: "v1",
    items: [{ kind: "Secret", data: { password: "cGFzcw==" } }],
  });
  const out = redactKubectlStdout(["get", "secrets", "-o", "json"], raw);
  const parsed = JSON.parse(out.trim()) as { items: { data: { password: string } }[] };
  assert.equal(parsed.items[0]!.data.password, "***");
});

test("wantsRedactedStdout on reordered flags", () => {
  const prev = process.env.KUBE_SHELL_REDACT_STDOUT;
  process.env.KUBE_SHELL_REDACT_STDOUT = "1";
  assert.equal(wantsRedactedStdout(["get", "-o", "yaml", "secret", "token"]), true);
  process.env.KUBE_SHELL_REDACT_STDOUT = prev;
});
