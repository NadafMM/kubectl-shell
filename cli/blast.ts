import type { Resource } from "../src/lib/kube/model.ts";
import { resolveResource } from "../src/lib/kube/model.ts";
import { flagValue, positionalWords } from "../src/lib/kube/parse.ts";
import type { Plan } from "./insight.ts";
import { manifestRefs, type ManifestRef } from "./manifest.ts";

export const BLAST_BUDGET_MS = 3000;

export type KubectlJsonResult = { ok: boolean; json: unknown; error: string };
export type GetJson = (args: string[]) => KubectlJsonResult;

export type BlastObject = { kind: string; namespace: string; name: string };

export type BlastService = {
  namespace: string;
  name: string;
  matchedPods: number;
  readyPodsRemaining: number;
};

export type Blast = {
  status: "ok" | "unknown" | "skipped";
  reason?: string;
  controllers?: BlastObject[];
  objects: BlastObject[];
  objectCount: number;
  services: BlastService[];
  serviceNote?: string;
  pdbs?: { namespace: string; name: string; disruptionsAllowed: number }[];
  pdbNote?: string;
  hpas?: {
    namespace: string;
    name: string;
    minReplicas: number;
    maxReplicas: number;
    target: string;
  }[];
  owners?: { namespace: string; pod: string; kind: string; name: string }[];
  dryRun?: { status: "ok" | "unknown"; count?: number; reason?: string };
};

export type BlastClock = { now: () => number };

type KubeObject = {
  kind?: string;
  metadata?: {
    name?: string;
    namespace?: string;
    labels?: Record<string, string>;
    ownerReferences?: { kind?: string; name?: string; controller?: boolean }[];
  };
  spec?: {
    selector?: unknown;
    minReplicas?: number;
    maxReplicas?: number;
    scaleTargetRef?: { kind?: string; name?: string };
  };
  status?: { conditions?: { type?: string; status?: string }[]; disruptionsAllowed?: number };
};

type CountGroup = { namespace: string; kind: string; count: number };
type Subject = { kind: string; namespace: string; groups?: CountGroup[] };

const subjects = new WeakMap<Blast, Subject>();

function skipped(reason: string): Blast {
  return { status: "skipped", reason, objects: [], objectCount: 0, services: [] };
}

function unknown(reason: string): Blast {
  return { status: "unknown", reason, objects: [], objectCount: 0, services: [] };
}

function noun(kind: string, count: number): string {
  return count === 1 ? kind : `${kind}s`;
}

function orderAndCap(objects: BlastObject[]): { shown: BlastObject[]; groups: CountGroup[] } {
  const counts = new Map<string, CountGroup>();
  for (const object of objects) {
    const key = `${object.namespace}\0${object.kind}`;
    const group = counts.get(key) ?? { namespace: object.namespace, kind: object.kind, count: 0 };
    group.count += 1;
    counts.set(key, group);
  }
  const ordered = objects.map((object, index) => ({ object, index }));
  ordered.sort((a, b) => a.object.namespace.localeCompare(b.object.namespace) || a.index - b.index);
  const groups = [...counts.values()].sort(
    (a, b) => a.namespace.localeCompare(b.namespace) || a.kind.localeCompare(b.kind),
  );
  return { shown: ordered.slice(0, 20).map((item) => item.object), groups };
}

function pack(
  objects: BlastObject[],
  partial: { kind: string; namespace: string },
  extra: Partial<Blast> = {},
): Blast {
  const { shown, groups } = orderAndCap(objects);
  return remember(
    { status: "ok", objects: shown, objectCount: objects.length, services: [], ...extra },
    { ...partial, groups },
  );
}

