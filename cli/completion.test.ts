import assert from "node:assert/strict";
import test from "node:test";
import { completionScript } from "./completion.ts";

test("zsh completion quotes matches", () => {
  const script = completionScript("zsh");
  assert.equal(script.includes("compadd"), true);
  assert.equal(script.includes("compadd -Q"), false);
});

test("bash and fish completion scripts stay in place", () => {
  assert.equal(completionScript("bash").includes("mapfile -t COMPREPLY"), true);
  assert.equal(completionScript("fish").includes("function __kube_complete"), true);
});
