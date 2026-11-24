import fs from "node:fs";
import path from "node:path";
import {
  EXPLAIN,
  RESOURCES,
  SUBS,
  VERBS,
  resolveResource,
  type Cluster,
  type Resource,
} from "./model";
import { flagsForVerb, type Scope } from "./parse";
import type { Candidate } from "./complete";

function resourceItems(filter?: (resource: Resource) => boolean): Candidate[] {
  return RESOURCES.filter((resource) => (filter ? filter(resource) : true)).map((resource) => ({
    value: resource.name,
    detail: resource.detail,
    kind: "resource",
    weight: resource.weight,
    aliases: [resource.singular, ...resource.short],
  }));
}

export function inScope(ns: string, scope: Scope): boolean {
  return scope.all || ns === scope.ns;
}

function namesOf(resource: Resource, scope: Scope, cluster: Cluster): Candidate[] {
  if (resource.kind === "namespace") {
    return cluster.namespaces.map((item) => ({
      value: item.name,
      detail: item.status,
      kind: "namespace",
      weight: 50,
    }));
  }
  if (resource.kind === "node") {
    return cluster.nodes.map((item) => ({
      value: item.name,
      detail: item.schedulable ? item.roles : "unschedulable",
      kind: "node",
      weight: 50,
    }));
  }
  if (resource.kind === "pv") {
    if (resource.name !== "persistentvolumes") return [];
    return cluster.pvs.map((item) => ({
      value: item.name,
      detail: `${item.capacity} ${item.status}`,
      kind: "volume",
      weight: 40,
    }));
  }
  if (resource.kind === "event") {
    if (resource.name !== "events") return [];
    return cluster.events
      .filter((item) => inScope(item.ns, scope))
      .map((item) => ({
        value: `${item.reason.toLowerCase()}.${item.object.replace("/", "-")}`,
        detail: item.message,
        kind: "event",
        weight: 20,
      }));
  }
  return cluster.objects
    .filter(
      (item) => item.kind === resource.kind && (!resource.namespaced || inScope(item.ns, scope)),
    )
    .map((item) => ({
      value: item.name,
      detail: scope.all
        ? `${item.ns} · ${item.status ?? item.kind}`
        : (item.status ?? item.svcType ?? item.kind),
      kind: item.kind,
      weight: item.status === "CrashLoopBackOff" || item.status === "Pending" ? 70 : 50,
    }));
}

export function slashNames(
  typeToken: string,
  namePrefix: string,
  scope: Scope,
  cluster: Cluster,
): Candidate[] {
  const resource = resolveResource(typeToken);
  if (!resource) return [];
  return namesOf(resource, scope, cluster)
    .filter((item) => item.value.toLowerCase().startsWith(namePrefix.toLowerCase()))
    .map((item) => ({ ...item, value: `${typeToken}/${item.value}` }));
}

function workloadSlash(kinds: Resource["kind"][], scope: Scope, cluster: Cluster): Candidate[] {
  const out: Candidate[] = [];
  for (const kind of kinds) {
    const resource =
      RESOURCES.find((item) => item.kind === kind && item.name.endsWith("s")) ??
      RESOURCES.find((item) => item.kind === kind);
    if (!resource) continue;
    for (const item of namesOf(resource, scope, cluster)) {
      out.push({
        value: `${resource.singular}/${item.value}`,
        detail: item.detail,
        kind: resource.singular,
        weight: (item.weight ?? 40) - 4,
      });
    }
  }
  return out;
}

export function manifestCandidates(prefix: string, root = process.cwd()): Candidate[] {
  const normalized = prefix.replace(/\\/g, "/");
  const slash = normalized.lastIndexOf("/");
  const dirPart = slash >= 0 ? normalized.slice(0, slash) : "";
  const dir = path.resolve(root, dirPart);
  const relative = path.relative(root, dir);
  if (relative.startsWith("..") || path.isAbsolute(relative)) return [];
  let entries: fs.Dirent[];
  try {
    entries = fs.readdirSync(dir, { withFileTypes: true });
  } catch {
    return [];
  }
  const out: Candidate[] = [];
  for (const entry of entries) {
    if (entry.name.startsWith(".") || entry.name === "node_modules" || entry.isSymbolicLink())
      continue;
    const rel = (dirPart ? `${dirPart}/${entry.name}` : entry.name).replace(/\\/g, "/");
    if (!rel.toLowerCase().startsWith(normalized.toLowerCase())) continue;
    if (entry.isDirectory())
      out.push({ value: `${rel}/`, detail: "Directory", kind: "file", weight: 35 });
    else if (/\.ya?ml$/i.test(entry.name))
      out.push({ value: rel, detail: "Manifest", kind: "file", weight: 40 });
  }
  return out;
}

