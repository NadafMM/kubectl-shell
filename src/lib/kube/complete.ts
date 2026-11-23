import { shellCommandCandidates, VIEW_ARGS } from "./shell-meta";
import { VERBS, type Cluster } from "./model";
import { candidatesFor, flagItems, inScope, manifestCandidates, slashNames } from "./grammar";

export { manifestCandidates };
import { locate, positionalWords, scopeOf, takesOf, verbOf, type Scope } from "./parse";

export type Candidate = {
  value: string;
  detail: string;
  kind: string;
  weight?: number;
  aliases?: string[];
};

export type Completion = { start: number; end: number; candidates: Candidate[] };

const LIMIT = 24;

function rank(prefix: string, items: Candidate[]): Candidate[] {
  const p = prefix.toLowerCase();
  const scored: { item: Candidate; score: number }[] = [];
  for (const item of items) {
    const value = item.value.toLowerCase();
    const aliases = (item.aliases ?? []).map((alias) => alias.toLowerCase());
    let score: number;
    if (!p) score = item.weight ?? 1;
    else if (value === p) score = 1200 + (item.weight ?? 0) / 1000;
    else if (aliases.includes(p)) score = 1100;
    else if (value.startsWith(p))
      score = 800 - Math.min(value.length, 180) + (item.weight ?? 0) / 100;
    else if (aliases.some((alias) => alias.startsWith(p))) score = 760 + (item.weight ?? 0) / 100;
    else if (p.length >= 2 && (value.includes(p) || aliases.some((alias) => alias.includes(p))))
      score = 420;
    else continue;
    scored.push({ item, score });
  }
  scored.sort((a, b) => b.score - a.score || a.item.value.localeCompare(b.item.value));
  const unique: Candidate[] = [];
  const seen = new Set<string>();
  for (const row of scored) {
    if (seen.has(row.item.value)) continue;
    seen.add(row.item.value);
    unique.push(row.item);
    if (unique.length >= LIMIT) break;
  }
  if (unique.length === 0 && p.length >= 3) {
    const near = items
      .map((item) => ({ item, dist: distance(p, item.value.toLowerCase()) }))
      .filter((row) => row.dist <= 2)
      .sort((a, b) => a.dist - b.dist || a.item.value.localeCompare(b.item.value))
      .slice(0, 6)
      .map((row) => row.item);
    return near;
  }
  return unique;
}

function distance(a: string, b: string): number {
  if (Math.abs(a.length - b.length) > 2) return 9;
  const dp = Array.from({ length: a.length + 1 }, () => new Array<number>(b.length + 1).fill(0));
  for (let i = 0; i <= a.length; i += 1) dp[i]![0] = i;
  for (let j = 0; j <= b.length; j += 1) dp[0]![j] = j;
  for (let i = 1; i <= a.length; i += 1) {
    for (let j = 1; j <= b.length; j += 1) {
      const cost = a[i - 1] === b[j - 1] ? 0 : 1;
      dp[i]![j] = Math.min(dp[i - 1]![j]! + 1, dp[i]![j - 1]! + 1, dp[i - 1]![j - 1]! + cost);
    }
  }
  return dp[a.length]![b.length]!;
}

