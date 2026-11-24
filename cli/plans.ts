import crypto from "node:crypto";
import fs from "node:fs";
import path from "node:path";
import { stateDir, writePrivate } from "./private";

const TTL_MS = 5 * 60 * 1000;

export type PlanRecord = {
  planId: string;
  context: string;
  namespace: string;
  argv: string[];
  expires: number;
};

function planPath(id: string): string {
  return path.join(stateDir(), "plans", `${id}.json`);
}

export function mintPlan(
  record: { context: string; namespace: string; argv: string[] },
  now = Date.now(),
): string {
  const planId = crypto.randomBytes(16).toString("hex");
  const stored: PlanRecord = { planId, ...record, expires: now + TTL_MS };
  writePrivate(planPath(planId), JSON.stringify(stored));
  return planId;
}

export function consumePlan(
  planId: string,
  expected: { context: string; namespace: string; argv: string[] },
  now = Date.now(),
): { ok: true } | { ok: false; reason: string } {
  const file = planPath(planId);
  let stored: PlanRecord;
  try {
    stored = JSON.parse(fs.readFileSync(file, "utf8")) as PlanRecord;
  } catch {
    return { ok: false, reason: "plan expired or unknown" };
  }
  if (!stored.expires || stored.expires <= now) {
    fs.rmSync(file, { force: true });
    return { ok: false, reason: "plan expired or unknown" };
  }
  const same =
    stored.context === expected.context &&
    stored.namespace === expected.namespace &&
    JSON.stringify(stored.argv) === JSON.stringify(expected.argv);
  if (!same) return { ok: false, reason: "plan does not match this command" };
  fs.rmSync(file, { force: true });
  return { ok: true };
}
