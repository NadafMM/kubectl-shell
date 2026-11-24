import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import { initialCluster } from "../src/lib/kube/fixture.ts";
import { blastRadius, formatBlast, type GetJson, type KubectlJsonResult } from "./blast.ts";
import { formatPlan, planCommand } from "./insight.ts";
import { runJson } from "./live.ts";

const none = () => ({ ok: false, json: null, error: "called" });

test("runJson parses stdout and trims the first stderr line to 120 chars", () => {
  const home = fs.mkdtempSync(path.join(os.tmpdir(), "kube-blast-"));
  const script = path.join(home, "kubectl.mjs");
  fs.writeFileSync(
    script,
    [
      "const args = process.argv.slice(2);",
      "if (args.includes('fail')) {",
      "  process.stderr.write('Error from server (Forbidden): ' + 'x'.repeat(200) + '\\nsecond line\\n');",
      "  process.exit(1);",
      "}",
      "if (args.includes('bad')) { process.stdout.write('not-json'); process.exit(0); }",
      "if (args.includes('sleep')) { Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, 5000); }",
      "process.stdout.write(JSON.stringify({ kind: 'Pod', metadata: { name: 'a' } }));",
    ].join("\n"),
  );
  const keys = ["USERPROFILE", "HOME", "KUBE_SHELL_KUBECTL", "KUBE_SHELL_NODE_SCRIPT"] as const;
  const prev = Object.fromEntries(keys.map((key) => [key, process.env[key]]));
  process.env.USERPROFILE = home;
  process.env.HOME = home;
  process.env.KUBE_SHELL_KUBECTL = process.execPath;
  process.env.KUBE_SHELL_NODE_SCRIPT = script;
  try {
    const ok = runJson(["get", "pod", "a", "-o", "json"]);
    assert.equal(ok.ok, true);
    assert.equal((ok.json as { kind: string }).kind, "Pod");
    const failed = runJson(["fail"]);
    assert.equal(failed.ok, false);
    assert.equal(failed.error.length, 120);
    assert.match(failed.error, /^Error from server \(Forbidden\):/);
    assert.doesNotMatch(failed.error, /second line/);
    const bad = runJson(["bad"]);
    assert.equal(bad.ok, false);
    assert.match(bad.error, /invalid JSON/);
    const timed = runJson(["sleep"], 200);
    assert.equal(timed.ok, false);
    assert.equal(timed.error, "timed out");
    assert.equal(fs.existsSync(path.join(home, ".kube-shell", "live.json")), false);
  } finally {
    for (const key of keys) {
      if (prev[key] === undefined) delete process.env[key];
      else process.env[key] = prev[key];
    }
  }
});

test("plan get pods skips blast and does not call kubectl", () => {
  const plan = planCommand("kubectl get pods", initialCluster());
  const before = formatPlan(plan);
  let calls = 0;
  const blast = blastRadius(plan, ["get", "pods"], () => {
    calls += 1;
    return none();
  });
  assert.equal(blast.status, "skipped");
  assert.equal(blast.reason, "not a delete");
  assert.deepEqual(blast.objects, []);
  assert.equal(blast.objectCount, 0);
  assert.deepEqual(blast.services, []);
  assert.equal(formatBlast(blast), "");
  assert.equal(calls, 0);
  assert.equal(formatPlan(plan), before);
});

test("a delete with no name or selector is not expanded", () => {
  for (const args of [["delete"], ["delete", "pod"]]) {
    let calls = 0;
    const plan = planCommand(`kubectl ${args.join(" ")}`, initialCluster());
    const blast = blastRadius(plan, args, () => {
      calls += 1;
      return none();
    });
    assert.equal(blast.status, "skipped");
    assert.equal(blast.reason, "no name or selector");
    assert.equal(formatBlast(blast), "blast: not expanded (no name or selector)");
    assert.equal(calls, 0);
  }
});

test("manifest deletes are skipped until Phase 2", () => {
  for (const args of [
    ["delete", "-f", "app.yaml"],
    ["delete", "-k", "dir"],
    ["delete", "--filename=app.yaml"],
    ["delete", "--kustomize=dir"],
  ]) {
    let calls = 0;
    const plan = planCommand(`kubectl ${args.join(" ")}`, initialCluster());
    const blast = blastRadius(plan, args, () => {
      calls += 1;
      return none();
    });
    assert.equal(blast.status, "skipped", args.join(" "));
    assert.equal(blast.reason, "manifest delete (-f/-k) is expanded in Phase 2");
    assert.equal(
      formatBlast(blast),
      "blast: skipped (manifest delete (-f/-k) is expanded in Phase 2)",
    );
    assert.equal(calls, 0);
  }
});

