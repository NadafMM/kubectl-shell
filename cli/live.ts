import { execFileSync, spawnSync } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import type { Cluster, KObject, ObjectKind } from "../src/lib/kube/model";
import { scopeOf, tokenize, unquote } from "../src/lib/kube/parse";
import { stateDir, tryWritePrivate } from "./private";
import { redactKubectlStdout, wantsRedactedStdout } from "./redact-output";

const TTL_MS = 12_000;

function cachePath(): string {
  return path.join(stateDir(), "live.json");
}

type CacheRow = { at: number; key: string; cluster: Cluster };
type KItem = {
  kind?: string;
  metadata?: {
    name?: string;
    namespace?: string;
    creationTimestamp?: string;
    labels?: Record<string, string>;
  };
  type?: string;
  reason?: string;
  message?: string;
  involvedObject?: { kind?: string; name?: string; namespace?: string };
  spec?: {
    type?: string;
    schedule?: string;
    unschedulable?: boolean;
    capacity?: { storage?: string };
    ingressClassName?: string;
    containers?: { name?: string }[];
  };
  status?: {
    phase?: string;
    readyReplicas?: number;
    replicas?: number;
    numberReady?: number;
    succeeded?: number;
    currentReplicas?: number;
    nodeInfo?: { kubeletVersion?: string };
    conditions?: { type?: string; status?: string }[];
    containerStatuses?: { state?: { waiting?: { reason?: string } } }[];
  };
};

const KIND: Record<string, ObjectKind> = {
  Pod: "pod",
  Deployment: "deployment",
  Service: "service",
  ConfigMap: "configmap",
  Secret: "secret",
  Ingress: "ingress",
  PersistentVolumeClaim: "pvc",
  ServiceAccount: "serviceaccount",
  ReplicaSet: "replicaset",
  StatefulSet: "statefulset",
  DaemonSet: "daemonset",
  Job: "job",
  CronJob: "cronjob",
  HorizontalPodAutoscaler: "hpa",
  Endpoints: "endpoints",
  PodDisruptionBudget: "poddisruptionbudget",
  NetworkPolicy: "networkpolicy",
  ResourceQuota: "resourcequota",
  LimitRange: "limitrange",
  StorageClass: "storageclass",
  PriorityClass: "priorityclass",
  CustomResourceDefinition: "customresourcedefinition",
};

let memory: CacheRow | null = null;
let contextMemory: { at: number; cluster: Cluster } | null = null;

function invocation(): { cmd: string; prefix: string[] } {
  const cmd = process.env.KUBE_SHELL_KUBECTL || "kubectl";
  const script = process.env.KUBE_SHELL_NODE_SCRIPT;
  return { cmd, prefix: script ? [script] : [] };
}

function run(args: string[], timeout = 8000): { code: number; stdout: string } {
  const { cmd, prefix } = invocation();
  try {
    const stdout = execFileSync(cmd, [...prefix, ...args], {
      encoding: "utf8",
      timeout,
      maxBuffer: 16 * 1024 * 1024,
      stdio: ["ignore", "pipe", "pipe"],
    });
    return { code: 0, stdout };
  } catch (err) {
    const failed = err as { status?: number; stdout?: string | Buffer };
    const stdout = typeof failed.stdout === "string" ? failed.stdout : "";
    return { code: typeof failed.status === "number" ? failed.status : 1, stdout };
  }
}

export function probeLive(): boolean {
  if (memory && Date.now() - memory.at < TTL_MS) return true;
  const result = run(["get", "ns", "--request-timeout=4s", "-o", "name"], 5000);
  return result.code === 0 && result.stdout.trim().length > 0;
}

function readCache(key: string): Cluster | null {
  const now = Date.now();
  if (memory && memory.key === key && now - memory.at < TTL_MS) return memory.cluster;
  try {
    const parsed = JSON.parse(fs.readFileSync(cachePath(), "utf8")) as CacheRow;
    if (parsed.key === key && now - parsed.at < TTL_MS && parsed.cluster?.objects) {
      memory = parsed;
      return parsed.cluster;
    }
  } catch {
    /* cold cache */
  }
  return null;
}

function writeCache(key: string, cluster: Cluster) {
  const row: CacheRow = { at: Date.now(), key, cluster };
  memory = row;
  tryWritePrivate(cachePath(), JSON.stringify(row));
}

