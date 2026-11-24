import fs from "node:fs";

const args = process.argv.slice(2);
const log = process.env.KUBE_FAKE_LOG;
if (log) fs.appendFileSync(log, `${JSON.stringify(args)}\n`);

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