function formatOk(blast: Blast): string {
  const lines = ["blast"];
  const subject = subjects.get(blast);
  const groups = subject?.groups ?? [];
  if (blast.objectCount === 0 || groups.length === 0) {
    const kind = subject?.kind ?? "object";
    const ns = subject?.namespace ?? "default";
    lines.push(`  0 ${noun(kind, 0)} in namespace ${ns}${fromSuffix(blast, ns, kind)}`);
  } else {
    for (const group of groups) {
      lines.push(
        `  ${group.count} ${noun(group.kind, group.count)} in namespace ${group.namespace}${fromSuffix(blast, group.namespace, group.kind)}`,
      );
    }
    for (const object of blast.objects) lines.push(objectLine(blast, object));
    if (blast.objectCount > 20) lines.push(`  and ${blast.objectCount - 20} more`);
  }
  const multi = new Set(groups.map((group) => group.namespace)).size > 1;
  for (const service of blast.services) lines.push(serviceLine(blast, service, multi));
  if (blast.serviceNote) lines.push(`  ${blast.serviceNote}`);
  for (const pdb of blast.pdbs ?? []) {
    const word = pdb.disruptionsAllowed === 1 ? "disruption" : "disruptions";
    lines.push(`  pdb/${pdb.name} allows ${pdb.disruptionsAllowed} ${word}`);
  }
  if (blast.pdbNote) lines.push(`  ${blast.pdbNote}`);
  for (const hpa of blast.hpas ?? []) {
    lines.push(
      `  hpa/${hpa.name} keeps ${hpa.target} between ${hpa.minReplicas} and ${hpa.maxReplicas}`,
    );
  }
  if (blast.dryRun?.status === "ok") lines.push(`  dry-run: ${blast.dryRun.count ?? 0} objects`);
  if (blast.dryRun?.status === "unknown") {
    lines.push(`  dry-run: unknown (${blast.dryRun.reason ?? "failed"})`);
  }
  return lines.join("\n");
}

function objectLine(blast: Blast, object: BlastObject): string {
  const owner = blast.owners?.find(
    (item) => item.pod === object.name && item.namespace === object.namespace,
  );
  const owned = owner ? ` owned by ${owner.kind}/${owner.name}` : "";
  return `  ${object.kind}/${object.name}${owned}`;
}

function fromSuffix(blast: Blast, namespace: string, kind: string): string {
  if (kind !== "pod" || !blast.controllers?.length) return "";
  const names = blast.controllers
    .filter((item) => item.namespace === namespace)
    .map((item) => `${item.kind}/${item.name}`);
  return names.length > 0 ? `, from ${names.join(", ")}` : "";
}

function deletedPodsIn(blast: Blast, namespace: string): number {
  const groups = subjects.get(blast)?.groups;
  if (groups) {
    return groups
      .filter((group) => group.namespace === namespace && group.kind === "pod")
      .reduce((sum, group) => sum + group.count, 0);
  }
  return blast.objects.filter((object) => object.kind === "pod" && object.namespace === namespace)
    .length;
}

function serviceLine(blast: Blast, service: BlastService, multi: boolean): string {
  const where = multi ? ` in namespace ${service.namespace}` : "";
  const deleted = deletedPodsIn(blast, service.namespace);
  const matched =
    service.matchedPods === deleted && deleted > 1
      ? `matches all ${service.matchedPods}`
      : `matches ${service.matchedPods}`;
  const ready =
    service.readyPodsRemaining === 0
      ? "would have 0 ready pods"
      : `would keep ${service.readyPodsRemaining} ready pod${service.readyPodsRemaining === 1 ? "" : "s"}`;
  return `  service/${service.name}${where} ${matched} and ${ready}`;
}

export function formatBlast(blast: Blast): string {
  if (blast.status === "skipped" && blast.reason === "not a delete") return "";
  if (blast.status === "skipped" && blast.reason === "no name or selector") {
    return "blast: not expanded (no name or selector)";
  }
  if (blast.status === "skipped") return `blast: skipped (${blast.reason ?? ""})`;
  if (blast.status === "unknown") return `blast: unknown (${blast.reason ?? ""})`;
  return formatOk(blast);
}