export function flagItems(verb: string | undefined): Candidate[] {
  return flagsForVerb(verb).map((flag) => ({
    value: flag.name,
    detail: flag.detail,
    kind: "flag",
    weight: flag.weight,
  }));
}

function ports(): Candidate[] {
  return [
    { value: "8080:80", detail: "Local 8080 to pod 80", kind: "port", weight: 80 },
    { value: "8443:443", detail: "Local 8443 to pod 443", kind: "port", weight: 50 },
    { value: "3000:3000", detail: "Grafana-style forward", kind: "port", weight: 40 },
    { value: "9090:9090", detail: "Prometheus-style forward", kind: "port", weight: 36 },
  ];
}

function targets(scope: Scope, cluster: Cluster, kinds: Resource["kind"][]): Candidate[] {
  const types = resourceItems((resource) => kinds.includes(resource.kind));
  return [...types, ...workloadSlash(kinds, scope, cluster)];
}

function afterResource(rest: string[], scope: Scope, cluster: Cluster, verb: string): Candidate[] {
  if (rest.length === 0) return resourceItems();
  if (rest.length === 1) {
    const resource = resolveResource(rest[0] ?? "");
    if (!resource || rest[0]?.includes("/")) return flagItems(verb);
    const names = namesOf(resource, scope, cluster);
    return names.length > 0 ? names : flagItems(verb);
  }
  return flagItems(verb);
}

const NAME_VERBS = new Set([
  "get",
  "describe",
  "delete",
  "edit",
  "label",
  "annotate",
  "patch",
  "wait",
]);
const FLAG_VERBS = new Set([
  "completion",
  "version",
  "api-resources",
  "api-versions",
  "cluster-info",
  "proxy",
  "run",
]);

type Fill = {
  verb: string;
  rest: string[];
  scope: Scope;
  cluster: Cluster;
  prefix: string;
};