export function invalidateLive() {
  memory = null;
  contextMemory = null;
  try {
    fs.rmSync(cachePath(), { force: true });
  } catch {
    /* ignore */
  }
}

type ConfigView = {
  "current-context"?: string;
  contexts?: { name?: string; context?: { namespace?: string; cluster?: string; user?: string } }[];
};

function contextCluster(): Cluster | null {
  if (contextMemory && Date.now() - contextMemory.at < 60_000) return contextMemory.cluster;
  const view = run(["config", "view", "-o", "json"], 4000);
  if (view.code !== 0 || !view.stdout.trim()) return null;
  let parsed: ConfigView;
  try {
    parsed = JSON.parse(view.stdout) as ConfigView;
  } catch {
    return null;
  }
  const current = parsed["current-context"]?.trim() ?? "";
  if (!current) return null;
  const names = run(["config", "get-contexts", "-o", "name"], 4000)
    .stdout.split("\n")
    .map((line) => line.trim())
    .filter(Boolean);
  if (!names.includes(current)) names.unshift(current);
  const namespaceOf = (name: string) =>
    parsed.contexts?.find((item) => item.name === name)?.context?.namespace || "default";
  const cluster: Cluster = {
    current,
    contexts: names.map((name) => ({
      name,
      cluster: parsed.contexts?.find((item) => item.name === name)?.context?.cluster || name,
      user: parsed.contexts?.find((item) => item.name === name)?.context?.user || name,
      namespace: namespaceOf(name),
    })),
    namespaces: [],
    nodes: [],
    pvs: [],
    events: [],
    objects: [],
  };
  contextMemory = { at: Date.now(), cluster };
  return cluster;
}

function contextArgs(context: string, current: string): string[] {
  return context && context !== current ? ["--context", context] : [];
}

function itemsOf(args: string[]): KItem[] {
  const result = run([...args, "-o", "json", "--request-timeout=6s"], 8000);
  if (result.code !== 0 || !result.stdout.trim()) return [];
  try {
    const parsed = JSON.parse(result.stdout) as { items?: KItem[] };
    return parsed.items ?? [];
  } catch {
    return [];
  }
}

export function ageOf(timestamp: string | undefined, now = Date.now()): string {
  if (!timestamp) return "";
  const then = Date.parse(timestamp);
  if (!Number.isFinite(then)) return "";
  const seconds = Math.max(0, Math.round((now - then) / 1000));
  if (seconds < 90) return `${seconds}s`;
  const minutes = Math.round(seconds / 60);
  if (minutes < 90) return `${minutes}m`;
  const hours = Math.round(minutes / 60);
  if (hours < 48) return `${hours}h`;
  return `${Math.round(hours / 24)}d`;
}

function detailOf(item: KItem): string {
  const waiting = item.status?.containerStatuses?.find(
    (container) => container.state?.waiting?.reason,
  )?.state?.waiting?.reason;
  if (waiting) return waiting;
  if (item.kind === "Deployment" || item.kind === "ReplicaSet" || item.kind === "StatefulSet") {
    return `${item.status?.readyReplicas ?? 0}/${item.status?.replicas ?? 0}`;
  }
  if (item.kind === "Service") return item.spec?.type || "Service";
  if (item.kind === "Secret") return item.type || "Secret";
  if (item.kind === "CronJob") return item.spec?.schedule || "CronJob";
  if (item.kind === "Node") return item.status?.nodeInfo?.kubeletVersion || "Ready";
  if (item.kind === "PersistentVolume")
    return `${item.spec?.capacity?.storage ?? ""} ${item.status?.phase ?? ""}`.trim();
  return item.status?.phase || item.kind || "";
}

export function mapListItem(item: KItem): KObject | null {
  const kind = KIND[item.kind ?? ""];
  const name = item.metadata?.name;
  if (!kind || !name) return null;
  return {
    kind,
    name,
    ns: item.metadata?.namespace || "default",
    age: ageOf(item.metadata?.creationTimestamp),
    labels: item.metadata?.labels ?? {},
    status: detailOf(item),
    svcType: item.kind === "Service" ? item.spec?.type : undefined,
    containers: containerNames(item),
  };
}