test("a failed target get is unknown and leaves the risk summary unchanged", () => {
  const plan = planCommand("kubectl delete pod missing", initialCluster());
  const before = formatPlan(plan);
  const blast = blastRadius(plan, ["delete", "pod", "missing"], () => ({
    ok: false,
    json: null,
    error: 'pods "missing" not found',
  }));
  assert.equal(formatPlan(plan), before);
  assert.match(before, /Deletes pod\/missing in default\. A controller may recreate a pod\./);
  assert.equal(blast.status, "unknown");
  assert.equal(blast.reason, 'kubectl get pod: pods "missing" not found');
  assert.equal(formatBlast(blast), 'blast: unknown (kubectl get pod: pods "missing" not found)');
  assert.deepEqual(blast.objects, []);
  assert.deepEqual(blast.services, []);
});

test("a configmap delete reports the object and does not look up services", () => {
  const plan = planCommand("kubectl delete configmap app", initialCluster());
  const calls: string[][] = [];
  const blast = blastRadius(plan, ["delete", "configmap", "app"], (args) => {
    calls.push(args);
    return {
      ok: true,
      json: { kind: "ConfigMap", metadata: { name: "app", namespace: "default" } },
      error: "",
    };
  });
  assert.equal(blast.status, "ok");
  assert.equal(blast.objectCount, 1);
  assert.deepEqual(blast.objects, [{ kind: "configmap", namespace: "default", name: "app" }]);
  assert.deepEqual(blast.services, []);
  assert.equal(calls.length, 1);
  const text = formatBlast(blast);
  assert.match(text, /1 configmap/);
  assert.match(text, /configmap\/app/);
});

function isResult(value: unknown): value is KubectlJsonResult {
  return (
    !!value &&
    typeof value === "object" &&
    "ok" in value &&
    "json" in value &&
    "error" in value &&
    typeof value.error === "string"
  );
}

function podJson(
  name: string,
  labels: Record<string, string>,
  ready: boolean,
  namespace = "default",
) {
  return {
    kind: "Pod",
    metadata: { name, namespace, labels },
    status: { conditions: [{ type: "Ready", status: ready ? "True" : "False" }] },
  };
}

function serviceJson(
  name: string,
  selector: Record<string, string> | undefined,
  namespace = "default",
) {
  const spec = selector === undefined ? {} : { selector };
  return { kind: "Service", metadata: { name, namespace }, spec };
}

const podGet = ["get", "pod", "checkout-abc", "-o", "json", "--request-timeout=4s"];
const serviceGet = ["get", "services", "-n", "default", "-o", "json", "--request-timeout=4s"];
const namespacePodsGet = ["get", "pods", "-n", "default", "-o", "json", "--request-timeout=4s"];

function routePods(
  target: unknown,
  services: unknown,
  namespacePods: unknown | { ok: false; json: null; error: string },
) {
  const calls: string[][] = [];
  const getJson: GetJson = (args) => {
    calls.push(args);
    if (args[1] === "services") return { ok: true, json: services, error: "" };
    if (args[1] === "pods") {
      if (isResult(namespacePods)) return namespacePods;
      return { ok: true, json: namespacePods, error: "" };
    }
    return { ok: true, json: target, error: "" };
  };
  return { calls, getJson };
}

test("deleting the only Ready pod behind a Service leaves zero ready pods", () => {
  const pod = podJson("checkout-abc", { app: "checkout" }, true);
  const { calls, getJson } = routePods(
    pod,
    { kind: "ServiceList", items: [serviceJson("checkout", { app: "checkout" })] },
    { kind: "PodList", items: [pod] },
  );
  const plan = planCommand("kubectl delete pod checkout-abc", initialCluster());
  const blast = blastRadius(plan, ["delete", "pod", "checkout-abc"], getJson);
  assert.deepEqual(calls, [podGet, serviceGet, namespacePodsGet]);
  assert.equal(blast.status, "ok");
  assert.deepEqual(blast.services, [
    { namespace: "default", name: "checkout", matchedPods: 1, readyPodsRemaining: 0 },
  ]);
  assert.match(formatBlast(blast), /service\/checkout matches 1 and would have 0 ready pods/);
});