function itemsOf(json: unknown, expectedKind: string | undefined): KubeObject[] | null {
  if (!json || typeof json !== "object") return null;
  const doc = json as KubeObject & { items?: unknown };
  if (expectedKind && doc.kind === expectedKind && doc.metadata?.name) return [doc];
  if (!expectedKind && doc.kind && doc.metadata?.name) return [doc];
  if (typeof doc.kind !== "string" || (doc.kind !== "List" && !doc.kind.endsWith("List"))) {
    return null;
  }
  if (!Array.isArray(doc.items)) return null;
  const items: KubeObject[] = [];
  for (const item of doc.items) {
    if (!item || typeof item !== "object") return null;
    const object = item as KubeObject;
    if (!object.metadata?.name) return null;
    if (expectedKind && object.kind !== expectedKind) return null;
    if (!expectedKind && !object.kind) return null;
    items.push(object);
  }
  return items;
}

type Target = {
  token: string;
  names: string[];
  resource: Resource | undefined;
  selector?: string;
  all: boolean;
};

type Asked = { namespace?: string; allNamespaces: boolean; context?: string };

function askedOf(args: string[]): Asked {
  let namespace: string | undefined;
  let context: string | undefined;
  let allNamespaces = false;
  for (let i = 0; i < args.length; i += 1) {
    const token = args[i]!;
    if (
      token === "-A" ||
      token === "--all-namespaces" ||
      token === "-A=true" ||
      token === "--all-namespaces=true"
    ) {
      allNamespaces = true;
    } else if (token.startsWith("-n=")) namespace = token.slice("-n=".length);
    else if ((token === "-n" || token === "--namespace") && args[i + 1]) namespace = args[++i];
    else if (token.startsWith("--namespace=")) namespace = token.slice("--namespace=".length);
    else if (token === "--context" && args[i + 1]) context = args[++i];
    else if (token.startsWith("--context=")) context = token.slice("--context=".length);
  }
  return { namespace, allNamespaces, context };
}

function askedFlags(asked: Asked): string[] {
  const flags: string[] = [];
  if (asked.namespace) flags.push("-n", asked.namespace);
  if (asked.context) flags.push("--context", asked.context);
  if (asked.allNamespaces) flags.push("-A");
  return flags;
}

function withContext(flags: string[], context?: string): string[] {
  return context ? [...flags, "--context", context] : flags;
}

function targetOf(args: string[]): Target | "mixed" | null {
  const rest = positionalWords(args).words.slice(1);
  const selector = flagValue(args, ["-l", "--selector"]);
  const all = args.some((token) => token === "--all" || token === "--all=true");
  const slash = rest.filter((word) => word.includes("/"));
  if (slash.length > 0) {
    if (slash.length !== rest.length) return "mixed";
    const kinds = slash.map((word) => word.slice(0, word.indexOf("/")));
    const names = slash.map((word) => word.slice(word.indexOf("/") + 1));
    if (new Set(kinds).size !== 1 || names.some((name) => name.length === 0)) return "mixed";
    const token = kinds[0]!;
    return { token, names, resource: resolveResource(token), selector: undefined, all: false };
  }
  const token = rest[0];
  if (!token) return null;
  const resource = resolveResource(token);
  const names = rest.slice(1);
  if (!all && !selector && names.length === 0) return null;
  if (all) return { token, names: [], resource, selector: undefined, all: true };
  if (selector) return { token, names: [], resource, selector, all: false };
  return { token, names, resource, selector: undefined, all: false };
}

function remember(blast: Blast, subject: Subject): Blast {
  subjects.set(blast, subject);
  return blast;
}

type PodRef = {
  namespace: string;
  name: string;
  labels: Record<string, string>;
  ready: boolean;
};

type Fetch = { getJson: GetJson; now: () => number; started: number };

function expired(fetch: Fetch): boolean {
  return fetch.now() - fetch.started >= BLAST_BUDGET_MS;
}

function jsonArgs(resource: string, middle: string[], scope: string[]): string[] {
  return ["get", resource, ...middle, ...scope, "-o", "json", "--request-timeout=4s"];
}

