import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import { appendAudit, redactArgv, tailAudit, type AuditEntry } from "./audit.ts";

function entry(argv: string[]): AuditEntry {
  return {
    id: "abc",
    timestamp: "2026-11-24T00:00:00.000Z",
    actor: "human-cli",
    context: "minikube",
    namespace: "default",
    argv,
    risk: "high",
    decision: "executed",
    approver: "flag-yes",
    exitCode: 0,
    durationMs: 1,
  };
}

test("audit redacts literal passwords and rotates to one backup", () => {
  const home = fs.mkdtempSync(path.join(os.tmpdir(), "kube-audit-"));
  const prev = process.env.USERPROFILE;
  const prevHome = process.env.HOME;
  process.env.USERPROFILE = home;
  process.env.HOME = home;
  try {
    const argv = redactArgv(["create", "secret", "generic", "t", "--from-literal=password=secret"]);
    assert.deepEqual(argv, ["create", "secret", "generic", "t", "--from-literal=password=***"]);
    assert.equal(argv.includes("--from-literal=password=secret"), false);
    appendAudit(entry(argv), 80);
    appendAudit(entry(["get", "pods"]), 80);
    const file = path.join(home, ".kube-shell", "audit.jsonl");
    const backup = `${file}.1`;
    assert.equal(fs.existsSync(backup), true);
    assert.equal(fs.existsSync(`${backup}.1`), false);
    const last = tailAudit(1)[0] ?? "";
    assert.match(last, /get/);
    assert.doesNotMatch(fs.readFileSync(file, "utf8"), /password=secret/);
  } finally {
    if (prev === undefined) delete process.env.USERPROFILE;
    else process.env.USERPROFILE = prev;
    if (prevHome === undefined) delete process.env.HOME;
    else process.env.HOME = prevHome;
  }
});