function containerNames(item: KItem): KObject["containers"] {
  const names = item.spec?.containers?.flatMap((container) =>
    container.name ? [container.name] : [],
  );
  if (!names || names.length === 0) return undefined;
  return names.map((name) => ({ name }));
}

export function mapNode(item: KItem): Cluster["nodes"][number] | null {
  const name = item.metadata?.name;
  if (!name) return null;
  if (item.kind && item.kind !== "Node") return null;
  const ready = item.status?.conditions?.find((condition) => condition.type === "Ready")?.status;
  const status = ready === "True" ? "Ready" : ready === "False" ? "NotReady" : "Unknown";
  return {
    name,
    status,
    roles: "",
    age: ageOf(item.metadata?.creationTimestamp),
    version: item.status?.nodeInfo?.kubeletVersion || "",
    ip: "",
    schedulable: item.spec?.unschedulable !== true,
  };
}

function mapEvent(item: KItem, fallbackNs: string): Cluster["events"][number] | null {
  if (item.kind !== "Event") return null;
  const involved = item.involvedObject;
  const object = involved?.name
    ? `${(involved.kind || "object").toLowerCase()}/${involved.name}`
    : "";
  return {
    ns: item.metadata?.namespace || involved?.namespace || fallbackNs,
    last: ageOf(item.metadata?.creationTimestamp),
    type: item.type || "",
    reason: item.reason || "",
    object,
    message: item.message || "",
  };
}

const OBJECT_GROUPS = [
  ["pods", "services", "configmaps", "serviceaccounts", "persistentvolumeclaims"],
  ["secrets"],
  ["deployments.apps", "replicasets.apps", "statefulsets.apps", "daemonsets.apps"],
  ["jobs.batch", "cronjobs.batch"],
  ["ingresses.networking.k8s.io", "networkpolicies.networking.k8s.io"],
  ["horizontalpodautoscalers.autoscaling"],
  ["endpoints", "poddisruptionbudgets.policy", "resourcequotas", "limitranges"],
];

export const PROJECTED_JSONPATH =
  '{range .items[*]}{.kind}{"\\t"}{.metadata.name}{"\\t"}{.metadata.namespace}{"\\t"}{.metadata.creationTimestamp}{"\\t"}{.type}{"\\t"}{.status.phase}{"\\t"}{.status.readyReplicas}{"\\t"}{.status.replicas}{"\\t"}{.spec.type}{"\\t"}{.spec.schedule}{"\\t"}{.spec.capacity.storage}{"\\t"}{.status.nodeInfo.kubeletVersion}{"\\t"}{.reason}{"\\t"}{.message}{"\\t"}{.involvedObject.kind}{"\\t"}{.involvedObject.name}{"\\t"}{.involvedObject.namespace}{"\\t"}{range .status.containerStatuses[*]}{.state.waiting.reason}{","}{end}{"\\t"}{range .spec.containers[*]}{.name}{","}{end}{range .spec.template.spec.containers[*]}{.name}{","}{end}{"\\t"}{range $k,$v := .metadata.labels}{$k}={$v}{","}{end}{"\\n"}{end}';

export function parseProjected(stdout: string): KItem[] {
  if (!stdout.trim()) return [];
  const items: KItem[] = [];
  for (const line of stdout.split("\n")) {
    if (!line.trim()) continue;
    const cols = line.split("\t");
    if (cols.length !== 20) continue;
    const [
      kind,
      name,
      namespace,
      creationTimestamp,
      type,
      phase,
      readyReplicas,
      replicas,
      specType,
      schedule,
      storage,
      kubeletVersion,
      reason,
      message,
      involvedKind,
      involvedName,
      involvedNs,
      waitingRaw,
      containersRaw,
      labelsRaw,
    ] = cols;

    const labels: Record<string, string> = {};
    if (labelsRaw) {
      for (const pair of labelsRaw.split(",")) {
        if (!pair) continue;
        const eq = pair.indexOf("=");
        if (eq >= 0) {
          labels[pair.slice(0, eq)] = pair.slice(eq + 1);
        }
      }
    }

    const containerStatuses = waitingRaw
      ? waitingRaw
          .split(",")
          .filter(Boolean)
          .map((r) => ({ state: { waiting: { reason: r } } }))
      : undefined;

    const item: KItem = {
      kind: kind || undefined,
      metadata: {
        name: name || undefined,
        namespace: namespace || undefined,
        creationTimestamp: creationTimestamp || undefined,
        labels,
      },
      type: type || undefined,
      reason: reason || undefined,
      message: message || undefined,
      involvedObject: {
        kind: involvedKind || undefined,
        name: involvedName || undefined,
        namespace: involvedNs || undefined,
      },
      spec: {
        type: specType || undefined,
        schedule: schedule || undefined,
        capacity: storage ? { storage } : undefined,
        containers: containersRaw
          ? containersRaw
              .split(",")
              .filter(Boolean)
              .map((name) => ({ name }))
          : undefined,
      },
      status: {
        phase: phase || undefined,
        readyReplicas: readyReplicas ? Number(readyReplicas) : undefined,
        replicas: replicas ? Number(replicas) : undefined,
        nodeInfo: kubeletVersion ? { kubeletVersion } : undefined,
        containerStatuses,
      },
    };
    items.push(item);
  }
  return items;
}

