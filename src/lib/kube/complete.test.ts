import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import { complete, manifestCandidates } from "./complete.ts";
import { initialCluster } from "./fixture.ts";
import { resolveResource } from "./model.ts";

const NO_POD_NAMES = [
  "networkpolicies",
  "poddisruptionbudgets",
  "resourcequotas",
  "limitranges",
  "priorityclasses",
  "customresourcedefinitions",
  "endpoints",
  "storageclasses",
];

test("an empty line offers view as a shell command", () => {
  const line = "";
  const found = complete(line, line.length, initialCluster());
  assert.ok(found.candidates.some((item) => item.value === "view"));
});

test("view completes deploy", () => {
  const line = "view ";
  const found = complete(line, line.length, initialCluster());
  assert.ok(found.candidates.some((item) => item.value === "deploy"));
});

test("get pods completes worker-0", () => {
  const line = "kubectl get pods ";
  const found = complete(line, line.length, initialCluster());
  assert.ok(found.candidates.some((item) => item.value === "worker-0"));
});

test("image completion does not invent an image name", () => {
  const line = "kubectl run web --image=";
  const found = complete(line, line.length, initialCluster());
  assert.equal(
    found.candidates.some((item) => item.value.includes("nginx")),
    false,
  );
});

test("-l completion includes app=worker from the fixture", () => {
  const line = "kubectl get pods -l ";
  const found = complete(line, line.length, initialCluster());
  assert.ok(found.candidates.some((item) => item.value === "app=worker"));
});

test("pods stay pods and the other catalog kinds are not borrowed", () => {
  assert.equal(resolveResource("pods")?.kind, "pod");
  assert.equal(resolveResource("endpoints")?.kind, "endpoints");
  assert.equal(resolveResource("poddisruptionbudgets")?.kind, "poddisruptionbudget");
  assert.equal(resolveResource("networkpolicies")?.kind, "networkpolicy");
  assert.equal(resolveResource("resourcequotas")?.kind, "resourcequota");
  assert.equal(resolveResource("limitranges")?.kind, "limitrange");
  assert.equal(resolveResource("storageclasses")?.kind, "storageclass");
  assert.equal(resolveResource("priorityclasses")?.kind, "priorityclass");
  assert.equal(resolveResource("customresourcedefinitions")?.kind, "customresourcedefinition");
});

for (const name of NO_POD_NAMES) {
  test(`${name} does not complete pod names`, () => {
    const line = `kubectl get ${name} `;
    const found = complete(line, line.length, initialCluster());
    assert.equal(
      found.candidates.some((item) => item.value === "worker-0"),
      false,
    );
    assert.ok(found.candidates.every((item) => item.kind === "flag"));
  });
}

test("networkpolicies completes its own name", () => {
  const cluster = initialCluster();
  cluster.objects.push({
    kind: "networkpolicy",
    name: "allow-web",
    ns: "default",
    age: "1d",
    labels: {},
  });
  const line = "kubectl get networkpolicies ";
  const found = complete(line, line.length, cluster);
  assert.ok(found.candidates.some((item) => item.value === "allow-web"));
  assert.equal(
    found.candidates.some((item) => item.value === "worker-0"),
    false,
  );
});

test("storageclasses complete from outside the namespace", () => {
  const cluster = initialCluster();
  cluster.objects.push({ kind: "storageclass", name: "standard", ns: "", age: "1d", labels: {} });
  const line = "kubectl get storageclasses ";
  const found = complete(line, line.length, cluster);
  assert.ok(found.candidates.some((item) => item.value === "standard"));
});

test("apply -f completes a manifest in the given directory", () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "kube-manifests-"));
  fs.writeFileSync(path.join(root, "deploy.yaml"), "kind: Pod\n");
  fs.mkdirSync(path.join(root, "k8s"));
  fs.writeFileSync(path.join(root, "k8s", "web.yaml"), "kind: Service\n");
  const top = manifestCandidates("", root);
  assert.ok(top.some((item) => item.value === "deploy.yaml"));
  assert.ok(top.some((item) => item.value === "k8s/"));
  const nested = manifestCandidates("k8s/w", root);
  assert.deepEqual(
    nested.map((item) => item.value),
    ["k8s/web.yaml"],
  );
  const line = "kubectl apply -f ";
  const previous = process.cwd();
  process.chdir(root);
  try {
    const found = complete(line, line.length, initialCluster());
    assert.ok(found.candidates.some((item) => item.value === "deploy.yaml"));
    assert.equal(
      found.candidates.some((item) => item.value === "./deploy.yaml"),
      false,
    );
  } finally {
    process.chdir(previous);
  }
});

test("a symlinked manifest is not a completion candidate", () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "kube-symlink-"));
  const target = path.join(root, "real.yaml");
  fs.writeFileSync(target, "kind: Pod\n");
  try {
    fs.symlinkSync(target, path.join(root, "deploy.yaml"));
  } catch (err) {
    const code = (err as NodeJS.ErrnoException).code;
    if (code === "EPERM") return;
    throw err;
  }
  const found = manifestCandidates("", root);
  assert.equal(
    found.some((item) => item.value === "deploy.yaml"),
    false,
  );
});