test("deleting one of two Ready pods keeps the sibling", () => {
  const doomed = podJson("checkout-abc", { app: "checkout" }, true);
  const sibling = podJson("checkout-def", { app: "checkout" }, true);
  const { getJson } = routePods(
    doomed,
    { kind: "ServiceList", items: [serviceJson("checkout", { app: "checkout" })] },
    { kind: "PodList", items: [doomed, sibling] },
  );
  const plan = planCommand("kubectl delete pod checkout-abc", initialCluster());
  const blast = blastRadius(plan, ["delete", "pod", "checkout-abc"], getJson);
  assert.equal(blast.services[0]?.matchedPods, 1);
  assert.equal(blast.services[0]?.readyPodsRemaining, 1);
  assert.match(formatBlast(blast), /would keep 1 ready pod/);
});

test("a sibling that is not Ready does not count as remaining", () => {
  const doomed = podJson("checkout-abc", { app: "checkout" }, true);
  const sibling = podJson("checkout-def", { app: "checkout" }, false);
  const { getJson } = routePods(
    doomed,
    { kind: "ServiceList", items: [serviceJson("checkout", { app: "checkout" })] },
    { kind: "PodList", items: [doomed, sibling] },
  );
  const plan = planCommand("kubectl delete pod checkout-abc", initialCluster());
  const blast = blastRadius(plan, ["delete", "pod", "checkout-abc"], getJson);
  assert.equal(blast.services[0]?.readyPodsRemaining, 0);
  assert.match(formatBlast(blast), /would have 0 ready pods/);
});

