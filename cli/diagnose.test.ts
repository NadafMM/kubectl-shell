import assert from "node:assert/strict";
import test from "node:test";
import { diagnosePod, inspectPods } from "./diagnose.ts";

function mustDiagnose(found: ReturnType<typeof diagnosePod>) {
  if (!found) throw new Error("expected a diagnosis");
  return found;
}

function checkout(status: unknown, spec?: unknown) {
  return {
    kind: "Pod",
    metadata: { name: "checkout", namespace: "default" },
    spec: spec ?? {
      containers: [
        {
          name: "app",
          image: "nginx:missing",
          resources: { limits: { memory: "64Mi" } },
        },
      ],
    },
    status,
  };
}

test("OOMKilled names the container, exit, and memory limit", () => {
  const found = diagnosePod(
    checkout({
      phase: "Running",
      containerStatuses: [
        {
          name: "app",
          ready: false,
          state: { waiting: { reason: "CrashLoopBackOff" } },
          lastState: { terminated: { reason: "OOMKilled", exitCode: 137 } },
        },
      ],
    }),
  );
  assert.equal(found?.pod, "checkout");
  assert.equal(found?.namespace, "default");
  assert.equal(found?.findings.length, 1);
  assert.equal(found?.findings[0]?.rule, "oom");
  assert.equal(
    found?.findings[0]?.cause,
    "container app was OOMKilled (exit 137); memory limit 64Mi",
  );
  assert.deepEqual(found?.findings[0]?.commands, [
    "describe pod checkout -n default",
    "logs checkout -c app --previous -n default",
  ]);
});

test("exit 137 without an OOMKilled reason is still OOM and not a crash", () => {
  const found = diagnosePod(
    checkout({
      phase: "Running",
      containerStatuses: [
        {
          name: "app",
          ready: false,
          state: { waiting: { reason: "CrashLoopBackOff" } },
          lastState: { terminated: { reason: "Error", exitCode: 137 } },
        },
      ],
    }),
  );
  assert.deepEqual(
    found?.findings.map((item) => item.rule),
    ["oom"],
  );
});

test("a missing memory limit prints none", () => {
  const found = diagnosePod(
    checkout(
      {
        phase: "Running",
        containerStatuses: [
          {
            name: "app",
            lastState: { terminated: { reason: "OOMKilled" } },
          },
        ],
      },
      { containers: [{ name: "app", image: "app:1" }] },
    ),
  );
  assert.match(found?.findings[0]?.cause ?? "", /memory limit none/);
  assert.match(found?.findings[0]?.cause ?? "", /exit 137/);
});

test("CrashLoopBackOff reports the exit code and the last 30 redacted log lines", () => {
  const lines = ["first", ...Array.from({ length: 30 }, (_, i) => `l${i + 1} password=secret`)];
  const found = diagnosePod(
    checkout({
      phase: "Running",
      containerStatuses: [
        {
          name: "app",
          ready: false,
          state: { waiting: { reason: "CrashLoopBackOff" } },
          lastState: { terminated: { reason: "Error", exitCode: 1 } },
        },
      ],
    }),
    [],
    { app: lines.join("\n") },
  );
  assert.equal(found?.findings.length, 1);
  assert.equal(found?.findings[0]?.rule, "crash");
  const cause = found?.findings[0]?.cause ?? "";
  assert.match(cause, /^container app exited 1; /);
  assert.equal(cause.includes("first"), false);
  assert.match(cause, /l30 password=\*\*\*/);
  assert.equal(cause.includes("password=secret"), false);
  assert.deepEqual(found?.findings[0]?.commands, ["logs checkout -c app --previous -n default"]);
});

test("a crash with no previous log omits the log tail", () => {
  const found = diagnosePod(
    checkout({
      phase: "Running",
      containerStatuses: [
        {
          name: "app",
          state: { waiting: { reason: "CrashLoopBackOff" } },
          lastState: { terminated: { exitCode: 2 } },
        },
      ],
    }),
  );
  assert.equal(found?.findings[0]?.cause, "container app exited 2");
});