function fetchJson(fetch: Fetch, args: string[]): KubectlJsonResult | "timeout" {
  if (expired(fetch)) return "timeout";
  const result = fetch.getJson(args);
  if (expired(fetch)) return "timeout";
  return result;
}

function scopeFlags(namespaces: string[]): string[] {
  if (namespaces.length > 1) return ["-A"];
  const ns = namespaces[0];
  return ns ? ["-n", ns] : [];
}

function stringMap(value: unknown): Record<string, string> | null {
  if (!value || typeof value !== "object" || Array.isArray(value)) return null;
  const entries = Object.entries(value as Record<string, unknown>);
  if (entries.length === 0) return null;
  const map: Record<string, string> = {};
  for (const [key, entry] of entries) {
    if (typeof entry !== "string") return null;
    map[key] = entry;
  }
  return map;
}

function labelsMatch(selector: Record<string, string>, labels: Record<string, string>): boolean {
  return Object.entries(selector).every(([key, value]) => labels[key] === value);
}

function podRef(item: KubeObject, fallbackNs: string): PodRef | null {
  const name = item.metadata?.name;
  if (!name) return null;
  return {
    namespace: item.metadata?.namespace || fallbackNs,
    name,
    labels: item.metadata?.labels ?? {},
    ready:
      item.status?.conditions?.some(
        (condition) => condition.type === "Ready" && condition.status === "True",
      ) === true,
  };
}

const CONTROLLERS = new Set(["Deployment", "StatefulSet", "DaemonSet", "ReplicaSet"]);

function selectorOf(item: KubeObject): Record<string, string> | null {
  const raw = item.spec?.selector;
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return null;
  const selector = raw as { matchLabels?: unknown; matchExpressions?: unknown };
  if (Array.isArray(selector.matchExpressions) && selector.matchExpressions.length > 0) return null;
  return stringMap(selector.matchLabels);
}

function labelSelector(labels: Record<string, string>): string {
  return Object.keys(labels)
    .sort()
    .map((key) => `${key}=${labels[key]}`)
    .join(",");
}

function expandControllers(
  items: KubeObject[],
  controllers: BlastObject[],
  fetch: Fetch,
  asked: Asked,
  fallbackNs: string,
  partial: { kind: string; namespace: string },
): Blast {
  const named = { controllers: controllers.slice(0, 20) };
  const selectors: Record<string, string>[] = [];
  const namespaces = new Set<string>();
  for (const item of items) {
    const selector = selectorOf(item);
    if (!selector) return withExtras(pack(controllers, partial, named), [], fetch, asked);
    selectors.push(selector);
    namespaces.add(item.metadata?.namespace || fallbackNs);
  }
  const encoded = new Set(selectors.map(labelSelector));
  if (encoded.size !== 1 || (namespaces.size > 1 && !asked.allNamespaces)) {
    return withExtras(pack(controllers, partial, named), [], fetch, asked);
  }
  const scope = asked.allNamespaces
    ? withContext(["-A"], asked.context)
    : withContext(scopeFlags([...namespaces]), asked.context);
  const result = fetchJson(fetch, jsonArgs("pods", ["-l", [...encoded][0]!], scope));
  if (result === "timeout") return unknown("timed out");
  if (!result.ok) return unknown(`kubectl get pods: ${result.error || "failed"}`);
  const podItems = itemsOf(result.json, "Pod");
  if (!podItems) return unknown("kubectl get pods: not a Pod object or list");
  const objects = podItems.flatMap((item) => {
    const name = item.metadata?.name;
    if (!name) return [];
    return [{ kind: "pod", namespace: item.metadata?.namespace || fallbackNs, name }];
  });
  const pods = podItems.flatMap((item) => {
    const pod = podRef(item, fallbackNs);
    return pod ? [pod] : [];
  });
  const podBase = pack(objects, { kind: "pod", namespace: fallbackNs }, named);
  const withServices = attachServices(podBase, pods, fetch, fallbackNs, {
    context: asked.context,
    reuse: false,
  });
  if (withServices.status !== "ok") return withServices;
  return withExtras(remember(withServices, subjects.get(podBase)!), pods, fetch, asked);
}

