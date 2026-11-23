import assert from "node:assert/strict";
import test from "node:test";
import { spliceToken, toEngine } from "./engine.ts";

test("repl inserts kubectl and keeps a cursor at column 0", () => {
  assert.deepEqual(toEngine("get pods", 0, "repl"), {
    line: "kubectl get pods",
    cursor: 0,
    delta: 8,
    skip: false,
  });
});

test("repl at end of the line shifts the cursor by kubectl plus a space", () => {
  assert.deepEqual(toEngine("get pods", 8, "repl"), {
    line: "kubectl get pods",
    cursor: 16,
    delta: 8,
    skip: false,
  });
});

test("shell rewrites a leading kube and skips a cursor still inside that word", () => {
  assert.equal(toEngine("kube get pods", 2, "shell").skip, true);
  assert.deepEqual(toEngine("kube get pods", "kube get pods".length, "shell"), {
    line: "kubectl get pods",
    cursor: "kubectl get pods".length,
    delta: 3,
    skip: false,
  });
});

test("repl keeps view as a shell command", () => {
  assert.deepEqual(toEngine("view", 4, "repl"), {
    line: "view",
    cursor: 4,
    delta: 0,
    skip: false,
  });
});

test("spliceToken pads a value that is not a flag or a slash", () => {
  assert.deepEqual(spliceToken("kubectl get ", 12, 12, "pods", true), {
    line: "kubectl get pods ",
    cursor: 17,
  });
});