const FILL: Record<string, (ctx: Fill) => Candidate[]> = {
  config(ctx) {
    const { rest, cluster, verb } = ctx;
    const sub = rest[0];
    if (sub === "use-context" || sub === "delete-context") {
      if (rest.length === 1) {
        return cluster.contexts.map((item) => ({
          value: item.name,
          detail: item.cluster,
          kind: "context",
          weight: 50,
        }));
      }
    }
    if (sub === "set-context" && rest.length === 1) {
      return [
        ...cluster.contexts.map((item) => ({
          value: item.name,
          detail: item.cluster,
          kind: "context",
          weight: 40,
        })),
        { value: "--current", detail: "Modify the active context", kind: "flag", weight: 80 },
        { value: "--namespace", detail: "Set the default namespace", kind: "flag", weight: 70 },
      ];
    }
    return flagItems(verb);
  },
  rollout(ctx) {
    const { rest, scope, cluster, verb } = ctx;
    const sub = rest[0] ?? "";
    const after = rest.slice(1);
    if (!sub) return [];
    if (after.length === 0)
      return targets(scope, cluster, ["deployment", "statefulset", "daemonset"]);
    if (after.length === 1 && !after[0]?.includes("/")) {
      const resource = resolveResource(after[0] ?? "");
      if (resource) return namesOf(resource, scope, cluster);
    }
    return flagItems(verb);
  },
  auth(ctx) {
    const { rest, verb } = ctx;
    if (rest.length === 1) {
      return ["get", "list", "watch", "create", "update", "patch", "delete", "*"].map(
        (value, index) => ({
          value,
          detail: "Verb to check",
          kind: "verb",
          weight: 80 - index,
        }),
      );
    }
    if (rest.length === 2) return resourceItems();
    return flagItems(verb);
  },
  create(ctx) {
    if (ctx.rest.length === 0) return [];
    return flagItems(ctx.verb);
  },
  set(ctx) {
    const { rest, scope, cluster, verb } = ctx;
    const sub = rest[0];
    const after = rest.slice(1);
    if (!sub) return [];
    if (after.length === 0)
      return targets(scope, cluster, ["deployment", "statefulset", "daemonset"]);
    if (sub === "image" && after.length >= 1) return [];
    return flagItems(verb);
  },
  top() {
    return flagItems("get");
  },
  logs: fillPodish,
  exec: fillPodish,
  debug: fillPodish,
  cp: fillPodish,
  "port-forward"(ctx) {
    const { rest, scope, cluster, verb } = ctx;
    if (rest.length === 0) {
      const pods = namesOf(resolveResource("pods")!, scope, cluster);
      return [...pods, ...workloadSlash(["service", "deployment"], scope, cluster)];
    }
    if (rest.length === 1) return ports();
    return flagItems(verb);
  },
  scale(ctx) {
    const { rest, scope, cluster, verb } = ctx;
    if (rest.length === 0)
      return targets(scope, cluster, ["deployment", "statefulset", "replicaset"]);
    if (rest.length === 1 && !rest[0]?.includes("/")) {
      const resource = resolveResource(rest[0] ?? "");
      if (resource) return namesOf(resource, scope, cluster);
    }
    return flagItems(verb);
  },
  expose(ctx) {
    const { rest, scope, cluster, verb } = ctx;
    if (rest.length === 0)
      return resourceItems((resource) =>
        ["deployment", "pod", "service", "replicaset"].includes(resource.kind),
      );
    if (rest.length === 1) {
      const resource = resolveResource(rest[0] ?? "");
      if (resource) return namesOf(resource, scope, cluster);
    }
    return flagItems(verb);
  },
  explain(ctx) {
    if (ctx.rest.length === 0) {
      return Object.entries(EXPLAIN).map(([value, info], index) => ({
        value,
        detail: info.detail,
        kind: "field",
        weight: 100 - index,
      }));
    }
    return [];
  },
  cordon: fillNode,
  uncordon: fillNode,
  drain: fillNode,
  taint: fillNode,
  apply: fillManifest,
  diff: fillManifest,
  replace: fillManifest,
  help() {
    return VERBS.map((item) => ({
      value: item.name,
      detail: item.detail,
      kind: "verb",
      weight: item.weight,
    }));
  },
  kustomize(ctx) {
    return manifestCandidates(ctx.prefix).filter((item) => item.value.endsWith("/"));
  },
};

function fillPodish(ctx: Fill): Candidate[] {
  const { verb, rest, scope, cluster } = ctx;
  if (rest.length === 0) {
    const pods = namesOf(resolveResource("pods")!, scope, cluster).map((item) => ({
      ...item,
      weight: 80,
    }));
    if (verb === "cp") {
      return pods.map((item) => ({
        ...item,
        value: `${item.value}:/app`,
        detail: "pod path",
        kind: "path",
      }));
    }
    return [...pods, ...workloadSlash(["deployment", "statefulset", "job"], scope, cluster)];
  }
  return flagItems(verb);
}

function fillNode(ctx: Fill): Candidate[] {
  if (ctx.rest.length === 0) return namesOf(resolveResource("nodes")!, ctx.scope, ctx.cluster);
  return flagItems(ctx.verb);
}

function fillManifest(ctx: Fill): Candidate[] {
  if (ctx.rest.length === 0 && !ctx.prefix.startsWith("-")) {
    return [
      { value: "-f", detail: "Apply or diff a file", kind: "flag", weight: 100 },
      { value: "-k", detail: "Kustomize directory", kind: "flag", weight: 70 },
      ...manifestCandidates(ctx.prefix),
    ];
  }
  return flagItems(ctx.verb);
}

export function candidatesFor(
  verb: string,
  rest: string[],
  _args: string[],
  scope: Scope,
  cluster: Cluster,
  prefix: string,
): Candidate[] {
  const subs = SUBS[verb];
  if (subs && rest.length === 0 && !prefix.includes("/")) {
    return subs.map((sub) => ({
      value: sub.value,
      detail: sub.detail,
      kind: "command",
      weight: sub.weight,
    }));
  }
  const fill = FILL[verb];
  if (fill) return fill({ verb, rest, scope, cluster, prefix });
  if (FLAG_VERBS.has(verb)) return flagItems(verb);
  if (NAME_VERBS.has(verb)) return afterResource(rest, scope, cluster, verb);
  return flagItems(verb);
}