function serviceNote(base: Blast, error: string): Blast {
  return { ...base, services: [], serviceNote: `services unknown (${error || "failed"})` };
}

function attachServices(
  base: Blast,
  pods: PodRef[],
  fetch: Fetch,
  fallbackNs: string,
  options: { context?: string; reuse: boolean },
): Blast {
  if (pods.length === 0) return base;
  if (pods.length > 200) {
    return {
      ...base,
      services: [],
      serviceNote: `service check skipped (${pods.length} pods exceeds 200)`,
    };
  }
  const namespaces = [...new Set(pods.map((pod) => pod.namespace))];
  if (namespaces.length > 5) {
    return {
      ...base,
      services: [],
      serviceNote: `service check skipped (${namespaces.length} namespaces exceeds 5)`,
    };
  }
  const services = fetchJson(
    fetch,
    jsonArgs("services", [], withContext(scopeFlags(namespaces), options.context)),
  );
  if (services === "timeout") return serviceNote(base, "timed out");
  if (!services.ok) return serviceNote(base, services.error);
  const items = itemsOf(services.json, "Service");
  if (!items) return serviceNote(base, "not a Service object or list");
  const matched: {
    namespace: string;
    name: string;
    selector: Record<string, string>;
    matchedPods: number;
  }[] = [];
  for (const item of items) {
    const selector = stringMap(item.spec?.selector);
    const name = item.metadata?.name;
    const ns = item.metadata?.namespace || (namespaces.length === 1 ? namespaces[0] : "");
    if (!selector || !name || !ns) continue;
    const matchedPods = pods.filter(
      (pod) => pod.namespace === ns && labelsMatch(selector, pod.labels),
    ).length;
    if (matchedPods === 0) continue;
    matched.push({ namespace: ns, name, selector, matchedPods });
  }
  if (matched.length === 0) return base;
  let alive = pods;
  if (!options.reuse) {
    const serviceNamespaces = [...new Set(matched.map((item) => item.namespace))];
    const survivors = fetchJson(
      fetch,
      jsonArgs("pods", [], withContext(scopeFlags(serviceNamespaces), options.context)),
    );
    if (survivors === "timeout") return serviceNote(base, "timed out");
    if (!survivors.ok) return serviceNote(base, survivors.error);
    const survivorItems = itemsOf(survivors.json, "Pod");
    if (!survivorItems) return serviceNote(base, "not a Pod object or list");
    alive = survivorItems.flatMap((item) => {
      const pod = podRef(item, fallbackNs);
      return pod ? [pod] : [];
    });
  }
  const deleted = new Set(pods.map((pod) => `${pod.namespace}/${pod.name}`));
  return {
    ...base,
    services: matched.map((item) => ({
      namespace: item.namespace,
      name: item.name,
      matchedPods: item.matchedPods,
      readyPodsRemaining: alive.filter(
        (pod) =>
          pod.namespace === item.namespace &&
          !deleted.has(`${pod.namespace}/${pod.name}`) &&
          labelsMatch(item.selector, pod.labels) &&
          pod.ready,
      ).length,
    })),
  };
}

function carry(next: Blast, prev: Blast): Blast {
  const subject = subjects.get(prev);
  if (subject) subjects.set(next, subject);
  return next;
}

function podBudget(pods: PodRef[]): boolean {
  return (
    pods.length > 0 && pods.length <= 200 && new Set(pods.map((pod) => pod.namespace)).size <= 5
  );
}

