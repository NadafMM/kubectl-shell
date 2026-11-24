import fs from "node:fs";
import type { Cluster } from "../src/lib/kube/model";
import { namespaceOf } from "../src/lib/kube/model";
import { appendAudit, auditEnabled, newAuditId, redactArgv } from "./audit";
import { blastRadius, formatBlast } from "./blast";
import { commandArgs } from "./commands";
import { toEngine } from "./engine";
import { planCommand } from "./insight";
import { captureLive, runJson } from "./live";
import { mintPlan, consumePlan } from "./plans";
import { policyFile, readPolicy, ruleFor, type Policy } from "./policy";
import { redactKubectlStdout } from "./redact-output";
import { sensitiveStdoutKind } from "./reveal-output";

type Rpc = {
  jsonrpc?: string;
  id?: number | string | null;
  method?: string;
  params?: { name?: string; arguments?: Record<string, unknown> };
};

const DEFAULT_ALLOW = ["get", "describe", "logs", "plan"];

function policy(): Policy | null {
  const loaded = readPolicy(policyFile({ agent: true }));
  if (loaded && "error" in loaded) return null;
  return loaded;
}

function toolResult(id: number | string | null, text: string, isError = false) {
  return {
    jsonrpc: "2.0",
    id,
    result: { content: [{ type: "text", text }], isError },
  };
}

function record(
  policy: Policy | null,
  cluster: Cluster,
  argv: string[],
  risk: "read" | "low" | "high",
  decision: "executed" | "denied-policy" | "refused-confirm",
  exitCode: number | null,
  started: number,
  approver: "none" | `plan-id:${string}`,
) {
  if (!auditEnabled(policy)) return;
  appendAudit({
    id: newAuditId(),
    timestamp: new Date().toISOString(),
    actor: "mcp",
    context: cluster.current,
    namespace: namespaceOf(cluster),
    argv: redactArgv(argv),
    risk,
    decision,
    approver,
    exitCode,
    durationMs: Date.now() - started,
  });
}

function plannedOf(command: string, cluster: Cluster) {
  const view = toEngine(command, command.length, "repl");
  const planned = planCommand(view.line, cluster);
  const argv = commandArgs(view.line, cluster);
  return { planned, argv };
}

export function handleRpc(message: Rpc, cluster: Cluster): unknown | null {
  if (message.id === undefined || message.id === null) return null;
  const id = message.id;
  if (message.method === "initialize") {
    return {
      jsonrpc: "2.0",
      id,
      result: {
        protocolVersion: "2024-11-05",
        capabilities: { tools: {} },
        serverInfo: { name: "kube", version: "0.5.1" },
      },
    };
  }
  if (message.method === "tools/list") {
    return {
      jsonrpc: "2.0",
      id,
      result: {
        tools: [
          { name: "kube_context", description: "Current context, namespace, and policy summary" },
          { name: "kube_plan", description: "Plan a command and mint a single-use plan id" },
          { name: "kube_execute", description: "Run an allowed read, or a write with a plan id" },
        ],
      },
    };
  }
  if (message.method !== "tools/call") {
    return { jsonrpc: "2.0", id, error: { code: -32601, message: "method not found" } };
  }
  const name = message.params?.name ?? "";
  const args = message.params?.arguments ?? {};
  const active = policy();
  const rule = ruleFor(active, cluster.current);
  const allow = active?.agents?.allow ?? DEFAULT_ALLOW;
  if (name === "kube_context") {
    return toolResult(
      id,
      JSON.stringify({
        context: cluster.current,
        namespace: namespaceOf(cluster),
        guarded: rule.mode === "guarded",
        confirm: rule.confirm,
        allow,
      }),
    );
  }
  const command = typeof args.command === "string" ? args.command : "";
  if (!command) return toolResult(id, "command is required", true);
  const { planned, argv } = plannedOf(command, cluster);
  const started = Date.now();
  if (name === "kube_plan") {
    const blast = blastRadius(planned, argv, (kubectl) => runJson(kubectl, 3000));
    const planId = mintPlan({ context: planned.context, namespace: planned.namespace, argv });
    record(active, cluster, argv, planned.risk, "executed", 0, started, "none");
    return toolResult(id, JSON.stringify({ ...planned, blast, planId, text: formatBlast(blast) }));
  }
  if (name !== "kube_execute") return toolResult(id, "unknown tool", true);
  if (rule.deny.includes(planned.verb)) {
    record(active, cluster, argv, planned.risk, "denied-policy", null, started, "none");
    return toolResult(id, `refused: ${planned.verb} is denied by policy`, true);
  }
  if (planned.mutating) {
    const typedBlocked = rule.confirm === "typed" && !allow.includes(planned.verb);
    const planId = typeof args.planId === "string" ? args.planId : "";
    if (typedBlocked || !planId) {
      record(active, cluster, argv, planned.risk, "denied-policy", null, started, "none");
      return toolResult(id, "refused: mutating commands need a plan id", true);
    }
    const consumed = consumePlan(planId, {
      context: planned.context,
      namespace: planned.namespace,
      argv,
    });
    if (!consumed.ok) {
      record(active, cluster, argv, planned.risk, "denied-policy", null, started, "none");
      return toolResult(id, `refused: ${consumed.reason}`, true);
    }
    const result = captureLive(argv);
    record(
      active,
      cluster,
      argv,
      planned.risk,
      "executed",
      result.code,
      started,
      `plan-id:${planId}`,
    );
    return toolResult(id, result.stdout || result.stderr, result.code !== 0);
  }
  if (!allow.includes(planned.verb)) {
    record(active, cluster, argv, planned.risk, "denied-policy", null, started, "none");
    return toolResult(id, `refused: ${planned.verb} is not allowed`, true);
  }
  const result = captureLive(argv);
  const stdout =
    sensitiveStdoutKind(argv) !== null ? redactKubectlStdout(argv, result.stdout) : result.stdout;
  record(active, cluster, argv, planned.risk, "executed", result.code, started, "none");
  return toolResult(id, stdout, result.code !== 0);
}

export function serveMcp(cluster: Cluster) {
  const buf = fs.readFileSync(0);
  let offset = 0;
  while (offset + 4 < buf.length) {
    const headerEnd = buf.indexOf("\r\n\r\n", offset);
    if (headerEnd < 0) break;
    const header = buf.subarray(offset, headerEnd).toString("utf8");
    const match = /Content-Length:\s*(\d+)/i.exec(header);
    if (!match) break;
    const length = Number(match[1]);
    const start = headerEnd + 4;
    const body = buf.subarray(start, start + length).toString("utf8");
    offset = start + length;
    const response = handleRpc(JSON.parse(body) as Rpc, cluster);
    if (!response) continue;
    const payload = Buffer.from(JSON.stringify(response));
    process.stdout.write(`Content-Length: ${payload.length}\r\n\r\n`);
    process.stdout.write(payload);
  }
}
