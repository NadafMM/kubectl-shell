import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import { leadingFlags } from "./flags.ts";
import {
  SANDBOX_SEED,
  sandboxDown,
  sandboxKubeconfig,
  sandboxUp,
  type SandboxRun,
} from "./sandbox.ts";

function fixture() {
  const home = fs.mkdtempSync(path.join(os.tmpdir(), "kube-sandbox-"));
  const userConfig = path.join(home, ".kube", "config");
  fs.mkdirSync(path.dirname(userConfig), { recursive: true });
  fs.writeFileSync(userConfig, "original-kubeconfig");
  const calls: { cmd: string; args: string[]; env: NodeJS.ProcessEnv; input?: string }[] = [];
  const run: SandboxRun = (command) => {
    calls.push(command);
    if (command.args[0] === "create") {
      fs.mkdirSync(path.dirname(sandboxKubeconfig(home)), { recursive: true });
      fs.writeFileSync(sandboxKubeconfig(home), "kind-config");
    }
    return { code: 0, stdout: "", stderr: "" };
  };
  return { home, userConfig, calls, run };
}

test("sandbox up creates kind with its own kubeconfig and seeds demo plus a broken pod", () => {
  const { home, userConfig, calls, run } = fixture();
  const previous = process.env.KUBECONFIG;
  process.env.KUBECONFIG = userConfig;
  try {
    const result = sandboxUp({ home, run, has: () => true });
    assert.equal(result.ok, true);
    assert.match(result.message, /kube-shell-sandbox is up/);
    assert.match(result.message, /sandbox[\\/]kubeconfig/);
  } finally {
    if (previous === undefined) delete process.env.KUBECONFIG;
    else process.env.KUBECONFIG = previous;
  }
  assert.equal(fs.readFileSync(userConfig, "utf8"), "original-kubeconfig");
  const create = calls[0];
  assert.ok(create);
  assert.match(create.cmd, /kind/);
  assert.deepEqual(create.args, [
    "create",
    "cluster",
    "--name",
    "kube-shell-sandbox",
    "--kubeconfig",
    sandboxKubeconfig(home),
  ]);
  assert.equal(create.env.KUBECONFIG, sandboxKubeconfig(home));
  assert.notEqual(create.env.KUBECONFIG, userConfig);
  const apply = calls[1];
  assert.ok(apply);
  assert.deepEqual(apply.args.slice(-3), ["apply", "-f", "-"]);
  assert.equal(apply.env.KUBECONFIG, sandboxKubeconfig(home));
  assert.equal(apply.input, SANDBOX_SEED);
  assert.match(SANDBOX_SEED, /kind: Deployment/);
  assert.match(SANDBOX_SEED, /name: demo/);
  assert.match(SANDBOX_SEED, /replicas: 2/);
  assert.match(SANDBOX_SEED, /kind: Service/);
  assert.match(SANDBOX_SEED, /kind: Pod/);
  assert.match(SANDBOX_SEED, /name: broken/);
  assert.match(SANDBOX_SEED, /image: kube-shell\/missing:none/);
  assert.equal(fs.readFileSync(sandboxKubeconfig(home), "utf8"), "kind-config");
});

test("sandbox up does nothing when kind or Docker is missing", () => {
  const { home, userConfig, calls, run } = fixture();
  const result = sandboxUp({ home, run, has: (name) => name === "kind" });
  assert.equal(result.ok, false);
  assert.equal(result.message, "kube sandbox needs kind and Docker on PATH");
  assert.equal(calls.length, 0);
  assert.equal(fs.existsSync(sandboxKubeconfig(home)), false);
  assert.equal(fs.readFileSync(userConfig, "utf8"), "original-kubeconfig");
});

test("a failed kind create does not apply the demo workload", () => {
  const { home, userConfig, calls, run } = fixture();
  const result = sandboxUp({
    home,
    has: () => true,
    run: (command) => {
      calls.push(command);
      return { code: 1, stdout: "", stderr: "kind exploded" };
    },
  });
  assert.equal(result.ok, false);
  assert.match(result.message, /kind exploded/);
  assert.equal(calls.length, 1);
  assert.equal(fs.readFileSync(userConfig, "utf8"), "original-kubeconfig");
  void run;
});

test("sandbox down deletes the cluster and only the sandbox kubeconfig", () => {
  const { home, userConfig, calls, run } = fixture();
  const config = sandboxKubeconfig(home);
  fs.mkdirSync(path.dirname(config), { recursive: true });
  fs.writeFileSync(config, "kind-config");
  const audit = path.join(home, ".kube-shell", "audit.jsonl");
  fs.writeFileSync(audit, "keep\n");
  const result = sandboxDown({ home, run, has: () => true });
  assert.equal(result.ok, true);
  assert.match(result.message, /kube-shell-sandbox is down/);
  assert.deepEqual(calls[0]?.args, ["delete", "cluster", "--name", "kube-shell-sandbox"]);
  assert.equal(calls[0]?.env.KUBECONFIG, config);
  assert.equal(fs.existsSync(config), false);
  assert.equal(fs.readFileSync(audit, "utf8"), "keep\n");
  assert.equal(fs.readFileSync(userConfig, "utf8"), "original-kubeconfig");
});

test("sandbox down keeps the kubeconfig when kind delete fails", () => {
  const { home, userConfig, run } = fixture();
  const config = sandboxKubeconfig(home);
  fs.mkdirSync(path.dirname(config), { recursive: true });
  fs.writeFileSync(config, "kind-config");
  const result = sandboxDown({
    home,
    has: () => true,
    run: () => ({ code: 1, stdout: "", stderr: "docker daemon is down" }),
  });
  assert.equal(result.ok, false);
  assert.match(result.message, /docker daemon is down/);
  assert.equal(fs.readFileSync(config, "utf8"), "kind-config");
  assert.equal(fs.readFileSync(userConfig, "utf8"), "original-kubeconfig");
  void run;
});

test("sandbox down removes a kubeconfig when the cluster is already gone", () => {
  const { home, run } = fixture();
  const config = sandboxKubeconfig(home);
  fs.mkdirSync(path.dirname(config), { recursive: true });
  fs.writeFileSync(config, "kind-config");
  const result = sandboxDown({
    home,
    has: () => true,
    run: () => ({ code: 1, stdout: "", stderr: "ERROR: failed to find cluster" }),
  });
  assert.equal(result.ok, true);
  assert.equal(fs.existsSync(config), false);
  void run;
});

test("--sandbox is accepted and does not swallow later flags", () => {
  const parsed = leadingFlags(["--sandbox", "--yes", "get", "pods"]);
  assert.equal(parsed.error, undefined);
  assert.equal(parsed.sandbox, true);
  assert.equal(parsed.yes, true);
  assert.deepEqual(parsed.rest, ["get", "pods"]);
});
