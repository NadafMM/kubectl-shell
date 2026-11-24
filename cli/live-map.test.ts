import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import { complete } from "../src/lib/kube/complete.ts";
import { initialCluster } from "../src/lib/kube/fixture.ts";
import {
  invalidateLive,
  liveCluster,
  mapListItem,
  mapNode,
  parseProjected,
  probeLive,
  PROJECTED_JSONPATH,
} from "./live.ts";

const fake = path.join(process.cwd(), "scripts", "fake-kubectl.mjs");

test("mapListItem keeps a network policy name", () => {
  const mapped = mapListItem({
    kind: "NetworkPolicy",
    metadata: { name: "allow-web", namespace: "app" },
  });
  assert.equal(mapped?.kind, "networkpolicy");
  assert.equal(mapped?.name, "allow-web");
});

test("mapListItem keeps labels and age", () => {
  const mapped = mapListItem({
    kind: "Pod",
    metadata: {
      name: "checkout",
      namespace: "app",
      creationTimestamp: new Date(Date.now() - 3_600_000).toISOString(),
      labels: { app: "checkout" },
    },
    status: { phase: "Running" },
  });
  assert.equal(mapped?.labels.app, "checkout");
  assert.notEqual(mapped?.age, "");
});

test("mapNode reads Ready false", () => {
  const node = mapNode({
    kind: "Node",
    metadata: { name: "worker-a" },
    spec: { unschedulable: true },
    status: { conditions: [{ type: "Ready", status: "False" }] },
  });
  assert.equal(node?.status, "NotReady");
  assert.equal(node?.schedulable, false);
});

test("mapNode without a Ready condition is Unknown", () => {
  const node = mapNode({ kind: "Node", metadata: { name: "worker-b" } });
  assert.equal(node?.status, "Unknown");
  assert.equal(node?.schedulable, true);
});

test("a mapped label completes -l app=checkout", () => {
  const mapped = mapListItem({
    kind: "Pod",
    metadata: { name: "checkout", namespace: "default", labels: { app: "checkout" } },
    status: { phase: "Running" },
  });
  const cluster = initialCluster();
  cluster.objects.push(mapped!);
  const line = "kubectl get pods -l ";
  const found = complete(line, line.length, cluster);
  assert.ok(found.candidates.some((item) => item.value === "app=checkout"));
});

function withFake(run: (log: string) => void) {
  const home = fs.mkdtempSync(path.join(os.tmpdir(), "kube-live-"));
  const log = path.join(home, "kubectl.log");
  const previous = {
    USERPROFILE: process.env.USERPROFILE,
    HOME: process.env.HOME,
    KUBE_SHELL_KUBECTL: process.env.KUBE_SHELL_KUBECTL,
    KUBE_SHELL_NODE_SCRIPT: process.env.KUBE_SHELL_NODE_SCRIPT,
    KUBE_FAKE_LOG: process.env.KUBE_FAKE_LOG,
    KUBE_FAKE_FIXTURE: process.env.KUBE_FAKE_FIXTURE,
    KUBE_FAKE_FAIL: process.env.KUBE_FAKE_FAIL,
  };
  process.env.USERPROFILE = home;
  process.env.HOME = home;
  process.env.KUBE_SHELL_KUBECTL = process.execPath;
  process.env.KUBE_SHELL_NODE_SCRIPT = fake;
  process.env.KUBE_FAKE_LOG = log;
  process.env.KUBE_FAKE_FIXTURE = "up";
  delete process.env.KUBE_FAKE_FAIL;
  invalidateLive();
  try {
    run(log);
  } finally {
    invalidateLive();
    for (const [key, value] of Object.entries(previous)) {
      if (value === undefined) delete process.env[key];
      else process.env[key] = value;
    }
  }
}

function logLines(log: string): string[] {
  return fs.existsSync(log) ? fs.readFileSync(log, "utf8").split("\n").filter(Boolean) : [];
}

test("a cold liveCluster makes at most five kubectl calls", () => {
  withFake((log) => {
    const cluster = liveCluster("kubectl");
    assert.equal(cluster?.current, "minikube");
    assert.ok(logLines(log).length <= 5);
    assert.ok(logLines(log).length >= 1);
  });
});

test("a second liveCluster inside the TTL does not call kubectl again", () => {
  withFake((log) => {
    liveCluster("kubectl");
    const first = logLines(log).length;
    liveCluster("kubectl");
    assert.equal(logLines(log).length, first);
  });
});

test("a failed combined get keeps objects from the groups that worked", () => {
  withFake((log) => {
    process.env.KUBE_FAKE_FAIL = "namespaced";
    invalidateLive();
    const cluster = liveCluster("kubectl");
    assert.ok(cluster?.objects.some((item) => item.name === "kept"));
    assert.ok(logLines(log).length > 4);
  });
});

test("PROJECTED_JSONPATH does not contain .data or stringData", () => {
  assert.equal(PROJECTED_JSONPATH.includes(".data"), false);
  assert.equal(PROJECTED_JSONPATH.includes("stringData"), false);
});

test("parseProjected parses 20 columns and drops shifted or oversized rows", () => {
  const line = "Pod\tkept\tdefault\t\t\tRunning\t\t\t\t\t\t\t\t\t\t\t\t\tweb\tapp=kept";
  const parsed = parseProjected(line);
  assert.equal(parsed.length, 1);
  assert.equal(parsed[0]?.metadata?.name, "kept");
  assert.equal(parsed[0]?.metadata?.labels?.app, "kept");
  assert.equal(parsed[0]?.spec?.containers?.[0]?.name, "web");
  assert.equal("data" in (parsed[0] ?? {}), false);

  const twentyFields = `${line}\textra`;
  assert.equal(parseProjected(twentyFields).length, 0);

  const tabInMessage =
    "Pod\tkept\tdefault\t\t\tRunning\t\t\t\t\t\t\tmsg\twith\ta\ttab\t\t\t\t\tapp=kept";
  assert.equal(parseProjected(tabInMessage).length, 0);
});

test("probeLive does not trust live.json cache file", () => {
  withFake(() => {
    process.env.KUBE_FAKE_FIXTURE = "down";
    invalidateLive();
    const home = process.env.USERPROFILE || process.env.HOME || "";
    const cache = path.join(home, ".kube-shell", "live.json");
    fs.mkdirSync(path.dirname(cache), { recursive: true });
    fs.writeFileSync(
      cache,
      JSON.stringify({
        at: Date.now(),
        key: "minikube|minikube|default",
        cluster: { current: "minikube" },
      }),
    );
    assert.equal(probeLive(), false);
  });
});