function valueItems(
  kind: string,
  args: string[],
  cluster: Cluster,
  scope: Scope,
  prefix = "",
): Candidate[] {
  switch (kind) {
    case "namespace":
      return cluster.namespaces.map((item) => ({
        value: item.name,
        detail: "Namespace",
        kind: "value",
        weight: 40,
      }));
    case "context":
      return cluster.contexts.map((item) => ({
        value: item.name,
        detail: item.cluster,
        kind: "context",
        weight: 40,
      }));
    case "output":
      return ["wide", "yaml", "json", "name", "jsonpath", "custom-columns"].map((value, index) => ({
        value,
        detail: "Output format",
        kind: "value",
        weight: 80 - index,
      }));
    case "container": {
      const pods = referencedPods(args, scope, cluster);
      const names = new Set<string>();
      for (const item of pods)
        for (const container of item.containers ?? []) names.add(container.name);
      if (names.size === 0) {
        for (const item of cluster.objects) {
          if (item.kind === "pod" && inScope(item.ns, scope)) {
            for (const container of item.containers ?? []) names.add(container.name);
          }
        }
      }
      return [...names].map((value) => ({ value, detail: "Container", kind: "value", weight: 40 }));
    }
    case "file":
    case "dir":
      return manifestCandidates(prefix);
    case "selector":
      return selectorItems(scope, cluster);
    case "replicas":
      return ["1", "2", "3", "4", "5"].map((value) => ({
        value,
        detail: "Replica count",
        kind: "value",
        weight: 40,
      }));
    case "tail":
      return ["20", "50", "100", "200", "500"].map((value, index) => ({
        value,
        detail: "Log lines",
        kind: "value",
        weight: 50 - index,
      }));
    case "dryrun":
      return [
        { value: "client", detail: "Validate locally", kind: "value", weight: 60 },
        { value: "server", detail: "Validate on the API server", kind: "value", weight: 50 },
        { value: "none", detail: "Persist the change", kind: "value", weight: 20 },
      ];
    case "image":
      return [];
    case "port":
      return ["80", "443", "8080", "3000", "9090"].map((value) => ({
        value,
        detail: "Port",
        kind: "value",
        weight: 30,
      }));
    case "for":
      return ["condition=ready", "condition=available", "condition=complete", "delete"].map(
        (value, index) => ({
          value,
          detail: "Wait condition",
          kind: "value",
          weight: 40 - index,
        }),
      );
    case "sort":
      return [".metadata.name", ".metadata.creationTimestamp", ".status.phase"].map((value) => ({
        value,
        detail: "JSON path",
        kind: "value",
        weight: 20,
      }));
    case "since":
      return ["5s", "15m", "1h", "24h"].map((value) => ({
        value,
        detail: "Duration",
        kind: "value",
        weight: 20,
      }));
    case "address":
      return ["127.0.0.1", "localhost", "0.0.0.0"].map((value) => ({
        value,
        detail: "Bind address",
        kind: "value",
        weight: 20,
      }));
    case "revision":
      return ["1", "2"].map((value) => ({ value, detail: "Revision", kind: "value", weight: 20 }));
    case "svctype":
      return ["ClusterIP", "NodePort", "LoadBalancer"].map((value) => ({
        value,
        detail: "Service type",
        kind: "value",
        weight: 30,
      }));
    case "restart":
      return ["Always", "OnFailure", "Never"].map((value) => ({
        value,
        detail: "Restart policy",
        kind: "value",
        weight: 30,
      }));
    case "literal":
      return [
        { value: "LOG_LEVEL=info", detail: "Example pair", kind: "value", weight: 20 },
        { value: "color=blue", detail: "Example pair", kind: "value", weight: 16 },
      ];
    case "timeout":
      return ["30s", "60s", "5m"].map((value) => ({
        value,
        detail: "Duration",
        kind: "value",
        weight: 20,
      }));
    case "grace":
      return ["0", "30"].map((value) => ({ value, detail: "Seconds", kind: "value", weight: 20 }));
    case "level":
      return ["0", "2", "4", "6", "8"].map((value) => ({
        value,
        detail: "Verbosity",
        kind: "value",
        weight: 10,
      }));
    case "field":
      return [
        { value: "status.phase=Running", detail: "Running pods", kind: "value", weight: 30 },
        { value: "status.phase=Pending", detail: "Pending pods", kind: "value", weight: 20 },
      ];
    case "user":
      return ["system:admin", "alice"].map((value) => ({
        value,
        detail: "User",
        kind: "value",
        weight: 10,
      }));
    case "group":
      return ["apps", "batch", ""]
        .filter(Boolean)
        .map((value) => ({ value, detail: "API group", kind: "value", weight: 10 }));
    case "patchtype":
      return ["strategic", "merge", "json"].map((value) => ({
        value,
        detail: "Patch type",
        kind: "value",
        weight: 20,
      }));
    case "name":
      return [];
    default:
      return [];
  }
}

function selectorItems(scope: Scope, cluster: Cluster): Candidate[] {
  const seen = new Set<string>();
  const items: Candidate[] = [];
  for (const object of cluster.objects) {
    if (!inScope(object.ns, scope)) continue;
    for (const [key, value] of Object.entries(object.labels)) {
      const pair = `${key}=${value}`;
      if (seen.has(pair)) continue;
      seen.add(pair);
      items.push({ value: pair, detail: "Label selector", kind: "value", weight: 30 });
    }
  }
  return items;
}

function referencedPods(args: string[], scope: Scope, cluster: Cluster) {
  const words = positionalWords(args).words;
  const names = new Set(words);
  return cluster.objects.filter(
    (object) => object.kind === "pod" && inScope(object.ns, scope) && names.has(object.name),
  );
}