function withPdbs(blast: Blast, pods: PodRef[], fetch: Fetch, asked: Asked): Blast {
  if (!podBudget(pods)) return blast;
  const namespaces = [...new Set(pods.map((pod) => pod.namespace))];
  const result = fetchJson(
    fetch,
    jsonArgs("pdb", [], withContext(scopeFlags(namespaces), asked.context)),
  );
  if (result === "timeout") return carry({ ...blast, pdbNote: "pdbs unknown (timed out)" }, blast);
  if (!result.ok)
    return carry({ ...blast, pdbNote: `pdbs unknown (${result.error || "failed"})` }, blast);
  const items = itemsOf(result.json, "PodDisruptionBudget");
  if (!items) {
    return carry(
      { ...blast, pdbNote: "pdbs unknown (not a PodDisruptionBudget object or list)" },
      blast,
    );
  }
  const pdbs: NonNullable<Blast["pdbs"]> = [];
  for (const item of items) {
    const selector = selectorOf(item);
    const name = item.metadata?.name;
    const ns = item.metadata?.namespace || (namespaces.length === 1 ? namespaces[0] : "");
    const allowed = item.status?.disruptionsAllowed;
    if (!selector || !name || !ns || typeof allowed !== "number") continue;
    const matched = pods.some((pod) => pod.namespace === ns && labelsMatch(selector, pod.labels));
    if (!matched) continue;
    pdbs.push({ namespace: ns, name, disruptionsAllowed: allowed });
  }
  return carry({ ...blast, pdbs }, blast);
}

function withHpas(blast: Blast, fetch: Fetch, asked: Asked): Blast {
  const controllers = blast.controllers ?? [];
  if (controllers.length === 0) return blast;
  const namespaces = [...new Set(controllers.map((item) => item.namespace))];
  if (namespaces.length > 5) return blast;
  const result = fetchJson(
    fetch,
    jsonArgs("hpa", [], withContext(scopeFlags(namespaces), asked.context)),
  );
  if (result === "timeout" || !result.ok) return blast;
  const items = itemsOf(result.json, "HorizontalPodAutoscaler");
  if (!items) return blast;
  const hpas: NonNullable<Blast["hpas"]> = [];
  for (const item of items) {
    const ref = item.spec?.scaleTargetRef;
    const name = item.metadata?.name;
    if (!ref?.name || !name) continue;
    const target = controllers.find(
      (controller) =>
        controller.name === ref.name && controller.kind === (ref.kind ?? "").toLowerCase(),
    );
    if (!target) continue;
    hpas.push({
      namespace: item.metadata?.namespace || target.namespace,
      name,
      minReplicas: item.spec?.minReplicas ?? 1,
      maxReplicas: item.spec?.maxReplicas ?? item.spec?.minReplicas ?? 1,
      target: `${target.kind}/${target.name}`,
    });
  }
  if (hpas.length === 0) return blast;
  return carry({ ...blast, hpas }, blast);
}

function withExtras(blast: Blast, pods: PodRef[], fetch: Fetch, asked: Asked): Blast {
  return withHpas(withPdbs(blast, pods, fetch, asked), fetch, asked);
}

function dryRunArgs(args: string[]): string[] {
  const next = [...args];
  if (!next.some((token) => token === "--dry-run" || token.startsWith("--dry-run="))) {
    next.push("--dry-run=server");
  }
  if (!next.includes("-o") && !next.some((token) => token.startsWith("--output"))) {
    next.push("-o", "json");
  }
  if (
    !next.some((token) => token === "--request-timeout" || token.startsWith("--request-timeout="))
  ) {
    next.push("--request-timeout=4s");
  }
  return next;
}

function countOf(json: unknown): number | null {
  if (!json || typeof json !== "object") return null;
  const doc = json as { items?: unknown; metadata?: { name?: string } };
  if (Array.isArray(doc.items)) return doc.items.length;
  if (doc.metadata?.name) return 1;
  return null;
}

function withDryRun(blast: Blast, args: string[], fetch: Fetch): Blast {
  if (args[0] !== "delete" || blast.status === "skipped") return blast;
  const result = fetchJson(fetch, dryRunArgs(args));
  if (result === "timeout") {
    return carry({ ...blast, dryRun: { status: "unknown", reason: "timed out" } }, blast);
  }
  if (!result.ok) {
    return carry(
      { ...blast, dryRun: { status: "unknown", reason: result.error || "failed" } },
      blast,
    );
  }
  const count = countOf(result.json);
  if (count === null) {
    return carry({ ...blast, dryRun: { status: "unknown", reason: "not a delete result" } }, blast);
  }
  return carry({ ...blast, dryRun: { status: "ok", count } }, blast);
}

