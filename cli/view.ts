import type { Cluster, KObject } from "../src/lib/kube/model";
import { namespaceOf } from "../src/lib/kube/model";
import { appendAudit, auditEnabled, newAuditId, redactArgv } from "./audit";
import { type Plan, planCommand } from "./insight";
import { captureLive, kubectlArgs } from "./live";
import { openPromptInput, readSyncRetry } from "./tty";
import { reduce, viewState, type ViewState } from "./view-state";

export type Board = "pod" | "deployment" | "service" | "node" | "namespace";

export type Row = {
  name: string;
  ns: string;
  info: string;
  status: string;
  age: string;
  kind: string;
};

const BOARDS: { id: Board; label: string }[] = [
  { id: "pod", label: "pods" },
  { id: "deployment", label: "deploy" },
  { id: "service", label: "svc" },
  { id: "node", label: "nodes" },
  { id: "namespace", label: "ns" },
];

export function deleteEngineLine(row: Pick<Row, "kind" | "name" | "ns">): string {
  const parts = ["kubectl", "delete", row.kind, row.name];
  if (row.ns) parts.push("-n", row.ns);
  return parts.join(" ");
}

function runDelete(plan: Plan, yes: boolean): string {
  const started = Date.now();
  const argv = kubectlArgs(plan.command);
  const result = captureLive(argv);
  if (auditEnabled(null)) {
    appendAudit({
      id: newAuditId(),
      timestamp: new Date().toISOString(),
      actor: process.stdin.isTTY ? "human-tty" : "human-cli",
      context: plan.context,
      namespace: plan.namespace,
      argv: redactArgv(argv),
      risk: plan.risk,
      decision: "executed",
      approver: yes ? "flag-yes" : "prompt-y",
      exitCode: result.code,
      durationMs: Date.now() - started,
    });
  }
  return (result.stderr || result.stdout || "deleted").trim().split("\n")[0] ?? "deleted";
}

export function boardFrom(word: string | undefined): Board {
  const token = (word ?? "pods").toLowerCase();
  if (["po", "pod", "pods"].includes(token)) return "pod";
  if (["deploy", "deployment", "deployments"].includes(token)) return "deployment";
  if (["svc", "service", "services"].includes(token)) return "service";
  if (["no", "node", "nodes"].includes(token)) return "node";
  if (["ns", "namespace", "namespaces"].includes(token)) return "namespace";
  return "pod";
}

function toneOf(status: string): "bad" | "warn" | "ok" | "plain" {
  if (/CrashLoop|Error|ImagePull|BackOff|Failed|Evicted|NotReady/i.test(status)) return "bad";
  if (/Pending|Unknown/i.test(status)) return "warn";
  if (/Running|Active|Ready|Bound|Succeeded/i.test(status)) return "ok";
  return "plain";
}

function objectRow(item: KObject): Row {
  const info = item.kind === "service" ? item.svcType || "" : "";
  return {
    name: item.name,
    ns: item.ns,
    info,
    status: item.status || item.kind,
    age: item.age,
    kind: item.kind,
  };
}

function byProblem(a: Row, b: Row): number {
  const rank = (row: Row) =>
    toneOf(row.status) === "bad" ? 0 : toneOf(row.status) === "warn" ? 1 : 2;
  return rank(a) - rank(b) || a.ns.localeCompare(b.ns) || a.name.localeCompare(b.name);
}

export function rowsOf(
  cluster: Cluster,
  board: Board,
  ns: string,
  all: boolean,
  filter: string,
): Row[] {
  const needle = filter.trim().toLowerCase();
  const keep = (row: Row) =>
    !needle || `${row.name} ${row.ns} ${row.status}`.toLowerCase().includes(needle);
  if (board === "node") {
    return cluster.nodes
      .map((node) => ({
        name: node.name,
        ns: "",
        info: node.roles || node.version,
        status: node.status,
        age: node.age,
        kind: "node",
      }))
      .filter(keep)
      .sort(byProblem);
  }
  if (board === "namespace") {
    return cluster.namespaces
      .map((item) => ({
        name: item.name,
        ns: "",
        info: "",
        status: item.status,
        age: item.age,
        kind: "namespace",
      }))
      .filter(keep)
      .sort(byProblem);
  }
  return cluster.objects
    .filter((item) => item.kind === board && (all || item.ns === ns))
    .map(objectRow)
    .filter(keep)
    .sort(byProblem);
}

