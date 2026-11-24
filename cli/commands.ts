import { metaOf } from "../src/lib/kube/shell-meta";
import { namespaceOf, type Cluster } from "../src/lib/kube/model";
import { completionScript } from "./completion";
import { toEngine } from "./engine";
import { BLAST_BUDGET_MS, blastRadius, formatBlast, type GetJson } from "./blast";
import { advise, formatAdvice, formatPlan, planCommand, type Plan } from "./insight";
import {
  applyLiveContext,
  invalidateLive,
  kubectlArgs,
  liveCluster,
  probeLive,
  runJson,
  runLive,
} from "./live";
import {
  appendAudit,
  auditEnabled,
  newAuditId,
  redactArgv,
  tailAudit,
  type AuditApprover,
  type AuditDecision,
} from "./audit";
import { consumePlan, mintPlan } from "./plans";
import { ruleFor, type Policy } from "./policy";
import { boardFrom, openView } from "./view";

function wantsBlast(plan: Plan): boolean {
  return (
    plan.verb === "delete" ||
    plan.verb === "drain" ||
    (plan.verb === "scale" && plan.risk === "high")
  );
}

function commandArgs(engineLine: string, cluster: Cluster): string[] {
  const args = [...kubectlArgs(engineLine)];
  const hasNs = args.some(
    (token) =>
      token === "-n" ||
      token === "--namespace" ||
      token.startsWith("--namespace=") ||
      token.startsWith("-n="),
  );
  const hasCtx = args.some((token) => token === "--context" || token.startsWith("--context="));
  if (!hasNs) args.push("-n", namespaceOf(cluster) || "default");
  if (!hasCtx && cluster.current) args.push("--context", cluster.current);
  return args;
}

function liveBlast(plan: Plan, engineLine: string, cluster: Cluster) {
  const started = Date.now();
  const now = () => Date.now();
  const getJson: GetJson = (args) => {
    const left = BLAST_BUDGET_MS - (now() - started);
    if (left <= 0) return { ok: false, json: null, error: "timed out" };
    return runJson(args, left);
  };
  return blastRadius(plan, commandArgs(engineLine, cluster), getJson, { now });
}

function useNamespace(cluster: Cluster, namespace: string): Cluster {
  return {
    ...cluster,
    contexts: cluster.contexts.map((item) =>
      item.name === cluster.current ? { ...item, namespace } : item,
    ),
  };
}

function useContext(cluster: Cluster, name: string): Cluster | null {
  if (!cluster.contexts.some((item) => item.name === name)) return null;
  return { ...cluster, current: name };
}

export type LineResult = { output: string; ok: boolean; cluster: Cluster; exit?: boolean };

export type LineHost = {
  yes: boolean;
  help: string;
  version: string;
  confirm: (prompt: string) => boolean;
  agent?: boolean;
  planId?: string;
  confirmContext?: string;
  policy?: Policy | null;
};