function ownerOf(item: KubeObject, namespace: string, name: string): Blast["owners"] {
  const owner =
    item.metadata?.ownerReferences?.find((ref) => ref.controller) ??
    item.metadata?.ownerReferences?.[0];
  if (!owner?.kind || !owner.name) return [];
  return [{ namespace, pod: name, kind: owner.kind.toLowerCase(), name: owner.name }];
}

function fromItems(
  items: KubeObject[],
  resource: Resource | undefined,
  asked: Asked,
  fetch: Fetch,
  fallbackNs: string,
  reuse: boolean,
): Blast {
  const owners: NonNullable<Blast["owners"]> = [];
  const objects = items.flatMap((item) => {
    const name = item.metadata?.name;
    if (!name) return [];
    const kind = resource?.singular ?? (item.kind ?? "object").toLowerCase();
    const namespace = item.metadata?.namespace || fallbackNs;
    if (kind === "pod") owners.push(...(ownerOf(item, namespace, name) ?? []));
    return [{ kind, namespace, name }];
  });
  const singular =
    resource?.singular ??
    (objects.every((object) => object.kind === "pod") ? "pod" : (objects[0]?.kind ?? "object"));
  const partial = { kind: singular, namespace: fallbackNs };
  if (items.length > 0 && items.every((item) => CONTROLLERS.has(item.kind ?? ""))) {
    return expandControllers(items, objects, fetch, asked, fallbackNs, partial);
  }
  const base = pack(objects, partial, owners.length ? { owners } : {});
  if (singular !== "pod" && !(!resource && objects.some((object) => object.kind === "pod"))) {
    return base;
  }
  const pods = items.flatMap((item) => {
    if (item.kind !== "Pod" && resource?.singular !== "pod") return [];
    if (resource?.singular === "pod" || item.kind === "Pod") {
      const pod = podRef(item, fallbackNs);
      return pod ? [pod] : [];
    }
    return [];
  });
  const withServices = attachServices(base, pods, fetch, fallbackNs, {
    context: asked.context,
    reuse,
  });
  if (withServices.status !== "ok") return withServices;
  return withExtras(remember(withServices, subjects.get(base)!), pods, fetch, asked);
}

function isKustomize(args: string[]): boolean {
  return args.some((token) => {
    const name = token.split("=", 1)[0];
    return name === "-k" || name === "--kustomize";
  });
}

function manifestFiles(args: string[]): string[] {
  const files: string[] = [];
  for (let i = 0; i < args.length; i += 1) {
    const token = args[i]!;
    if (token === "-f" || token === "--filename") {
      if (args[i + 1]) files.push(args[(i += 1)]!);
    } else if (token.startsWith("--filename=")) files.push(token.slice("--filename=".length));
    else if (token.startsWith("-f=")) files.push(token.slice("-f=".length));
  }
  return files;
}

function isDaemon(item: KubeObject): boolean {
  return item.metadata?.ownerReferences?.some((owner) => owner.kind === "DaemonSet") === true;
}