test("a failed namespace pod list does not report zero ready pods", () => {
  const pod = podJson("checkout-abc", { app: "checkout" }, true);
  const { getJson } = routePods(
    pod,
    { kind: "ServiceList", items: [serviceJson("checkout", { app: "checkout" })] },
    { ok: false, json: null, error: "Forbidden" },
  );
  const plan = planCommand("kubectl delete pod checkout-abc", initialCluster());
  const blast = blastRadius(plan, ["delete", "pod", "checkout-abc"], getJson);
  assert.equal(blast.status, "ok");
  assert.deepEqual(blast.objects, [{ kind: "pod", namespace: "default", name: "checkout-abc" }]);
  assert.deepEqual(blast.services, []);
  assert.match(blast.serviceNote ?? "", /^services unknown \(/);
  assert.match(blast.serviceNote ?? "", /Forbidden/);
  assert.doesNotMatch(formatBlast(blast), /0 ready pods/);
});

test("empty selectors and other namespaces do not match, extra labels still do", () => {
  const pod = podJson("checkout-abc", { app: "checkout", tier: "web" }, true);
  const services = {
    kind: "ServiceList",
    items: [
      serviceJson("empty", {}),
      serviceJson("blank", undefined),
      serviceJson("checkout", { app: "checkout" }, "other"),
      serviceJson("checkout", { app: "checkout" }),
    ],
  };
  const { getJson } = routePods(pod, services, { kind: "PodList", items: [pod] });
  const plan = planCommand("kubectl delete pod checkout-abc", initialCluster());
  const blast = blastRadius(plan, ["delete", "pod", "checkout-abc"], getJson);
  assert.deepEqual(
    blast.services.map((item) => ({
      namespace: item.namespace,
      name: item.name,
      matchedPods: item.matchedPods,
    })),
    [{ namespace: "default", name: "checkout", matchedPods: 1 }],
  );
});

test("slash pod deletes resolve like positional deletes", () => {
  const pod = podJson("checkout-abc", { app: "checkout" }, true);
  const services = {
    kind: "ServiceList",
    items: [serviceJson("checkout", { app: "checkout" })],
  };
  const namespace = { kind: "PodList", items: [pod] };
  const positional = routePods(pod, services, namespace);
  const slashed = routePods(pod, services, namespace);
  const plan = planCommand("kubectl delete pod checkout-abc", initialCluster());
  const plain = blastRadius(plan, ["delete", "pod", "checkout-abc"], positional.getJson);
  const slash = blastRadius(plan, ["delete", "pod/checkout-abc"], slashed.getJson);
  assert.deepEqual(slashed.calls[0], podGet);
  assert.deepEqual(slash.objects, plain.objects);
  assert.deepEqual(slash.services, plain.services);
  assert.equal(slash.services[0]?.readyPodsRemaining, 0);
});

test("mixed slash kinds are skipped", () => {
  let calls = 0;
  const plan = planCommand("kubectl delete pod/a svc/b", initialCluster());
  const blast = blastRadius(plan, ["delete", "pod/a", "svc/b"], () => {
    calls += 1;
    return none();
  });
  assert.equal(blast.status, "skipped");
  assert.equal(blast.reason, "mixed resource kinds");
  assert.equal(formatBlast(blast), "blast: skipped (mixed resource kinds)");
  assert.equal(calls, 0);
});

test("a label selector is passed through to kubectl get", () => {
  const pod = podJson("web-0", { app: "web" }, true);
  const calls: string[][] = [];
  const plan = planCommand("kubectl delete pods -l app=web", initialCluster());
  const blast = blastRadius(plan, ["delete", "pods", "-l", "app=web"], (args) => {
    calls.push(args);
    if (args[1] === "services") {
      return {
        ok: true,
        json: { kind: "ServiceList", items: [serviceJson("web", { app: "web" })] },
        error: "",
      };
    }
    return { ok: true, json: { kind: "PodList", items: [pod] }, error: "" };
  });
  assert.deepEqual(calls[0], [
    "get",
    "pods",
    "-l",
    "app=web",
    "-o",
    "json",
    "--request-timeout=4s",
  ]);
  assert.equal(blast.objectCount, 1);
});

test("delete pods --all reuses that list and leaves zero ready pods", () => {
  const first = podJson("a", { app: "checkout" }, true);
  const second = podJson("b", { app: "checkout" }, true);
  const calls: string[][] = [];
  const plan = planCommand("kubectl delete pods --all", initialCluster());
  const blast = blastRadius(plan, ["delete", "pods", "--all"], (args) => {
    calls.push(args);
    if (args[1] === "services") {
      return {
        ok: true,
        json: { kind: "ServiceList", items: [serviceJson("checkout", { app: "checkout" })] },
        error: "",
      };
    }
    return { ok: true, json: { kind: "PodList", items: [first, second] }, error: "" };
  });
  assert.equal(calls.length, 2);
  assert.deepEqual(calls[0], ["get", "pods", "-o", "json", "--request-timeout=4s"]);
  assert.equal(blast.services[0]?.readyPodsRemaining, 0);
  assert.match(formatBlast(blast), /would have 0 ready pods/);
});

test("namespace and context flags are forwarded on every get", () => {
  const pod = podJson("checkout-abc", { app: "checkout" }, true, "payments");
  const calls: string[][] = [];
  const plan = planCommand(
    "kubectl delete pod checkout-abc -n payments --context prod",
    initialCluster(),
  );
  const blast = blastRadius(
    plan,
    ["delete", "pod", "checkout-abc", "-n", "payments", "--context", "prod"],
    (args) => {
      calls.push(args);
      if (args[1] === "services") {
        return {
          ok: true,
          json: {
            kind: "ServiceList",
            items: [serviceJson("checkout", { app: "checkout" }, "payments")],
          },
          error: "",
        };
      }
      if (args[1] === "pods")
        return { ok: true, json: { kind: "PodList", items: [pod] }, error: "" };
      return { ok: true, json: pod, error: "" };
    },
  );
  assert.ok(calls.length >= 1);
  for (const call of calls) {
    assert.equal(call[call.indexOf("-n") + 1], "payments", call.join(" "));
    assert.equal(call[call.indexOf("--context") + 1], "prod", call.join(" "));
  }
  assert.equal(blast.objects[0]?.namespace, "payments");
});

test("an empty pod list is zero pods, not unknown", () => {
  const calls: string[][] = [];
  const plan = planCommand("kubectl delete pods --all", initialCluster());
  const blast = blastRadius(plan, ["delete", "pods", "--all"], (args) => {
    calls.push(args);
    return { ok: true, json: { kind: "PodList", items: [] }, error: "" };
  });
  assert.equal(blast.status, "ok");
  assert.equal(blast.objectCount, 0);
  assert.match(formatBlast(blast), /0 pods/);
  assert.doesNotMatch(formatBlast(blast), /unknown/);
  assert.equal(calls.length, 1);
});

test("services are matched inside each namespace", () => {
  const payments = podJson("a", { app: "web" }, true, "payments");
  const other = podJson("b", { app: "web" }, true, "default");
  const calls: string[][] = [];
  const plan = planCommand("kubectl delete pod a b", initialCluster());
  const blast = blastRadius(plan, ["delete", "pod", "a", "b"], (args) => {
    calls.push(args);
    if (args[1] === "services") {
      return {
        ok: true,
        json: {
          kind: "ServiceList",
          items: [
            serviceJson("web", { app: "web" }, "payments"),
            serviceJson("web", { app: "other" }, "default"),
          ],
        },
        error: "",
      };
    }
    return { ok: true, json: { kind: "PodList", items: [payments, other] }, error: "" };
  });
  const services = calls.find((call) => call[1] === "services") ?? [];
  assert.equal(services.includes("-A"), true);
  assert.equal(services.includes("-n"), false);
  const text = formatBlast(blast);
  assert.match(text, /in namespace payments/);
  assert.match(text, /in namespace default/);
  assert.deepEqual(
    blast.services.map((item) => ({
      namespace: item.namespace,
      name: item.name,
      matchedPods: item.matchedPods,
      readyPodsRemaining: item.readyPodsRemaining,
    })),
    [{ namespace: "payments", name: "web", matchedPods: 1, readyPodsRemaining: 0 }],
  );
});

function deploymentJson(
  name: string,
  matchLabels: Record<string, string>,
  matchExpressions?: { key: string; operator: string; values: string[] }[],
) {
  return {
    kind: "Deployment",
    metadata: { name, namespace: "default" },
    spec: {
      selector: matchExpressions ? { matchLabels, matchExpressions } : { matchLabels },
    },
  };
}

test("a deployment delete expands to the pods its labels select", () => {
  const selected = Array.from({ length: 14 }, (_, index) => {
    const name = `p${String(index + 1).padStart(2, "0")}`;
    const labels: Record<string, string> =
      index < 2 ? { app: "checkout", tier: "admin" } : { app: "checkout" };
    return podJson(name, labels, true);
  });
  const extras = [1, 2, 3].map((n) => podJson(`admin-${n}`, { tier: "admin" }, true));
  const calls: string[][] = [];
  const plan = planCommand("kubectl delete deployment checkout", initialCluster());
  const blast = blastRadius(plan, ["delete", "deployment", "checkout"], (args) => {
    calls.push(args);
    if (args[1] === "services") {
      return {
        ok: true,
        json: {
          kind: "ServiceList",
          items: [
            serviceJson("checkout", { app: "checkout" }),
            serviceJson("checkout-admin", { tier: "admin" }),
          ],
        },
        error: "",
      };
    }
    if (args[1] === "pods" && args.includes("-l")) {
      return { ok: true, json: { kind: "PodList", items: selected }, error: "" };
    }
    if (args[1] === "pods") {
      return { ok: true, json: { kind: "PodList", items: [...selected, ...extras] }, error: "" };
    }
    return { ok: true, json: deploymentJson("checkout", { app: "checkout" }), error: "" };
  });
  assert.deepEqual(calls[1]?.slice(0, 4), ["get", "pods", "-l", "app=checkout"]);
  assert.equal(blast.objectCount, 14);
  assert.deepEqual(blast.controllers, [
    { kind: "deployment", namespace: "default", name: "checkout" },
  ]);
  const text = formatBlast(blast);
  assert.match(text, /deployment\/checkout/);
  assert.match(text, /service\/checkout matches all 14 and would have 0 ready pods/);
  assert.match(text, /service\/checkout-admin matches 2 and would keep 3 ready pods/);
});

test("matchExpressions skips pod and service lookups", () => {
  const calls: string[][] = [];
  const plan = planCommand("kubectl delete deployment checkout", initialCluster());
  const blast = blastRadius(plan, ["delete", "deployment", "checkout"], (args) => {
    calls.push(args);
    return {
      ok: true,
      json: deploymentJson("checkout", { app: "checkout" }, [
        { key: "app", operator: "In", values: ["checkout"] },
      ]),
      error: "",
    };
  });
  assert.equal(calls.length, 1);
  assert.equal(blast.objectCount, 1);
  assert.deepEqual(blast.services, []);
  const text = formatBlast(blast);
  assert.match(text, /1 deployment/);
  assert.match(text, /deployment\/checkout/);
});

test("deployments with different matchLabels are not expanded", () => {
  const calls: string[][] = [];
  const plan = planCommand("kubectl delete deployment a b", initialCluster());
  const blast = blastRadius(plan, ["delete", "deployment", "a", "b"], (args) => {
    calls.push(args);
    return {
      ok: true,
      json: {
        kind: "DeploymentList",
        items: [deploymentJson("a", { app: "a" }), deploymentJson("b", { app: "b" })],
      },
      error: "",
    };
  });
  assert.equal(calls.length, 1);
  assert.equal(blast.objectCount, 2);
  assert.deepEqual(blast.services, []);
  assert.equal(
    calls.some((call) => call.includes("-l")),
    false,
  );
});

function podList(count: number, namespaceAt?: (index: number) => string) {
  return {
    kind: "PodList",
    items: Array.from({ length: count }, (_, index) =>
      podJson(`p${index}`, { app: "web" }, true, namespaceAt ? namespaceAt(index) : "default"),
    ),
  };
}

test("a forbidden service list keeps the pod blast", () => {
  const pod = podJson("checkout-abc", { app: "checkout" }, true);
  const plan = planCommand("kubectl delete pod checkout-abc", initialCluster());
  const blast = blastRadius(plan, ["delete", "pod", "checkout-abc"], (args) => {
    if (args[1] === "services") return { ok: false, json: null, error: "Forbidden" };
    return { ok: true, json: pod, error: "" };
  });
  assert.equal(blast.status, "ok");
  assert.deepEqual(blast.objects, [{ kind: "pod", namespace: "default", name: "checkout-abc" }]);
  assert.match(blast.serviceNote ?? "", /Forbidden/);
  assert.doesNotMatch(formatBlast(blast), /would have/);
});

test("object names cap at 20 while the count stays complete", () => {
  const plan = planCommand("kubectl delete pods --all", initialCluster());
  const blast = blastRadius(plan, ["delete", "pods", "--all"], (args) => {
    if (args[1] === "services")
      return { ok: true, json: { kind: "ServiceList", items: [] }, error: "" };
    return { ok: true, json: podList(25), error: "" };
  });
  assert.equal(blast.objects.length, 20);
  assert.equal(blast.objectCount, 25);
  assert.match(formatBlast(blast), /and 5 more/);
});

test("more than 200 pods skips the service check", () => {
  const calls: string[][] = [];
  const plan = planCommand("kubectl delete pods --all", initialCluster());
  const blast = blastRadius(plan, ["delete", "pods", "--all"], (args) => {
    calls.push(args);
    return { ok: true, json: podList(201), error: "" };
  });
  assert.deepEqual(blast.services, []);
  assert.equal(blast.serviceNote, "service check skipped (201 pods exceeds 200)");
  assert.equal(
    calls.some((call) => call[1] === "services"),
    false,
  );
});

test("200 pods still checks services", () => {
  const calls: string[][] = [];
  const plan = planCommand("kubectl delete pods --all", initialCluster());
  blastRadius(plan, ["delete", "pods", "--all"], (args) => {
    calls.push(args);
    if (args[1] === "services")
      return { ok: true, json: { kind: "ServiceList", items: [] }, error: "" };
    return { ok: true, json: podList(200), error: "" };
  });
  assert.equal(
    calls.some((call) => call[1] === "services"),
    true,
  );
});

test("more than five namespaces skips the service check", () => {
  const calls: string[][] = [];
  const plan = planCommand("kubectl delete pod a b c d e f", initialCluster());
  const blast = blastRadius(plan, ["delete", "pod", "a", "b", "c", "d", "e", "f"], () => {
    calls.push([]);
    return {
      ok: true,
      json: podList(6, (index) => `ns${index + 1}`),
      error: "",
    };
  });
  assert.equal(blast.serviceNote, "service check skipped (6 namespaces exceeds 5)");
  assert.equal(calls.length, 1);
});

test("a get that overruns the shared budget is timed out", () => {
  let time = 0;
  const plan = planCommand("kubectl delete pod checkout-abc", initialCluster());
  const before = formatPlan(plan);
  let calls = 0;
  const blast = blastRadius(
    plan,
    ["delete", "pod", "checkout-abc"],
    () => {
      calls += 1;
      time += 3001;
      return {
        ok: true,
        json: podJson("checkout-abc", { app: "checkout" }, true),
        error: "",
      };
    },
    { now: () => time },
  );
  assert.equal(blast.status, "unknown");
  assert.equal(blast.reason, "timed out");
  assert.equal(formatBlast(blast), "blast: unknown (timed out)");
  assert.equal(calls, 1);
  assert.match(before, /Deletes pod\/checkout-abc in default\. A controller may recreate a pod\./);
  assert.equal(formatPlan(plan), before);
});

test("kubectl get does not receive the delete --all flag", () => {
  const calls: string[][] = [];
  const plan = planCommand("kubectl delete pods --all", initialCluster());
  blastRadius(plan, ["delete", "pods", "--all"], (args) => {
    calls.push(args);
    return { ok: true, json: { kind: "PodList", items: [] }, error: "" };
  });
  assert.equal(calls[0]?.includes("--all"), false);
});

test("a mixed kind list keeps each object's own kind", () => {
  const plan = planCommand("kubectl delete all -l app=web", initialCluster());
  const blast = blastRadius(plan, ["delete", "all", "-l", "app=web"], () => ({
    ok: true,
    json: {
      kind: "List",
      items: [
        podJson("web-0", { app: "web" }, true),
        {
          kind: "Service",
          metadata: { name: "web", namespace: "default" },
          spec: { selector: { app: "web" } },
        },
      ],
    },
    error: "",
  }));
  assert.deepEqual(
    blast.objects.map((item) => item.kind),
    ["pod", "service"],
  );
});

test("namespace counts survive the 20-name cap", () => {
  const plan = planCommand("kubectl delete pods -l app=web -A", initialCluster());
  const blast = blastRadius(plan, ["delete", "pods", "-l", "app=web", "-A"], (args) => {
    if (args[1] === "services")
      return { ok: true, json: { kind: "ServiceList", items: [] }, error: "" };
    const items = [
      ...Array.from({ length: 22 }, (_, index) =>
        podJson(`pay-${index}`, { app: "web" }, true, "payments"),
      ),
      ...Array.from({ length: 8 }, (_, index) =>
        podJson(`def-${index}`, { app: "web" }, true, "default"),
      ),
    ];
    return { ok: true, json: { kind: "PodList", items }, error: "" };
  });
  assert.equal(blast.objects.length, 20);
  assert.equal(blast.objectCount, 30);
  const text = formatBlast(blast);
  assert.match(text, /22 pods in namespace payments/);
  assert.match(text, /8 pods in namespace default/);
});

test("a service lookup that overruns the budget keeps the pods", () => {
  let time = 0;
  const plan = planCommand("kubectl delete pod checkout-abc", initialCluster());
  const blast = blastRadius(
    plan,
    ["delete", "pod", "checkout-abc"],
    (args) => {
      if (args[1] === "services") time += 3001;
      return { ok: true, json: podJson("checkout-abc", { app: "checkout" }, true), error: "" };
    },
    { now: () => time },
  );
  assert.equal(blast.status, "ok");
  assert.deepEqual(blast.objects, [{ kind: "pod", namespace: "default", name: "checkout-abc" }]);
  assert.equal(blast.serviceNote, "services unknown (timed out)");
  assert.doesNotMatch(formatBlast(blast), /would have 0 ready pods/);
});

test("equals-form namespace and all-namespace flags are forwarded", () => {
  const calls: string[][] = [];
  const plan = planCommand("kubectl delete pod checkout-abc -n=payments", initialCluster());
  blastRadius(plan, ["delete", "pod", "checkout-abc", "-n=payments"], (args) => {
    calls.push(args);
    return {
      ok: true,
      json: podJson("checkout-abc", { app: "checkout" }, true, "payments"),
      error: "",
    };
  });
  assert.equal(calls[0]?.[calls[0].indexOf("-n") + 1], "payments");
  const wide: string[][] = [];
  blastRadius(plan, ["delete", "pod", "checkout-abc", "--all-namespaces=true"], (args) => {
    wide.push(args);
    return {
      ok: true,
      json: podJson("checkout-abc", { app: "checkout" }, true, "payments"),
      error: "",
    };
  });
  assert.equal(wide[0]?.includes("-A"), true);
});
