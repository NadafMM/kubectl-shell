import fs from "node:fs";

const args = process.argv.slice(2);
const log = process.env.KUBE_FAKE_LOG;
if (log) fs.appendFileSync(log, `${JSON.stringify(args)}\n`);
if (process.env.KUBE_FAKE_KUBECONFIG_OUT) {
  fs.writeFileSync(process.env.KUBE_FAKE_KUBECONFIG_OUT, process.env.KUBECONFIG || "");
}

if ((process.env.KUBE_FAKE_FIXTURE || "down") === "down") process.exit(1);

const outputAt = args.indexOf("-o");
const output = outputAt >= 0 ? args[outputAt + 1] : "";
const getAt = args.indexOf("get");
const resource = getAt >= 0 ? (args[getAt + 1] ?? "") : "";
const combinedNamespaced =
  resource.includes("pods") &&
  resource.includes("deployments.apps") &&
  resource.includes("ingresses");

if (args[0] === "delete") {
  process.stdout.write("deleted\n");
  process.exit(0);
}

if (process.env.KUBE_FAKE_DIAG === "oom" && args[0] === "logs" && args.includes("--previous")) {
  process.stdout.write("password=secret\nboom\n");
  process.exit(0);
}

if (args[0] === "logs" && args.includes("-f")) {
  process.stdout.write("following logs...\n");
  await new Promise((resolve) => setTimeout(resolve, 30_000));
  process.exit(0);
}

if (args[0] === "config" && args[1] === "current-context") {
  process.stdout.write("minikube\n");
  process.exit(0);
}
if (args[0] === "config" && args[1] === "view") {
  process.stdout.write(
    `${JSON.stringify({
      "current-context": "minikube",
      contexts: [
        {
          name: "minikube",
          context: { cluster: "minikube", user: "minikube", namespace: "default" },
        },
      ],
    })}\n`,
  );
  process.exit(0);
}
if (args[0] === "config") {
  process.stdout.write("minikube\n");
  process.exit(0);
}

if (process.env.KUBE_FAKE_FAIL === "namespaced" && combinedNamespaced) process.exit(1);

if (output.startsWith("{range .items[*]}")) {
  if (process.env.KUBE_FAKE_FAIL === "namespaced" && resource.startsWith("pods,")) {
    const cols = [
      "Pod",
      "kept",
      "default",
      "",
      "",
      "Running",
      "",
      "",
      "",
      "",
      "",
      "",
      "",
      "",
      "",
      "",
      "",
      "",
      "",
      "app=kept",
    ];
    process.stdout.write(`${cols.join("\t")}\n`);
    process.exit(0);
  }
  if (resource.includes("deployments.apps")) {
    const cols = [
      "Deployment",
      "web",
      "default",
      "",
      "",
      "",
      "1",
      "1",
      "",
      "",
      "",
      "",
      "",
      "",
      "",
      "",
      "",
      "",
      "",
      "",
    ];
    process.stdout.write(`${cols.join("\t")}\n`);
    process.exit(0);
  }
  process.stdout.write("");
  process.exit(0);
}

if (
  process.env.KUBE_FAKE_DIAG === "oom" &&
  output === "json" &&
  getAt >= 0 &&
  /^pods?$/.test(resource)
) {
  const pod = {
    kind: "Pod",
    metadata: { name: "checkout", namespace: "default" },
    spec: {
      containers: [
        {
          name: "app",
          image: "app:1",
          resources: { limits: { memory: "64Mi" } },
        },
      ],
    },
    status: {
      phase: "Running",
      containerStatuses: [
        {
          name: "app",
          ready: false,
          restartCount: 3,
          state: { waiting: { reason: "CrashLoopBackOff" } },
          lastState: { terminated: { reason: "OOMKilled", exitCode: 137 } },
        },
      ],
    },
  };
  const next = args[getAt + 2];
  const named = next && !String(next).startsWith("-");
  process.stdout.write(`${JSON.stringify(named ? pod : { items: [pod] })}\n`);
  process.exit(0);
}

if (output === "json" && getAt >= 0 && resource === "events") {
  process.stdout.write(
    `${JSON.stringify({
      items: [
        {
          kind: "Event",
          metadata: {
            name: "ev-1",
            namespace: "default",
            creationTimestamp: new Date().toISOString(),
          },
          type: "Warning",
          reason: "Failed",
          message: "tab\\tnewline",
          involvedObject: { kind: "Pod", name: "checkout", namespace: "default" },
        },
      ],
    })}\n`,
  );
  process.exit(0);
}

if (output === "json" && getAt >= 0 && (resource === "secret" || resource === "secrets")) {
  process.stdout.write(
    `${JSON.stringify({ apiVersion: "v1", kind: "Secret", data: { password: "cGFzcw==" } })}\n`,
  );
  process.exit(0);
}

if (args[0] === "port-forward" || args[0] === "run" || args[0] === "expose") {
  process.stdout.write("ok\n");
  process.exit(0);
}

const undoNamed =
  process.env.KUBE_FAKE_UNDO === "1" &&
  output === "json" &&
  getAt >= 0 &&
  args[getAt + 2] &&
  !String(args[getAt + 2]).startsWith("-");
if (undoNamed && /deploy/.test(resource)) {
  process.stdout.write(
    `${JSON.stringify({
      apiVersion: "apps/v1",
      kind: "Deployment",
      metadata: {
        name: "checkout",
        namespace: "default",
        resourceVersion: "99",
        uid: "uid-1",
        creationTimestamp: "2020-01-01T00:00:00Z",
        generation: 4,
        managedFields: [{ manager: "kube" }],
        annotations: { "deployment.kubernetes.io/revision": "3" },
      },
      spec: { template: { spec: { containers: [{ name: "nginx", image: "nginx:1.0" }] } } },
      status: { replicas: 1 },
    })}\n`,
  );
  process.exit(0);
}
if (undoNamed && /secret/.test(resource)) {
  process.stdout.write(
    `${JSON.stringify({
      kind: "Secret",
      metadata: { name: "token", namespace: "default" },
      data: { password: "cGFzcw==" },
    })}\n`,
  );
  process.exit(0);
}
if (undoNamed && /^pod/.test(resource)) {
  process.stdout.write(
    `${JSON.stringify({
      kind: "Pod",
      metadata: { name: "worker-0", namespace: "default", resourceVersion: "5" },
      spec: {},
      status: { phase: "Running" },
    })}\n`,
  );
  process.exit(0);
}

if (output === "json") {
  process.stdout.write('{"items":[]}\n');
  process.exit(0);
}

if (args.includes("get") && (resource === "ns" || resource.startsWith("ns"))) {
  process.stdout.write("namespace/default\n");
  process.exit(0);
}

process.stdout.write("\n");
process.exit(0);
