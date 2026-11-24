import fs from "node:fs";
import path from "node:path";
import type { AuditDecision } from "./audit";
import { redactArgv } from "./audit";
import { VERBS } from "../src/lib/kube/model";
import { writePrivate } from "./private";

const VERB_NAMES = new Set(VERBS.map((item) => item.name));

export type RunbookEntry = {
  decision: AuditDecision;
  verb: string;
  timestamp: string;
  context: string;
  namespace: string;
  command: string;
  risk: "read" | "low" | "high";
  exitCode: number | null;
  summary?: string;
  blast?: string;
};

let active: string | null = null;

export function sessionPath(home: string, context: string, now: Date): string {
  const stamp = now.toISOString().replace(/[:.]/g, "-");
  const safe = context.replace(/[^A-Za-z0-9._-]+/g, "-").replace(/^-|-$/g, "") || "none";
  return path.join(home, ".kube-shell", "sessions", `${stamp}-${safe}.md`);
}

export function redactCommand(argv: string[]): string {
  return `kubectl ${redactArgv(argv).join(" ")}`;
}

export function shouldRecord(entry: Pick<RunbookEntry, "decision" | "verb">): boolean {
  if (entry.decision !== "executed" && entry.decision !== "dry-run") return false;
  return VERB_NAMES.has(entry.verb);
}

export function recording(): string | null {
  return active;
}

export function startRecord(
  file: string,
  header: { context: string; namespace: string; started: string },
): string {
  if (active) return active;
  active = file;
  if (!fs.existsSync(file)) {
    writePrivate(
      file,
      `# kube session\n\n- context: ${header.context}\n- namespace: ${header.namespace}\n- started: ${header.started}\n`,
    );
  }
  return file;
}

export function stopRecord(): string | null {
  const file = active;
  active = null;
  return file;
}

export function noteRan(entry: RunbookEntry): void {
  if (!active || !shouldRecord(entry)) return;
  const lines = [
    "",
    `## ${entry.command}`,
    "",
    `- at: ${entry.timestamp}`,
    `- context: ${entry.context}`,
    `- namespace: ${entry.namespace}`,
    `- exit: ${entry.exitCode}`,
    "",
  ];
  if (entry.summary) lines.push(entry.summary, "");
  if (entry.blast) lines.push(entry.blast, "");
  const previous = fs.readFileSync(active, "utf8");
  writePrivate(active, `${previous}${lines.join("\n")}`);
}
