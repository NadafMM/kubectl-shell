import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import { tryWritePrivate, writePrivate } from "./private.ts";

test("writePrivate replaces the file and does not append", () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "kube-private-"));
  const file = path.join(dir, "nested", "history");
  writePrivate(file, "one\n");
  writePrivate(file, "two\n");
  assert.equal(fs.readFileSync(file, "utf8"), "two\n");
  if (process.platform !== "win32") {
    assert.equal(fs.statSync(file).mode & 0o777, 0o600);
    assert.equal(fs.statSync(path.dirname(file)).mode & 0o777, 0o700);
  }
});

test("writePrivate leaves no temp file behind", () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "kube-private-"));
  writePrivate(path.join(dir, "live.json"), "{}");
  assert.deepEqual(fs.readdirSync(dir), ["live.json"]);
});

test("tryWritePrivate returns false instead of throwing", () => {
  const file = path.join(fs.mkdtempSync(path.join(os.tmpdir(), "kube-private-")), "f");
  fs.writeFileSync(file, "");
  assert.equal(tryWritePrivate(path.join(file, "child", "live.json"), "{}"), false);
});