function blastFiles(files: string[], plan: Plan, asked: Asked, fetch: Fetch): Blast {
  const fallbackNs = asked.namespace || plan.namespace;
  const groups = new Map<string, ManifestRef[]>();
  for (const file of files) {
    const read = manifestRefs(file);
    if (!read.ok) return unknown(`manifest: ${read.error}`);
    for (const ref of read.refs) {
      const namespace = ref.namespace || fallbackNs;
      const key = `${ref.kind.toLowerCase()}\0${namespace}`;
      const list = groups.get(key) ?? [];
      list.push({ ...ref, namespace });
      groups.set(key, list);
    }
  }
  if (groups.size === 0) return skipped("no name or selector");
  const items: KubeObject[] = [];
  for (const group of groups.values()) {
    const token = group[0]!.kind.toLowerCase();
    const resource = resolveResource(token);
    const namespace = group[0]!.namespace || fallbackNs;
    const result = fetchJson(
      fetch,
      jsonArgs(
        resource?.singular ?? token,
        group.map((ref) => ref.name),
        withContext(["-n", namespace], asked.context),
      ),
    );
    if (result === "timeout")
      return items.length
        ? fromItems(items, undefined, asked, fetch, fallbackNs, false)
        : unknown("timed out");
    if (!result.ok) {
      return items.length
        ? fromItems(items, undefined, asked, fetch, fallbackNs, false)
        : unknown(`kubectl get ${token}: ${result.error || "failed"}`);
    }
    const parsed = itemsOf(result.json, resource?.apiKind);
    if (!parsed) {
      return items.length
        ? fromItems(items, undefined, asked, fetch, fallbackNs, false)
        : unknown(`kubectl get ${token}: not a ${resource?.apiKind ?? "object"} object or list`);
    }
    items.push(...parsed);
  }
  return fromItems(items, undefined, asked, fetch, fallbackNs, false);
}

function blastDrain(args: string[], plan: Plan, fetch: Fetch, asked: Asked): Blast {
  const node = positionalWords(args).words[1];
  if (!node) return skipped("no name or selector");
  const fallbackNs = asked.namespace || plan.namespace;
  const scope = asked.namespace && !asked.allNamespaces ? ["-n", asked.namespace] : ["-A"];
  const result = fetchJson(
    fetch,
    jsonArgs(
      "pods",
      ["--field-selector", `spec.nodeName=${node}`],
      withContext(scope, asked.context),
    ),
  );
  if (result === "timeout") return unknown("timed out");
  if (!result.ok) return unknown(`kubectl get pods: ${result.error || "failed"}`);
  const parsed = itemsOf(result.json, "Pod");
  if (!parsed) return unknown("kubectl get pods: not a Pod object or list");
  return fromItems(
    parsed.filter((item) => !isDaemon(item)),
    resolveResource("pod"),
    asked,
    fetch,
    fallbackNs,
    false,
  );
}

export function blastRadius(
  plan: Plan,
  args: string[],
  getJson: GetJson,
  clock?: BlastClock,
): Blast {
  const now = clock?.now ?? Date.now;
  const started = now();
  const fetch = { getJson, now, started };
  const asked = askedOf(args);
  const scaleZero = plan.verb === "scale" && flagValue(args, ["--replicas"]) === "0";
  if (plan.verb === "drain") return blastDrain(args, plan, fetch, asked);
  if (plan.verb !== "delete" && !scaleZero) return skipped("not a delete");
  if (isKustomize(args)) return skipped("kustomize delete (-k) is not expanded");
  const files = manifestFiles(args);
  if (files.some((file) => /^https?:\/\//i.test(file))) {
    return skipped("remote manifest URLs are not expanded");
  }
  if (files.length > 0) return withDryRun(blastFiles(files, plan, asked, fetch), args, fetch);
  const target = targetOf(args);
  if (target === "mixed") return skipped("mixed resource kinds");
  if (!target) return skipped("no name or selector");
  const middle = target.selector ? ["-l", target.selector] : target.all ? [] : target.names;
  const result = fetchJson(fetch, jsonArgs(target.token, middle, askedFlags(asked)));
  if (result === "timeout") return unknown("timed out");
  if (!result.ok) return unknown(`kubectl get ${target.token}: ${result.error || "failed"}`);
  const items = itemsOf(result.json, target.resource?.apiKind);
  if (!items) {
    const kind = target.resource?.apiKind ?? "object";
    return unknown(`kubectl get ${target.token}: not a ${kind} object or list`);
  }
  const finished = fromItems(
    items,
    target.resource,
    asked,
    fetch,
    asked.namespace || plan.namespace,
    target.all &&
      (target.resource?.singular === "pod" || target.token === "pods" || target.token === "pod"),
  );
  return withDryRun(finished, args, fetch);
}
