import fs from "node:fs";
import path from "node:path";
import { resolveResource } from "../src/lib/kube/model";
import type { Cluster } from "../src/lib/kube/model";
import { namespaceOf } from "../src/lib/kube/model";
import { flagValue, positionalWords } from "../src/lib/kube/parse";
import { captureLive } from "./live";
import { stateDir, writePrivate } from "./private";

const SNAPSHOT_VERBS = new Set(["apply", "replace", "patch", "scale", "label", "annotate", "set"]);
const WORKLOADS = new Set(["deployment", "statefulset", "daemonset"]);
const DAY_MS = 24 * 60 * 60 * 1000;

export type UndoRecord = {
  id: string;
  at: number;
  verb: string;
  kind: string;
  name: string;
  namespace: string;
  context: string;
  revision?: string;
  snapshot?: unknown;
  refused?: string;
};

function undoDir(): string {
  return path.join(stateDir(), "undo");
}

function singular(token: string): string {
  return resolveResource(token)?.singular ?? token.toLowerCase();
}

export function namedTarget(argv: string[]): { verb: string; kind: string; name: string } | null {
  const words = positionalWords(argv).words;
  const verb = words[0] ?? "";
  if (!SNAPSHOT_VERBS.has(verb)) return null;
  const token = verb === "set" ? words[2] : words[1];
  const plainName = verb === "set" ? words[3] : words[2];
  if (!token) return null;
  if (token.includes("/")) {
    const [kind, name] = token.split("/");
    if (!kind || !name) return null;
    return { verb, kind: singular(kind), name };
  }
  if (!plainName || plainName.startsWith("-")) return null;
  return { verb, kind: singular(token), name: plainName };
}

export function cleanSnapshot(value: unknown): unknown {
  if (!value || typeof value !== "object" || Array.isArray(value)) return value;
  const copy = JSON.parse(JSON.stringify(value)) as {
    status?: unknown;
    metadata?: Record<string, unknown>;
  };
  delete copy.status;
  if (copy.metadata) {
    for (const key of [
      "resourceVersion",
      "uid",
      "creationTimestamp",
      "generation",
      "managedFields",
    ]) {
      delete copy.metadata[key];
    }
  }
  return copy;
}

function namespaceFrom(argv: string[], cluster: Cluster): string {
  return flagValue(argv, ["-n", "--namespace"]) || namespaceOf(cluster);
}

export function rememberUndo(argv: string[], cluster: Cluster, id: string, now = Date.now()): void {
  const target = namedTarget(argv);
  if (!target) return;
  const namespace = namespaceFrom(argv, cluster);
  const base: UndoRecord = {
    id,
    at: now,
    verb: target.verb,
    kind: target.kind,
    name: target.name,
    namespace,
    context: cluster.current,
  };
  if (target.kind === "secret" || target.kind === "configmap") {
    base.refused = `${target.kind} objects are not snapshotted`;
    writePrivate(path.join(undoDir(), `${id}.json`), JSON.stringify(base));
    return;
  }
  const got = captureLive(["get", target.kind, target.name, "-n", namespace, "-o", "json"]);
  if (got.code !== 0 || !got.stdout.trim()) return;
  let live: { metadata?: { name?: string; annotations?: Record<string, string> } };
  try {
    live = JSON.parse(got.stdout) as typeof live;
  } catch {
    return;
  }
  if (!live.metadata?.name) return;
  base.snapshot = cleanSnapshot(live);
  const revision = live.metadata.annotations?.["deployment.kubernetes.io/revision"];
  if (revision && WORKLOADS.has(target.kind)) base.revision = revision;
  writePrivate(path.join(undoDir(), `${id}.json`), JSON.stringify(base));
  pruneUndo(now);
}

export function listUndo(now = Date.now()): UndoRecord[] {
  const dir = undoDir();
  if (!fs.existsSync(dir)) return [];
  const records: UndoRecord[] = [];
  for (const name of fs.readdirSync(dir)) {
    if (!name.endsWith(".json")) continue;
    try {
      const record = JSON.parse(fs.readFileSync(path.join(dir, name), "utf8")) as UndoRecord;
      if (now - record.at <= DAY_MS) records.push(record);
    } catch {
      /* skip a damaged record */
    }
  }
  return records.sort((a, b) => b.at - a.at).slice(0, 20);
}

export function latestUndo(now = Date.now()): UndoRecord | null {
  return listUndo(now)[0] ?? null;
}

export function undoRefusal(record: UndoRecord): string | null {
  if (record.kind === "secret" || record.kind === "configmap") {
    return "kube undo does not restore Secrets or ConfigMaps. They are not snapshotted, because a restore without their data would destroy it.";
  }
  if (record.kind === "pod") {
    return "kube undo cannot restore a Pod. A controller recreates pods.";
  }
  return null;
}

export function restoreArgs(record: UndoRecord): string[] {
  if (record.revision && WORKLOADS.has(record.kind)) {
    return [
      "rollout",
      "undo",
      `${record.kind}/${record.name}`,
      `-n`,
      record.namespace,
      `--to-revision=${record.revision}`,
    ];
  }
  return ["apply", "-n", record.namespace, "-f", "-"];
}

function pruneUndo(now: number) {
  const records = listUndo(now);
  const keep = new Set(records.map((record) => record.id));
  for (const name of fs.readdirSync(undoDir())) {
    const id = name.replace(/\.json$/, "");
    if (!keep.has(id)) fs.rmSync(path.join(undoDir(), name), { force: true });
  }
}