function fetchProjectedLists(ctx: string[], scope: string[], groups: string[][]): KItem[][] {
  const combinedName = groups.flat().join(",");
  const combined = run(
    [...ctx, "get", combinedName, ...scope, "-o", PROJECTED_JSONPATH, "--request-timeout=6s"],
    8000,
  );
  if (combined.code === 0) return [parseProjected(combined.stdout)];

  const lists: KItem[][] = [];
  for (const group of groups) {
    const res = run(
      [...ctx, "get", group.join(","), ...scope, "-o", PROJECTED_JSONPATH, "--request-timeout=6s"],
      8000,
    );
    if (res.code === 0) {
      lists.push(parseProjected(res.stdout));
      continue;
    }
    for (const kind of group) {
      const single = run(
        [...ctx, "get", kind, ...scope, "-o", PROJECTED_JSONPATH, "--request-timeout=6s"],
        8000,
      );
      if (single.code === 0) lists.push(parseProjected(single.stdout));
    }
  }
  return lists;
}

function fetchEvents(ctx: string[], ns: string, all: boolean): Cluster["events"] {
  const scope = all ? ["--all-namespaces"] : ["-n", ns || "default"];
  const items = itemsOf([...ctx, "get", "events", ...scope]);
  return items.flatMap((item) => {
    const event = mapEvent(item, ns || "default");
    return event ? [event] : [];
  });
}

function fetchObjects(
  ctx: string[],
  ns: string,
  all: boolean,
): { objects: KObject[]; events: Cluster["events"] } {
  const scope = all ? ["--all-namespaces"] : ["-n", ns || "default"];
  const lists = fetchProjectedLists(ctx, scope, OBJECT_GROUPS);
  const objects: KObject[] = [];
  for (const items of lists) {
    for (const item of items) {
      const mapped = mapListItem(item);
      if (!mapped) continue;
      if (!mapped.ns) mapped.ns = ns || "default";
      objects.push(mapped);
    }
  }
  return { objects, events: fetchEvents(ctx, ns, all) };
}

/** Build a completion snapshot from the current kubeconfig and API. */
export function liveCluster(engineLine: string): Cluster | null {
  const base = contextCluster();
  if (!base) return null;
  const tokens = tokenize(engineLine).map((token) => unquote(token.text));
  const scope = scopeOf(tokens.length ? tokens : ["kubectl"], base);
  const key = `${base.current}|${scope.context}|${scope.all ? "*" : scope.ns}`;
  const cached = readCache(key);
  if (cached) return cached;
  const ctx = contextArgs(scope.context, base.current);
  const scoped = itemsOf([
    ...ctx,
    "get",
    "ns,nodes,pv,storageclasses.storage.k8s.io,priorityclasses.scheduling.k8s.io,customresourcedefinitions.apiextensions.k8s.io",
  ]);
  const namespaces = scoped
    .filter((item) => item.kind === "Namespace" && item.metadata?.name)
    .map((item) => ({
      name: item.metadata?.name ?? "",
      status: item.status?.phase || "Active",
      age: ageOf(item.metadata?.creationTimestamp),
    }));
  const nodes = scoped.flatMap((item) =>
    item.kind === "Node" ? [mapNode(item)].filter((node) => node !== null) : [],
  );
  const pvs = scoped.flatMap((item) => {
    if (item.kind !== "PersistentVolume" || !item.metadata?.name) return [];
    return [
      {
        name: item.metadata.name,
        capacity: item.spec?.capacity?.storage || "",
        mode: "",
        status: item.status?.phase || "",
        claim: "",
        age: ageOf(item.metadata.creationTimestamp),
      },
    ];
  });
  const fetched = fetchObjects(ctx, scope.ns, scope.all);
  const clusterObjects = scoped.flatMap((item) => {
    const mapped = mapListItem(item);
    return mapped ? [mapped] : [];
  });
  const cluster: Cluster = {
    ...base,
    namespaces: namespaces.length ? namespaces : base.namespaces,
    nodes,
    pvs,
    events: fetched.events,
    objects: [...clusterObjects, ...fetched.objects],
  };
  writeCache(key, cluster);
  return cluster;
}
export function applyLiveContext(cluster: Cluster): Cluster {
  const base = contextCluster();
  if (!base) return cluster;
  return { ...cluster, current: base.current, contexts: base.contexts };
}

