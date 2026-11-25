/**
 * Smoke tests against a real cluster (kind/minikube) and kubectl on PATH.
 * Requires KUBECONFIG or ~/.kube/config with a reachable API (CI sets this via kind-action).
 */
import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const kube = path.join(root, "bin", "kube");

/** Resolve kubeconfig before HOME is redirected to an empty temp dir for kube-shell cache. */
function hostKubeconfig() {
  if (process.env.KUBECONFIG) {
    return process.env.KUBECONFIG;
  }
  const home = process.env.HOME || os.homedir();
  const cfg = path.join(home, ".kube", "config");
  return fs.existsSync(cfg) ? cfg : undefined;
}

function spawnKube(args, extraEnv = {}) {
  if (!fs.existsSync(kube)) {
    throw new Error(`missing ${kube}; run go build -o bin/kube ./cmd/kube`);
  }
  const home = fs.mkdtempSync(path.join(os.tmpdir(), "kube-e2e-"));
  const kubeconfig = hostKubeconfig();
  return spawnSync(kube, args, {
    cwd: root,
    encoding: "utf8",
    env: {
      ...process.env,
      HOME: home,
      USERPROFILE: home,
      ...(kubeconfig ? { KUBECONFIG: kubeconfig } : {}),
      ...extraEnv,
    },
  });
}

function kubectl(args) {
  return spawnSync("kubectl", args, { encoding: "utf8" });
}

const cluster = kubectl(["cluster-info"]);
if (cluster.status !== 0) {
  console.error("kubectl cluster-info failed; set KUBECONFIG or start kind/minikube.");
  console.error(cluster.stderr || cluster.stdout);
  process.exit(1);
}

const getNs = spawnKube(["get", "ns"]);
assert.equal(getNs.status, 0, getNs.stderr || getNs.stdout);

const plan = spawnKube(["plan", "get", "pods", "--json"]);
assert.equal(plan.status, 0, plan.stderr || plan.stdout);
const parsed = JSON.parse(plan.stdout.trim());
assert.equal(parsed.risk, "read");

const help = spawnKube(["--help"]);
assert.equal(help.status, 0);
assert.match(help.stdout, /plan/i);

console.log("e2e-kind: ok");