function shellWords(): Candidate[] {
  return [
    { value: "bash", detail: "Interactive shell", kind: "command", weight: 100 },
    { value: "sh", detail: "POSIX shell", kind: "command", weight: 80 },
    { value: "ls", detail: "List files", kind: "command", weight: 70 },
    { value: "env", detail: "Print environment", kind: "command", weight: 64 },
    { value: "printenv", detail: "Print environment", kind: "command", weight: 40 },
    { value: "pwd", detail: "Working directory", kind: "command", weight: 50 },
    { value: "whoami", detail: "Current user", kind: "command", weight: 46 },
    { value: "ps", detail: "Process list", kind: "command", weight: 30 },
    { value: "cat", detail: "Print a file", kind: "command", weight: 28 },
  ];
}

export function complete(line: string, cursor: number, cluster: Cluster): Completion {
  const loc = locate(line, cursor);
  const prior = loc.tokens.slice(0, loc.index).map((token) => token.text);
  const prefix = line.slice(loc.start, cursor);
  const base = { start: loc.start, end: loc.end };

  if (prior.length === 0) {
    return {
      ...base,
      candidates: rank(prefix, [
        {
          value: "kubectl",
          detail: "Kubernetes CLI",
          kind: "command",
          weight: 100,
          aliases: ["k"],
        },
        ...shellCommandCandidates(prefix),
        { value: "help", detail: "How Tab completion works here", kind: "command", weight: 10 },
      ]),
    };
  }

  const bin = prior[0];
  if (bin === "view" || bin === "ui") {
    const tail = prior.slice(1);
    if (tail.length === 0) {
      return { ...base, candidates: rank(prefix, [...VIEW_ARGS]) };
    }
  }

  if (bin !== "kubectl" && bin !== "k" && bin !== "help") {
    const shell = shellCommandCandidates(bin ?? "");
    if (shell.length > 0) {
      return { ...base, candidates: rank(prefix, shell) };
    }
    return {
      ...base,
      candidates: rank(prefix, [
        ...shellCommandCandidates(bin ?? ""),
        { value: "kubectl", detail: "Commands start with kubectl", kind: "command", weight: 60 },
      ]),
    };
  }

  if (bin === "help") {
    return {
      ...base,
      candidates: rank(
        prefix,
        VERBS.map((item) => ({
          value: item.name,
          detail: item.detail,
          kind: "verb",
          weight: item.weight,
        })),
      ),
    };
  }

  const args = prior.slice(1);
  const scope = scopeOf(["kubectl", ...args], cluster);

  const equals = /^(--?[A-Za-z0-9-]+)=(.*)$/.exec(prefix);
  if (equals) {
    const flag = equals[1] ?? "";
    const kind = takesOf(flag, verbOf(args));
    const values = kind ? valueItems(kind, args, cluster, scope, equals[2] ?? "") : [];
    return {
      ...base,
      candidates: rank(
        prefix,
        values.map((item) => ({ ...item, value: `${flag}=${item.value}` })),
      ),
    };
  }

  if (prefix.startsWith("-")) {
    return { ...base, candidates: rank(prefix, flagItems(verbOf(args))) };
  }

  const previous = args[args.length - 1];
  const verb = verbOf(args);
  if (previous && takesOf(previous, verb) && !previous.includes("=")) {
    const kind = takesOf(previous, verb)!;
    return {
      ...base,
      candidates: rank(prefix, valueItems(kind, args.slice(0, -1), cluster, scope, prefix)),
    };
  }

  const parsed = positionalWords(args);
  if (parsed.afterDashDash) return { ...base, candidates: rank(prefix, shellWords()) };

  const words = parsed.words;
  if (words.length === 0) {
    return {
      ...base,
      candidates: rank(
        prefix,
        VERBS.map((item) => ({
          value: item.name,
          detail: item.detail,
          kind: "verb",
          weight: item.weight,
        })),
      ),
    };
  }

  const rest = words.slice(1);

  if (prefix.includes("/") && rest.length === 0) {
    const slash = prefix.indexOf("/");
    const typeToken = prefix.slice(0, slash);
    const namePrefix = prefix.slice(slash + 1);
    return { ...base, candidates: rank(prefix, slashNames(typeToken, namePrefix, scope, cluster)) };
  }

  return {
    ...base,
    candidates: rank(prefix, candidatesFor(verb ?? "", rest, args, scope, cluster, prefix)),
  };
}
