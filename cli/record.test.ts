import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import { leadingFlags } from "./flags.ts";
import { noteRan, redactCommand, sessionPath, startRecord, stopRecord } from "./record.ts";

test("a runbook keeps an executed get and drops a refused delete", () => {
  const home = fs.mkdtempSync(path.join(os.tmpdir(), "kube-record-"));
  const file = path.join(home, "session.md");
  startRecord(file, {
    context: "minikube",
    namespace: "default",
    started: "2026-11-25T00:00:00.000Z",
  });
  try {
    noteRan({
      decision: "refused-confirm",
      verb: "delete",
      timestamp: "2026-11-25T00:00:01.000Z",
      context: "minikube",
      namespace: "default",
      command: "kubectl delete pod worker-0",
      risk: "high",
      exitCode: null,
    });
    noteRan({
      decision: "executed",
      verb: "got",
      timestamp: "2026-11-25T00:00:02.000Z",
      context: "minikube",
      namespace: "default",
      command: "kubectl got pods",
      risk: "read",
      exitCode: 1,
    });
    noteRan({
      decision: "executed",
      verb: "get",
      timestamp: "2026-11-25T00:00:03.000Z",
      context: "minikube",
      namespace: "default",
      command: redactCommand(["get", "pods", "--from-literal", "password=secret"]),
      risk: "read",
      exitCode: 0,
    });
    const text = fs.readFileSync(file, "utf8");
    assert.match(text, /context: minikube/);
    assert.match(text, /namespace: default/);
    assert.match(text, /kubectl get pods --from-literal password=\*\*\*/);
    assert.equal(text.includes("delete"), false);
    assert.equal(text.includes("got pods"), false);
    assert.equal(text.includes("password=secret"), false);
    if (process.platform !== "win32") assert.equal(fs.statSync(file).mode & 0o777, 0o600);
  } finally {
    stopRecord();
  }
});

test("a mutation runbook keeps the plan summary and blast", () => {
  const home = fs.mkdtempSync(path.join(os.tmpdir(), "kube-record-blast-"));
  const file = path.join(home, "session.md");
  startRecord(file, {
    context: "minikube",
    namespace: "default",
    started: "2026-11-25T00:00:00.000Z",
  });
  try {
    noteRan({
      decision: "executed",
      verb: "delete",
      timestamp: "2026-11-25T00:00:04.000Z",
      context: "minikube",
      namespace: "default",
      command: "kubectl delete pod worker-0",
      risk: "high",
      exitCode: 0,
      summary: "Deletes pod/worker-0 in default.",
      blast: "blast: checkout would have 0 ready pods",
    });
    const text = fs.readFileSync(file, "utf8");
    assert.match(text, /exit: 0/);
    assert.match(text, /Deletes pod\/worker-0/);
    assert.match(text, /0 ready pods/);
  } finally {
    stopRecord();
  }
});

test("the default session path is under kube-shell and names the context", () => {
  const file = sessionPath("/home/dev", "prod/main", new Date("2026-11-25T00:00:00.000Z"));
  assert.match(file, /[\\/]\.kube-shell[\\/]sessions[\\/]/);
  assert.match(file, /prod-main\.md$/);
  assert.equal(file.includes(":"), false);
});

test("--record keeps its path", () => {
  const parsed = leadingFlags(["--record", "out.md", "get", "pods"]);
  assert.equal(parsed.error, undefined);
  assert.equal(parsed.record, "out.md");
  assert.deepEqual(parsed.rest, ["get", "pods"]);
});
