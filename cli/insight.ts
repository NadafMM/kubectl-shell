import type { Cluster } from "../src/lib/kube/model";
import { namespaceOf, resolveResource } from "../src/lib/kube/model";
import { flagValue, positionalWords, scopeOf, tokenize, unquote } from "../src/lib/kube/parse";
import { sensitiveStdoutKind } from "./reveal-output";

export type Plan = {
  command: string;
  verb: string;
  context: string;
  namespace: string;
  allNamespaces: boolean;
  targets: string[];
  mutating: boolean;
  risk: "read" | "low" | "high";
  summary: string;
};

export type Advice = {
  why: string;
  commands: string[];
};

function argsOf(engineLine: string): string[] {
  const tokens = tokenize(engineLine).map((token) => unquote(token.text));
  if (tokens[0] === "kubectl" || tokens[0] === "k") return tokens.slice(1);
  return tokens;
}

function has(tokens: string[], name: string): boolean {
  return tokens.some((token) => token === name || token.startsWith(`${name}=`));
}

function dryRun(tokens: string[]): boolean {
  return tokens.some((token) => token === "--dry-run" || token.startsWith("--dry-run="));
}

function riskOf(verb: string, sub: string, tokens: string[]): "read" | "low" | "high" {
  if (verb === "delete" || verb === "drain" || verb === "cordon" || verb === "taint") return "high";
  if (verb === "cp" || verb === "attach" || verb === "exec" || verb === "debug") return "high";
  if (verb === "port-forward" || verb === "proxy") return "high";
  if (verb === "certificate" && (sub === "approve" || sub === "deny")) return "high";
  if (verb === "config" && sub === "set-credentials") return "high";
  if (!dryRun(tokens) && (verb === "run" || verb === "expose")) return "high";
  if (verb === "scale" && flagValue(tokens, ["--replicas"]) === "0") return "high";
  if (verb === "rollout" && (sub === "undo" || sub === "restart")) return "high";
  if (verb === "replace" && has(tokens, "--force")) return "high";
  if (
    verb === "config" &&
    (sub === "delete-context" || sub === "delete-cluster" || sub === "delete-user")
  )
    return "high";
  if (!dryRun(tokens) && ["apply", "create", "replace", "patch"].includes(verb)) return "high";
  if (
    [
      "scale",
      "edit",
      "label",
      "annotate",
      "set",
      "rollout",
      "run",
      "expose",
      "autoscale",
      "uncordon",
      "apply",
      "create",
      "replace",
      "patch",
    ].includes(verb)
  ) {
    return "low";
  }
  if (verb === "config" && ["use-context", "set-context", "set-cluster", "set"].includes(sub))
    return "low";
  return "read";
}

function summaryFor(
  plan: Pick<Plan, "verb" | "targets" | "namespace" | "allNamespaces" | "risk">,
  sub: string,
  tokens: string[],
): string {
  const where = plan.allNamespaces ? "every namespace" : plan.namespace;
  const what = plan.targets.join(", ") || plan.verb;
  const reveal = sensitiveStdoutKind(tokens);
  if (reveal === "secret") {
    return `Reads ${what} in ${where}. kubectl may print Secret data on stdout; kube does not redact it unless KUBE_SHELL_REDACT_STDOUT=1 and stdout is not a TTY.`;
  }
  if (reveal === "configmap") {
    return `Reads ${what} in ${where}. kubectl may print ConfigMap data on stdout; kube does not redact it unless KUBE_SHELL_REDACT_STDOUT=1 and stdout is not a TTY.`;
  }
  if (reveal === "kubeconfig") {
    return `Prints the kubeconfig including client credentials. kube does not redact it unless KUBE_SHELL_REDACT_STDOUT=1 and stdout is not a TTY.`;
  }
  if (plan.verb === "port-forward") return `Opens a local port to ${what} in ${where}.`;
  if (plan.verb === "proxy") return `Starts a local API proxy using your kubeconfig credentials.`;
  if (plan.verb === "delete")
    return `Deletes ${what} in ${where}. A controller may recreate a pod.`;
  if (plan.verb === "drain") return `Drains ${what}. Pods are evicted off that node.`;
  if (plan.verb === "scale" && plan.risk === "high")
    return `Scales ${what} to zero replicas in ${where}.`;
  if (plan.verb === "rollout" && sub === "undo")
    return `Rolls ${what} back to the previous revision.`;
  if (plan.verb === "rollout" && sub === "restart") return `Restarts ${what} in ${where}.`;
  if (plan.verb === "cp") return `Copies files to or from ${what}.`;
  if (plan.verb === "attach") return `Attaches to a process in ${what}.`;
  if (plan.verb === "exec")
    return `Runs a process inside ${what}. The container filesystem can change.`;
  if (plan.verb === "debug") return `Starts a debug container for ${what}.`;
  if (plan.risk === "read") return `Reads ${what} in ${where}. Nothing in the cluster changes.`;
  if (plan.verb === "patch") return `Patches ${what} in ${where}.`;
  if (plan.verb === "apply" || plan.verb === "create" || plan.verb === "replace")
    return `Writes ${what} in ${where}.`;
  return `Changes ${what} in ${where}.`;
}