export function tableText(cluster: Cluster, board: Board, all: boolean): string {
  const ns = namespaceOf(cluster);
  const rows = rowsOf(cluster, board, ns, all, "");
  const lines = [all ? "NAMESPACE  NAME  STATUS" : "NAME  STATUS"];
  for (const row of rows)
    lines.push(`${all && row.ns ? `${row.ns}  ` : ""}${row.name}  ${row.status}`);
  return lines.join("\n");
}

function fit(text: string, width: number): string {
  const clean = text.replace(/\s+/g, " ");
  if (clean.length >= width) return clean.slice(0, Math.max(0, width));
  return clean.padEnd(width);
}

function paint(text: string, tone: "bad" | "warn" | "ok" | "plain" | "selected" | "head"): string {
  if (tone === "selected") return `\x1b[7m${text}\x1b[0m`;
  if (tone === "head") return `\x1b[1m${text}\x1b[0m`;
  if (tone === "bad") return `\x1b[31m${text}\x1b[0m`;
  if (tone === "warn") return `\x1b[33m${text}\x1b[0m`;
  if (tone === "ok") return `\x1b[32m${text}\x1b[0m`;
  return text;
}

function readKeys(fd: number): string[] {
  const buf = Buffer.alloc(64);
  const n = readSyncRetry(fd, buf);
  if (n <= 0) return ["q"];
  const raw = buf.subarray(0, n).toString("utf8");
  const keys: string[] = [];
  for (let i = 0; i < raw.length; i += 1) {
    if (raw[i] !== "\x1b") {
      keys.push(raw[i] ?? "");
      continue;
    }
    const rest = raw.slice(i);
    if (rest.startsWith("\x1b[A")) {
      keys.push("up");
      i += 2;
    } else if (rest.startsWith("\x1b[B")) {
      keys.push("down");
      i += 2;
    } else if (rest.startsWith("\x1b[5~")) {
      keys.push("pgup");
      i += 3;
    } else if (rest.startsWith("\x1b[6~")) {
      keys.push("pgdn");
      i += 3;
    } else if (rest.startsWith("\x1b[3~")) {
      keys.push("del");
      i += 3;
    } else {
      keys.push("esc");
    }
  }
  return keys.filter(Boolean);
}

function lookupText(verb: "describe" | "logs", row: Row): string {
  const args = verb === "logs" ? ["logs", row.name, "--tail=80"] : ["describe", row.kind, row.name];
  if (row.ns) args.push("-n", row.ns);
  const result = captureLive(args);
  return (result.stdout || result.stderr || "(no output)").replace(/\s+$/, "");
}

export type ViewOptions = {
  board: Board;
  all: boolean;
  yes: boolean;
  typedConfirm?: boolean;
  reload: (all: boolean, ns: string) => Cluster | null;
};

