import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import { consumePlan, mintPlan } from "./plans.ts";

test("a plan id works once and rejects a different command", () => {
  const home = fs.mkdtempSync(path.join(os.tmpdir(), "kube-plan-"));
  const prev = process.env.USERPROFILE;
  const prevHome = process.env.HOME;
  process.env.USERPROFILE = home;
  process.env.HOME = home;
  try {
    const argv = ["delete", "pod", "worker-0", "-n", "default", "--context", "minikube"];
    const id = mintPlan({ context: "minikube", namespace: "default", argv });
    const wrong = consumePlan(id, {
      context: "minikube",
      namespace: "default",
      argv: ["delete", "pod", "other"],
    });
    assert.equal(wrong.ok, false);
    const ok = consumePlan(id, { context: "minikube", namespace: "default", argv });
    assert.equal(ok.ok, true);
    const replay = consumePlan(id, { context: "minikube", namespace: "default", argv });
    assert.equal(replay.ok, false);
  } finally {
    if (prev === undefined) delete process.env.USERPROFILE;
    else process.env.USERPROFILE = prev;
    if (prevHome === undefined) delete process.env.HOME;
    else process.env.HOME = prevHome;
  }
});