test("ImagePullBackOff classifies a missing tag, auth, and a registry timeout", () => {
  const imagePod = (message: string) =>
    mustDiagnose(
      diagnosePod(
        checkout({
          phase: "Pending",
          containerStatuses: [
            {
              name: "app",
              ready: false,
              state: { waiting: { reason: "ImagePullBackOff", message } },
            },
          ],
        }),
        [{ reason: "Failed", message, involvedObject: { name: "checkout" } }],
      ),
    );

  assert.equal(
    imagePod("manifest unknown").findings[0]?.cause,
    "image nginx:missing failed to pull (tag not found); manifest unknown",
  );
  assert.match(
    imagePod("unauthorized: authentication required").findings[0]?.cause ?? "",
    /\(auth\)/,
  );
  assert.match(imagePod("dial tcp: i/o timeout").findings[0]?.cause ?? "", /\(registry timeout\)/);
  assert.deepEqual(imagePod("manifest unknown").findings[0]?.commands, [
    "describe pod checkout -n default",
  ]);
  assert.deepEqual(
    imagePod("manifest unknown").findings.map((item) => item.rule),
    ["image"],
  );
});

test("CreateContainerConfigError names the Secret key from the event", () => {
  const found = diagnosePod(
    checkout({
      phase: "Pending",
      containerStatuses: [
        {
          name: "app",
          state: {
            waiting: {
              reason: "CreateContainerConfigError",
              message: "couldn't find key password in Secret default/db",
            },
          },
        },
      ],
    }),
  );
  assert.equal(found?.findings[0]?.cause, "missing Secret db key password");
  assert.deepEqual(
    found?.findings.map((item) => item.rule),
    ["config"],
  );
  assert.deepEqual(found?.findings[0]?.commands, [
    "describe pod checkout -n default",
    "get secret db -n default",
  ]);
});

test("CreateContainerConfigError uses the container env when the event has no key", () => {
  const found = diagnosePod(
    checkout(
      {
        phase: "Pending",
        containerStatuses: [
          {
            name: "app",
            state: {
              waiting: { reason: "CreateContainerConfigError", message: "container create failed" },
            },
          },
        ],
      },
      {
        containers: [
          {
            name: "app",
            image: "app:1",
            env: [{ name: "TOKEN", valueFrom: { configMapKeyRef: { name: "app", key: "TOKEN" } } }],
          },
        ],
      },
    ),
  );
  assert.equal(found?.findings[0]?.cause, "missing ConfigMap app key TOKEN");
  assert.deepEqual(found?.findings[0]?.commands[1], "get configmap app -n default");
});

test("Pending reports cpu, memory, an unbound claim, a taint, and affinity", () => {
  const pending = (message: string, spec?: unknown) =>
    mustDiagnose(
      diagnosePod(
        checkout(
          {
            phase: "Pending",
            conditions: [{ type: "PodScheduled", status: "False", message }],
            containerStatuses: [
              { name: "app", ready: false, state: { waiting: { reason: "ContainerCreating" } } },
            ],
          },
          spec,
        ),
        [{ reason: "FailedScheduling", message, involvedObject: { name: "checkout" } }],
      ),
    );

  assert.equal(
    pending("0/3 nodes are available: 3 Insufficient cpu.").findings[0]?.cause,
    "Pending: insufficient cpu",
  );
  assert.equal(
    pending("0/3 nodes are available: 3 Insufficient memory.").findings[0]?.cause,
    "Pending: insufficient memory",
  );
  assert.equal(
    pending("pod has unbound immediate PersistentVolumeClaims", {
      containers: [{ name: "app", image: "app:1" }],
      volumes: [{ name: "data", persistentVolumeClaim: { claimName: "data" } }],
    }).findings[0]?.cause,
    "Pending: unbound PersistentVolumeClaim data",
  );
  assert.equal(
    pending("0/1 nodes are available: 1 node(s) had untolerated taint {dedicated: gpu}.")
      .findings[0]?.cause,
    "Pending: node taint",
  );
  assert.equal(
    pending("0/1 nodes are available: 1 node(s) didn't match Pod's node affinity/selector.")
      .findings[0]?.cause,
    "Pending: node affinity mismatch",
  );
  assert.deepEqual(pending("3 Insufficient cpu.").findings[0]?.commands, [
    "describe pod checkout -n default",
    "get events -n default",
  ]);
});

