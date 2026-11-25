import assert from "node:assert/strict";
import test from "node:test";
import type { Cluster } from "../src/lib/kube/model";
import { planCommand } from "./insight";
import { deleteEngineLine, paintText, rowsOf, tableText, type Row } from "./view";
import { reduce, viewState } from "./view-state";

const cluster: Cluster = {
  current: "minikube",
  contexts: [{ name: "minikube", cluster: "minikube", user: "minikube", namespace: "default" }],
  namespaces: [
    { name: "default", status: "Active", age: "1d" },
    { name: "app", status: "Active", age: "1d" },
  ],
  nodes: [
    {
      name: "node-1",
      status: "Ready",
      roles: "worker",
      age: "1d",
      version: "v1.31.1",
      ip: "",
      schedulable: true,
    },
  ],
  pvs: [],
  events: [],
  objects: [
    { kind: "pod", name: "api", ns: "default", age: "1d", labels: {}, status: "Running" },
    {
      kind: "pod",
      name: "worker-0",
      ns: "default",
      age: "6h",
      labels: {},
      status: "CrashLoopBackOff",
    },
    { kind: "pod", name: "checkout", ns: "app", age: "2h", labels: {}, status: "Pending" },
    { kind: "deployment", name: "web", ns: "default", age: "2d", labels: {}, status: "1/1" },
  ],
};

test("problem pods sort above running ones", () => {
  const rows = rowsOf(cluster, "pod", "default", false, "");
  assert.deepEqual(
    rows.map((row) => row.name),
    ["worker-0", "api"],
  );
});

test("all namespaces includes the other namespace", () => {
  assert.match(tableText(cluster, "pod", true), /app {2}checkout {2}Pending/);
});

test("deploy board lists web", () => {
  assert.match(tableText(cluster, "deployment", false), /web/);
});

test("delete plan for a pod is high risk", () => {
  const plan = planCommand(
    deleteEngineLine({ kind: "pod", name: "worker-0", ns: "default" }),
    cluster,
  );
  assert.equal(plan.risk, "high");
  assert.match(plan.command, /delete pod worker-0/);
});

test("d then y asks to delete and y confirms", () => {
  const rows: Row[] = rowsOf(cluster, "pod", "default", false, "");
  const armed = reduce(viewState({ board: "pod", all: false, ns: "default", yes: false }), "d", {
    rows,
    namespaces: ["default"],
  });
  assert.equal(armed.effect.type, "arm-delete");
  assert.equal(armed.state.mode, "confirm");
  const confirmed = reduce(armed.state, "y", { rows, namespaces: ["default"] });
  assert.equal(confirmed.effect.type, "delete");
  const cancelled = reduce(armed.state, "n", { rows, namespaces: ["default"] });
  assert.equal(cancelled.effect.type, "none");
  assert.equal(cancelled.state.message, "not deleted");
});

test("q leaves a submode and then quits", () => {
  const rows: Row[] = [];
  const help = reduce(
    viewState({ board: "pod", all: false, ns: "default", yes: false, mode: "help" }),
    "q",
    { rows, namespaces: [] },
  );
  assert.equal(help.state.mode, "list");
  assert.equal(help.effect.type, "none");
  const quit = reduce(help.state, "q", { rows, namespaces: [] });
  assert.equal(quit.effect.type, "quit");
});

test("a filter keeps only the matching name", () => {
  assert.deepEqual(
    rowsOf(cluster, "pod", "default", true, "check").map((row) => row.name),
    ["checkout"],
  );
});

test("NO_COLOR strips view color codes", () => {
  const previous = process.env.NO_COLOR;
  process.env.NO_COLOR = "1";
  try {
    for (const tone of ["bad", "warn", "ok", "selected", "head", "plain"] as const) {
      assert.equal(paintText("status", tone).includes("\x1b["), false);
    }
  } finally {
    if (previous === undefined) delete process.env.NO_COLOR;
    else process.env.NO_COLOR = previous;
  }
});

test("an empty NO_COLOR still paints status colors", () => {
  const previous = process.env.NO_COLOR;
  process.env.NO_COLOR = "";
  try {
    assert.equal(paintText("down", "bad"), "\x1b[31mdown\x1b[0m");
    assert.equal(paintText("ok", "ok"), "\x1b[32mok\x1b[0m");
  } finally {
    if (previous === undefined) delete process.env.NO_COLOR;
    else process.env.NO_COLOR = previous;
  }
});