export function kubectlArgs(engineLine: string): string[] {
  return tokenize(engineLine)
    .slice(1)
    .map((token) => unquote(token.text));
}

export type KubectlJsonResult = { ok: boolean; json: unknown; error: string };

function stderrLine(stderr: string): string {
  const line = stderr
    .split(/\r?\n/)
    .map((item) => item.trim())
    .find(Boolean);
  return (line ?? "").slice(0, 120);
}

/** Read one kubectl JSON document. Does not read or write the completion cache. */
export function runJson(args: string[], timeoutMs = 3000): KubectlJsonResult {
  const { cmd, prefix } = invocation();
  const text = (value: string | Buffer | undefined) =>
    typeof value === "string" ? value : value ? value.toString("utf8") : "";
  try {
    const stdout = execFileSync(cmd, [...prefix, ...args], {
      encoding: "utf8",
      timeout: timeoutMs,
      maxBuffer: 16 * 1024 * 1024,
      stdio: ["ignore", "pipe", "pipe"],
    });
    try {
      return { ok: true, json: JSON.parse(stdout), error: "" };
    } catch {
      return { ok: false, json: null, error: "invalid JSON" };
    }
  } catch (err) {
    const failed = err as {
      code?: string;
      stdout?: string | Buffer;
      stderr?: string | Buffer;
    };
    const line = stderrLine(text(failed.stderr));
    const error = line || (failed.code === "ETIMEDOUT" ? "timed out" : "failed");
    return { ok: false, json: null, error };
  }
}

export function captureLive(args: string[]): { code: number; stdout: string; stderr: string } {
  const { cmd, prefix } = invocation();
  try {
    const stdout = execFileSync(cmd, [...prefix, ...args], {
      encoding: "utf8",
      timeout: 8000,
      maxBuffer: 4 * 1024 * 1024,
      stdio: ["ignore", "pipe", "pipe"],
    });
    return { code: 0, stdout, stderr: "" };
  } catch (err) {
    const failed = err as { status?: number; stdout?: string | Buffer; stderr?: string | Buffer };
    const text = (value: string | Buffer | undefined) =>
      typeof value === "string" ? value : value ? value.toString("utf8") : "";
    return {
      code: typeof failed.status === "number" ? failed.status : 1,
      stdout: text(failed.stdout),
      stderr: text(failed.stderr),
    };
  }
}

export type RunLiveOptions = { inheritStdin?: boolean };

export function runLive(args: string[], options: RunLiveOptions = {}): number {
  if (wantsRedactedStdout(args) && !process.stdout.isTTY) {
    const { code, stdout, stderr } = captureLive(args);
    if (stderr) process.stderr.write(stderr);
    process.stdout.write(redactKubectlStdout(args, stdout));
    invalidateLive();
    return code;
  }
  const { cmd, prefix } = invocation();
  const stdin = process.stdin.isTTY || options.inheritStdin ? "inherit" : "ignore";
  const result = spawnSync(cmd, [...prefix, ...args], { stdio: [stdin, "inherit", "inherit"] });
  invalidateLive();
  if (result.error) {
    process.stderr.write(`kubectl: ${result.error.message}\n`);
    return 127;
  }
  return result.status ?? 1;
}
