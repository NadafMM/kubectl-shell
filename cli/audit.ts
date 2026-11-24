import crypto from "node:crypto";
import fs from "node:fs";
import path from "node:path";
import { stateDir, writePrivate } from "./private";
import { redactLine } from "./redact";
import type { Policy } from "./policy";

const MAX_BYTES = 5 * 1024 * 1024;

export type AuditDecision = "executed" | "refused-confirm" | "denied-policy" | "dry-run";
export type AuditApprover =
  "typed-context" | "prompt-y" | "flag-yes" | `plan-id:${string}` | "none";
export type AuditActor = "human-tty" | "human-cli" | "agent-cli" | "mcp";

export type AuditEntry = {
  id: string;
  timestamp: string;
  actor: AuditActor;
  context: string;
  namespace: string;
  argv: string[];
  risk: "read" | "low" | "high" | "none";
  decision: AuditDecision;
  approver: AuditApprover;
  exitCode: number | null;
  durationMs: number;
};

function auditPath(): string {
  return path.join(stateDir(), "audit.jsonl");
}

export function auditEnabled(policy: Policy | null, env = process.env): boolean {
  if (policy?.audit === "required") return true;
  return env.KUBE_SHELL_AUDIT !== "0";
}

export function redactArgv(argv: string[]): string[] {
  return redactLine(argv.join(" ")).split(" ");
}

export function appendAudit(entry: AuditEntry, maxBytes = MAX_BYTES): void {
  const file = auditPath();
  const line = `${JSON.stringify(entry)}\n`;
  try {
    if (fs.existsSync(file) && fs.statSync(file).size >= maxBytes) {
      const previous = `${file}.1`;
      fs.rmSync(previous, { force: true });
      fs.renameSync(file, previous);
    }
    if (!fs.existsSync(file)) writePrivate(file, line);
    else {
      fs.appendFileSync(file, line);
      fs.chmodSync(file, 0o600);
    }
  } catch {
    return;
  }
  if (process.env.KUBE_SHELL_AUDIT_STDERR === "1") process.stderr.write(line);
}

export function tailAudit(count = 20): string[] {
  const file = auditPath();
  if (!fs.existsSync(file)) return [];
  const lines = fs.readFileSync(file, "utf8").split("\n").filter(Boolean);
  const n = Number.isFinite(count) && count > 0 ? Math.floor(count) : 20;
  return lines.slice(-n);
}

export function newAuditId(): string {
  return crypto.randomBytes(8).toString("hex");
}