export function runUserLine(
  line: string,
  cluster: Cluster,
  host: LineHost,
  options: { inheritStdin?: boolean } = {},
): LineResult {
  let trimmed = line.trim();
  const meta = metaOf(trimmed);
  if (meta === "exit" || meta === "quit") return { output: "", ok: true, cluster, exit: true };
  if (meta === "reset") {
    invalidateLive();
    const next = liveCluster("kubectl") ?? cluster;
    return { output: `refreshed names from ${next.current}`, ok: true, cluster: next };
  }
  if (meta === "sample") {
    return {
      output: "sample mode was removed. kube only runs against a reachable cluster.",
      ok: false,
      cluster,
    };
  }
  if (meta === "live") {
    invalidateLive();
    if (!probeLive())
      return {
        output: "No live cluster. Check kubectl and the current context.",
        ok: false,
        cluster,
      };
    const next = liveCluster("kubectl") ?? cluster;
    return { output: `live cluster ${next.current}`, ok: true, cluster: next };
  }
  if (meta === "clear") return { output: "\x1bc", ok: true, cluster };
  if (meta === "help" || meta === "--help" || meta === "-h")
    return { output: host.help, ok: true, cluster };
  if (meta === "--version" || meta === "-v") return { output: host.version, ok: true, cluster };
  if (meta === "completion") {
    const shell = trimmed.split(/\s+/)[1] ?? "";
    const script = completionScript(shell);
    if (!script) return { output: "usage: kube completion bash|zsh|fish", ok: false, cluster };
    return { output: script, ok: true, cluster };
  }
  if (meta === "audit") {
    const parts = trimmed.split(/\s+/);
    if (parts[1] && parts[1] !== "tail") {
      return { output: "usage: kube audit tail [-n 20]", ok: false, cluster };
    }
    let count = 20;
    const flag = parts.indexOf("-n");
    if (flag >= 0) count = Number(parts[flag + 1]);
    const lines = tailAudit(count);
    return { output: lines.join("\n"), ok: true, cluster };
  }
  if (meta === "ns" || meta === "ctx") {
    const name = trimmed.split(/\s+/).slice(1).join(" ").trim();
    if (!name) return { output: `usage: ${meta} <name>`, ok: false, cluster };
    if (!cluster.current) return { output: "no current context", ok: false, cluster };
    if (meta === "ns") {
      return { output: `namespace ${name}`, ok: true, cluster: useNamespace(cluster, name) };
    }
    const next = useContext(cluster, name);
    if (!next) return { output: `no context ${name}`, ok: false, cluster };
    return { output: `context ${name}`, ok: true, cluster: next };
  }
  if (meta === "view" || meta === "ui") {
    const parts = trimmed.split(/\s+/).slice(1);
    const all = parts.includes("-A") || parts.includes("--all-namespaces") || parts.includes("all");
    const word = parts.find((part) => part !== "all" && !part.startsWith("-"));
    const rule = ruleFor(host.policy ?? null, cluster.current);
    const next = openView(cluster, {
      board: boardFrom(word),
      all,
      yes: host.yes,
      typedConfirm: rule.confirm === "typed",
      reload: (wantAll, namespace) => {
        invalidateLive();
        return liveCluster(wantAll ? "kubectl get pods -A" : `kubectl get pods -n ${namespace}`);
      },
    });
    return { output: "", ok: true, cluster: next };
  }
  if (meta === "plan") {
    const asJson = /(^|\s)--json(\s|$)/.test(trimmed);
    const body = trimmed
      .replace(/^plan\b\s*/, "")
      .replace(/(^|\s)--json(\s|$)/g, " ")
      .trim();
    if (!body) return { output: "usage: kube plan [--json] delete pod <name>", ok: false, cluster };
    const view = toEngine(body, body.length, "repl");
    const planned = planCommand(view.line, cluster);
    const blast = wantsBlast(planned) ? liveBlast(planned, view.line, cluster) : undefined;
    if (asJson) {
      const payload = blast ? { ...planned, blast } : planned;
      const planId = mintPlan({
        context: planned.context,
        namespace: planned.namespace,
        argv: commandArgs(view.line, cluster),
      });
      return { output: JSON.stringify({ ...payload, planId }), ok: true, cluster };
    }
    const text = blast ? formatBlast(blast) : "";
    return {
      output: text ? `${formatPlan(planned)}\n${text}` : formatPlan(planned),
      ok: true,
      cluster,
    };
  }
  if (meta === "ask") {
    const asJson = /(^|\s)--json(\s|$)/.test(trimmed);
    const question = trimmed
      .replace(/^\?\s*/, "")
      .replace(/^ask\s+/, "")
      .replace(/(^|\s)--json(\s|$)/g, " ")
      .trim()
      .replace(/^["']|["']$/g, "");
    const advice = advise(question, cluster);
    if (asJson) return { output: JSON.stringify(advice), ok: true, cluster };
    if (!process.stdin.isTTY) return { output: formatAdvice(advice), ok: true, cluster };
    process.stderr.write(`${formatAdvice(advice)}\n`);
    if (!host.confirm("Run the first command? [y/N] ")) return { output: "", ok: true, cluster };
    const next = advice.commands[0];
    if (!next) return { output: "", ok: true, cluster };
    trimmed = next;
  }
  const view = toEngine(trimmed, trimmed.length, "repl");
  const planned = planCommand(view.line, cluster);
  const started = Date.now();
  const rule = ruleFor(host.policy ?? null, cluster.current);
  const argv = commandArgs(view.line, cluster);
  const actor = host.agent ? "agent-cli" : process.stdin.isTTY ? "human-tty" : "human-cli";
  const writeAudit = (
    decision: AuditDecision,
    approver: AuditApprover,
    exitCode: number | null,
  ) => {
    if (!auditEnabled(host.policy ?? null)) return;
    appendAudit({
      id: newAuditId(),
      timestamp: new Date().toISOString(),
      actor,
      context: planned.context,
      namespace: planned.namespace,
      argv: redactArgv(argv),
      risk: planned.risk,
      decision,
      approver,
      exitCode,
      durationMs: Date.now() - started,
    });
  };
  if (rule.deny.includes(planned.verb)) {
    writeAudit("denied-policy", "none", null);
    return { output: `refused: ${planned.verb} is denied by policy`, ok: false, cluster };
  }
  if (
    host.agent &&
    host.policy?.agents &&
    planned.risk === "read" &&
    !host.policy.agents.allow.includes(planned.verb)
  ) {
    writeAudit("denied-policy", "none", null);
    return { output: `refused: ${planned.verb} is not allowed for agents`, ok: false, cluster };
  }
  const needsPlan =
    host.agent === true &&
    host.policy?.agents?.writeRequires === "approved-plan" &&
    planned.mutating;
  if (needsPlan) {
    if (!host.planId) {
      writeAudit("denied-policy", "none", null);
      return {
        output: "refused: agent writes need --plan-id from plan --json",
        ok: false,
        cluster,
      };
    }
    const consumed = consumePlan(host.planId, {
      context: planned.context,
      namespace: planned.namespace,
      argv,
    });
    if (!consumed.ok) {
      writeAudit("denied-policy", "none", null);
      return { output: `refused: ${consumed.reason}`, ok: false, cluster };
    }
  } else if (rule.confirm === "typed" && host.yes && host.confirmContext !== cluster.current) {
    writeAudit("refused-confirm", "none", null);
    return {
      output: `refused: pass --confirm-context ${cluster.current}`,
      ok: false,
      cluster,
    };
  } else if (
    (planned.risk === "high" || (rule.mode === "guarded" && planned.risk === "low")) &&
    !host.yes
  ) {
    const extra = wantsBlast(planned) ? formatBlast(liveBlast(planned, view.line, cluster)) : "";
    process.stderr.write(`${formatPlan(planned)}${extra ? `\n${extra}` : ""}\n`);
    if (!host.confirm("Run it? [y/N] ")) {
      writeAudit("refused-confirm", "none", null);
      return { output: "not run", ok: false, cluster };
    }
  }
  const approver: AuditApprover = needsPlan
    ? `plan-id:${host.planId}`
    : rule.confirm === "typed" && host.yes
      ? "typed-context"
      : host.yes
        ? "flag-yes"
        : "prompt-y";
  const code = runLive(argv, { inheritStdin: options.inheritStdin });
  const dryRun = argv.some((token) => token === "--dry-run" || token.startsWith("--dry-run="));
  writeAudit(dryRun ? "dry-run" : "executed", approver, code);
  return { output: "", ok: code === 0, cluster: applyLiveContext(cluster) };
}