test("a failing readiness probe counts Unhealthy warnings", () => {
  const events = [1, 2, 3].map(() => ({
    type: "Warning",
    reason: "Unhealthy",
    message: "Readiness probe failed: HTTP probe failed with statuscode: 500",
    involvedObject: { name: "checkout" },
  }));
  const found = diagnosePod(
    checkout(
      {
        phase: "Running",
        containerStatuses: [{ name: "app", ready: false, state: { running: { startedAt: "t" } } }],
      },
      {
        containers: [
          {
            name: "app",
            image: "app:1",
            readinessProbe: { httpGet: { path: "/health", port: 8080 } },
          },
        ],
      },
    ),
    events,
  );
  assert.equal(found?.findings[0]?.rule, "probe");
  assert.equal(
    found?.findings[0]?.cause,
    "readiness probe http://:8080/health failed (3 Unhealthy warnings)",
  );
  assert.deepEqual(found?.findings[0]?.commands, ["describe pod checkout -n default"]);
});

test("one Unhealthy warning is singular and a liveness probe is named", () => {
  const found = diagnosePod(
    checkout(
      {
        phase: "Running",
        containerStatuses: [{ name: "app", ready: false, state: { running: {} } }],
      },
      {
        containers: [
          {
            name: "app",
            image: "app:1",
            livenessProbe: { tcpSocket: { port: 3306 } },
          },
        ],
      },
    ),
    [
      {
        reason: "Unhealthy",
        message: "Liveness probe failed",
        involvedObject: { name: "checkout" },
      },
    ],
  );
  assert.equal(found?.findings[0]?.cause, "liveness probe tcp:3306 failed (1 Unhealthy warning)");
});

test("a general question does not fetch cluster state", () => {
  const found = inspectPods({
    question: "how do I port-forward",
    namespace: "default",
    fetchJson: () => {
      throw new Error("fetchJson");
    },
    fetchText: () => {
      throw new Error("fetchText");
    },
  });
  assert.equal(found, null);
});

test("a namespace scan ignores the 11th pod", () => {
  const calls: string[][] = [];
  const healthy = (name: string) => ({
    metadata: { name, namespace: "default" },
    spec: { containers: [{ name: "app", image: "app:1" }] },
    status: {
      phase: "Running",
      containerStatuses: [{ name: "app", ready: true, state: { running: {} } }],
    },
  });
  const oom = {
    metadata: { name: "eleventh", namespace: "default" },
    spec: {
      containers: [{ name: "app", image: "app:1", resources: { limits: { memory: "64Mi" } } }],
    },
    status: {
      phase: "Running",
      containerStatuses: [
        {
          name: "app",
          ready: false,
          lastState: { terminated: { reason: "OOMKilled", exitCode: 137 } },
        },
      ],
    },
  };
  const found = inspectPods({
    question: "why are pods crashing",
    namespace: "default",
    fetchJson: (args) => {
      calls.push(args);
      return {
        ok: true,
        json: { items: [...Array.from({ length: 10 }, (_, i) => healthy(`p${i}`)), oom] },
      };
    },
    fetchText: () => {
      throw new Error("fetchText");
    },
  });
  assert.equal(found, null);
  assert.equal(calls.length, 1);
  assert.deepEqual(calls[0]?.slice(0, 3), ["get", "pods", "-n"]);
});

test("a named crash fetches that pod, its events, and previous logs", () => {
  const calls: string[][] = [];
  const found = inspectPods({
    question: "why is pod/checkout crashing",
    namespace: "payments",
    fetchJson: (args) => {
      calls.push(args);
      if (args[1] === "pod") {
        return {
          ok: true,
          json: checkout({
            phase: "Running",
            containerStatuses: [
              {
                name: "app",
                ready: false,
                state: { waiting: { reason: "CrashLoopBackOff" } },
                lastState: { terminated: { reason: "Error", exitCode: 1 } },
              },
            ],
          }),
        };
      }
      return { ok: true, json: { items: [] } };
    },
    fetchText: (args) => {
      calls.push(args);
      return "password=secret";
    },
  });
  assert.equal(found?.findings[0]?.cause, "container app exited 1; password=***");
  assert.deepEqual(calls[0], ["get", "pod", "checkout", "-n", "payments", "-o", "json"]);
  assert.deepEqual(calls[1], [
    "get",
    "events",
    "-n",
    "payments",
    "--field-selector",
    "involvedObject.name=checkout",
    "-o",
    "json",
  ]);
  assert.deepEqual(calls[2], ["logs", "checkout", "-n", "payments", "-c", "app", "--previous"]);
});

test("a healthy running pod has no findings", () => {
  const found = diagnosePod(
    checkout({
      phase: "Running",
      containerStatuses: [{ name: "app", ready: true, state: { running: {} } }],
    }),
  );
  assert.equal(found, null);
});