/** Full-screen resource table. Without a terminal, prints the table and returns. */
export function openView(clusterIn: Cluster, options: ViewOptions): Cluster {
  if (!process.stdin.isTTY || !process.stdout.isTTY) {
    process.stdout.write(`${tableText(clusterIn, options.board, options.all)}\n`);
    return clusterIn;
  }

  let cluster = clusterIn;
  let text: string[] = [];
  let pendingDelete: Plan | null = null;
  let state: ViewState = viewState({
    board: options.board,
    all: options.all,
    ns: namespaceOf(cluster),
    yes: options.yes,
    nsCursor: Math.max(
      0,
      cluster.namespaces.findIndex((item) => item.name === namespaceOf(cluster)),
    ),
  });

  process.stdin.setRawMode(true);
  const input = openPromptInput();
  process.stdout.write("\x1b[?1049h\x1b[?25l");

  const rows = () => rowsOf(cluster, state.board, state.ns, state.all, state.filter);
  const reload = () => {
    const next = options.reload(state.all, state.ns);
    if (next) {
      cluster = next;
      state = { ...state, message: "refreshed" };
      return;
    }
    state = { ...state, message: "refresh failed" };
  };
  const draw = () => {
    const { board, all, ns, filter, filtering, mode, command, commanding } = state;
    let cursor = state.cursor;
    let nsCursor = state.nsCursor;
    const message = state.message;
    const scroll = state.scroll;
    const width = Math.max(40, process.stdout.columns || 80);
    const height = Math.max(12, process.stdout.rows || 24);
    const list = rows();
    if (cursor >= list.length) cursor = Math.max(0, list.length - 1);
    const lines: string[] = [];
    const label = BOARDS.find((item) => item.id === board)?.label ?? board;
    lines.push(
      paint(
        fit(
          ` kube  ${cluster.current}  ${all ? "all namespaces" : ns}  ${label}  ${list.length}`,
          width,
        ),
        "head",
      ),
    );
    if (mode === "help") {
      for (const line of [
        "j/k or arrows   move",
        "enter           describe",
        "l               logs, pods only",
        "d               delete, then y",
        "/               filter by name",
        "1-5             pods, deploy, svc, nodes, ns",
        "a               all namespaces",
        "n               pick a namespace",
        "r               refresh",
        ":pods :deploy   same as the number keys",
        "q               back to the shell",
      ]) {
        lines.push(fit(line, width));
      }
    } else if (mode === "text") {
      for (const line of text.slice(scroll, scroll + height - 3)) lines.push(fit(line, width));
    } else if (mode === "ns") {
      const names = cluster.namespaces.map((item) => item.name);
      if (nsCursor >= names.length) nsCursor = Math.max(0, names.length - 1);
      const room = height - 3;
      const start = Math.max(0, Math.min(nsCursor - 2, Math.max(0, names.length - room)));
      for (let i = start; i < Math.min(names.length, start + room); i += 1) {
        const name = names[i] ?? "";
        lines.push(
          paint(
            fit(`  ${name}${name === ns ? "  current" : ""}`, width),
            i === nsCursor ? "selected" : "plain",
          ),
        );
      }
    } else {
      const showNs = all && board !== "node" && board !== "namespace";
      const nameW = Math.max(16, Math.min(36, width - (showNs ? 48 : 32)));
      lines.push(
        paint(
          fit(
            `${showNs ? fit("NAMESPACE", 16) : ""}${fit("NAME", nameW)}  ${fit("INFO", 16)}  STATUS`,
            width,
          ),
          "head",
        ),
      );
      const room = height - 4;
      const start = Math.max(
        0,
        Math.min(cursor - Math.floor(room / 2), Math.max(0, list.length - room)),
      );
      for (let i = start; i < Math.min(list.length, start + room); i += 1) {
        const row = list[i]!;
        const body = `${showNs ? fit(row.ns, 16) : ""}${fit(row.name, nameW)}  ${fit(row.info, 16)}  ${row.status}  ${row.age}`;
        lines.push(paint(fit(body, width), i === cursor ? "selected" : toneOf(row.status)));
      }
    }
    while (lines.length < height - 1) lines.push(fit("", width));
    const hint = commanding
      ? `:${command}`
      : filtering
        ? `/${filter}`
        : mode === "confirm"
          ? message
          : message ||
            "1 pods  2 deploy  3 svc  4 nodes  5 ns    enter describe   l logs   / filter   d delete   ? help   q quit";
    lines.push(paint(fit(hint, width), "head"));
    state.cursor = cursor;
    state.nsCursor = nsCursor;
    process.stdout.write(`\x1b[H\x1b[2J${lines.slice(0, height).join("\n")}`);
  };

  try {
    draw();
    for (;;) {
      for (const key of readKeys(input.fd)) {
        const list = rows();
        const step = reduce(state, key, {
          rows: list,
          namespaces: cluster.namespaces.map((item) => item.name),
        });
        state = step.state;
        const effect = step.effect;
        if (effect.type === "quit") return cluster;
        if (effect.type === "reload") reload();
        if (effect.type === "describe" || effect.type === "logs") {
          const row = list[effect.index];
          if (row) {
            text = lookupText(effect.type === "logs" ? "logs" : "describe", row).split("\n");
            state = { ...state, textLength: text.length, scroll: 0, mode: "text" };
          }
        }
        if (effect.type === "arm-delete" || effect.type === "delete") {
          if (options.typedConfirm) {
            state = {
              ...state,
              message: "typed context: run kube delete from the shell",
            };
            continue;
          }
        }
        if (effect.type === "arm-delete") {
          const row = list[effect.index];
          if (row) {
            pendingDelete = planCommand(deleteEngineLine(row), cluster);
            state = {
              ...state,
              message: `${pendingDelete.risk} risk: ${pendingDelete.summary}  [y/N]`,
            };
          }
        }
        if (effect.type === "delete") {
          const row = list[effect.index];
          const planned =
            pendingDelete ?? (row ? planCommand(deleteEngineLine(row), cluster) : null);
          pendingDelete = null;
          if (planned) {
            state = { ...state, message: runDelete(planned, options.yes) };
            reload();
          }
        }
      }
      draw();
    }
  } finally {
    input.close();
    process.stdout.write("\x1b[?25h\x1b[?1049l");
    if (process.stdin.isTTY) process.stdin.setRawMode(false);
  }
}
