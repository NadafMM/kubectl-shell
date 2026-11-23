import assert from "node:assert/strict";
import test from "node:test";
import { completionValue, plainText } from "./terminal.ts";

test("plainText removes an OSC sequence and keeps the text", () => {
  const cleaned = plainText("ok\u001b]0;pwn\u0007");
  assert.equal(cleaned.includes("\u001b"), false);
  assert.equal(cleaned.includes("ok"), true);
  assert.equal(plainText("fine"), "fine");
});

test("completionValue drops shell metacharacters", () => {
  assert.equal(completionValue("deploy.yaml"), "deploy.yaml");
  assert.equal(completionValue("foo$(id).yaml"), null);
  assert.equal(completionValue("foo`id`.yaml"), null);
  assert.equal(completionValue("foo;id.yaml"), null);
  assert.equal(completionValue("foo|id.yaml"), null);
  assert.equal(completionValue("foo\nid.yaml"), null);
});