/** What a kubectl line will touch. Safe for an agent to read before it runs anything. */
export function planCommand(engineLine: string, cluster: Cluster): Plan {
  const args = argsOf(engineLine);
  const scope = scopeOf(args, cluster);
  const parsed = positionalWords(args);
  const verb = parsed.words[0] ?? "";
  const sub = parsed.words[1] ?? "";
  const resource = resolveResource(sub);
  const names = resource
    ? parsed.words.slice(2)
    : verb === "drain" || verb === "cordon" || verb === "uncordon"
      ? parsed.words.slice(1)
      : [];
  const kind = resource?.singular || (verb === "drain" ? "node" : "");
  const targets =
    names.length > 0
      ? names.map((name) => (kind ? `${kind}/${name}` : name))
      : sub
        ? [has(args, "--all") ? `all ${sub}` : sub]
        : [];
  const risk = riskOf(verb, resource ? (parsed.words[2] ?? "") : sub, args);
  const plan: Plan = {
    command: ["kubectl", ...args].join(" "),
    verb,
    context: scope.context,
    namespace: scope.ns || namespaceOf(cluster),
    allNamespaces: scope.all,
    targets,
    mutating: risk !== "read",
    risk,
    summary: "",
  };
  plan.summary = summaryFor(
    plan,
    verb === "rollout" ? sub : resource ? (parsed.words[1] ?? "") : sub,
    args,
  );
  return plan;
}

export function formatPlan(plan: Plan): string {
  const where = plan.allNamespaces ? "all namespaces" : `namespace ${plan.namespace}`;
  return `${plan.command}\ncontext ${plan.context} · ${where} · risk ${plan.risk}\n${plan.summary}`;
}

type Hit = { name: string; kind: string; ns: string; status: string };

function unhealthy(cluster: Cluster): Hit | undefined {
  const hit = cluster.objects.find((item) =>
    /CrashLoop|ImagePull|ErrImage|Error|Pending|BackOff|OOM/i.test(item.status ?? ""),
  );
  if (!hit) return undefined;
  return { name: hit.name, kind: hit.kind, ns: hit.ns, status: hit.status ?? hit.kind };
}

function mentioned(question: string, cluster: Cluster): Hit | undefined {
  const q = question.toLowerCase();
  const ranked = [...cluster.objects].sort((a, b) => b.name.length - a.name.length);
  for (const item of ranked) {
    if (item.name.length >= 3 && q.includes(item.name.toLowerCase())) {
      return { name: item.name, kind: item.kind, ns: item.ns, status: item.status ?? item.kind };
    }
  }
  return undefined;
}

function nsSuffix(hit: Hit | undefined, cluster: Cluster): string {
  if (!hit) return "";
  if (hit.ns && hit.ns !== namespaceOf(cluster)) return ` -n ${hit.ns}`;
  return "";
}

function podish(hit: Hit | undefined): string {
  if (!hit) return "pod";
  if (hit.kind === "pod") return "pod";
  return hit.kind;
}

/** Turn a developer question into kubectl lines. No network and no model call. */
export function advise(question: string, cluster: Cluster): Advice {
  const q = question.trim().toLowerCase();
  const named = mentioned(question, cluster);
  const sick = named ?? unhealthy(cluster);
  const ns = nsSuffix(sick, cluster);
  const name = sick?.name ?? "<name>";
  const kind = podish(sick);
  const whyName = sick
    ? `${sick.name} is ${sick.status} in ${sick.ns}.`
    : "No matching object in this context.";

  if (!q) {
    return {
      why: "Ask about a pod, a crash, a pending schedule, or the current context.",
      commands: ["get pods", "get events"],
    };
  }
  if (/context|where am i|which cluster/.test(q)) {
    return {
      why: `Current context is ${cluster.current}, namespace ${namespaceOf(cluster)}.`,
      commands: ["config current-context", "config get-contexts"],
    };
  }
  if (/namespace|which ns|what ns/.test(q)) {
    return {
      why: `Namespace on ${cluster.current} is ${namespaceOf(cluster)}.`,
      commands: ["get ns", "config view --minify"],
    };
  }
  if (/crash|backoff|restart|oom|not starting|failing/.test(q)) {
    return {
      why: whyName,
      commands: [`describe ${kind} ${name}${ns}`, `logs ${name} --tail=80${ns}`, `get events${ns}`],
    };
  }
  if (/pending|unschedul|cannot schedule|can't schedule|insufficient/.test(q)) {
    return {
      why: whyName,
      commands: [`describe ${kind} ${name}${ns}`, "get nodes", `get events${ns}`],
    };
  }
  if (/imagepull|errimage|pull back|registry/.test(q)) {
    return { why: whyName, commands: [`describe ${kind} ${name}${ns}`, `get events${ns}`] };
  }
  if (/not ready|unhealthy|readiness|liveness|probe/.test(q)) {
    return { why: whyName, commands: [`describe ${kind} ${name}${ns}`, `get pods${ns}`] };
  }
  if (/\blogs?\b/.test(q)) {
    return { why: whyName, commands: [`logs ${name} --tail=100${ns}`] };
  }
  if (/event/.test(q)) {
    return {
      why: "Recent events explain most scheduling and probe failures.",
      commands: [`get events${ns}`],
    };
  }
  if (/node/.test(q)) {
    return {
      why: "Node pressure shows up as Pending pods and evictions.",
      commands: ["get nodes", "describe nodes"],
    };
  }
  if (/\btop\b|cpu|memory hog|using memory/.test(q)) {
    return {
      why: "Metrics need the metrics-server. If top fails, describe the pod instead.",
      commands: [`top pods${ns}`, "top nodes"],
    };
  }
  if (/endpoint|service .*down|svc .*down|can.?t connect/.test(q)) {
    return {
      why: "A service with no endpoints has no ready pods behind it.",
      commands: [`get endpoints${ns}`, `get pods${ns}`],
    };
  }
  if (/port-?forward|forward/.test(q)) {
    return {
      why: `Forward a local port to ${name}.`,
      commands: [`port-forward ${kind}/${name} 8080:80${ns}`],
    };
  }
  if (/what('?s| is) running|all pods|list pods|show pods/.test(q)) {
    return { why: whyName, commands: [`get pods${ns}`, `get deploy,svc${ns}`] };
  }
  return {
    why: whyName,
    commands: [`get pods${ns}`, `describe ${kind} ${name}${ns}`, `get events${ns}`],
  };
}

export function formatAdvice(advice: Advice): string {
  return [advice.why, "", ...advice.commands.map((line) => `kubectl ${line}`)].join("\n");
}
