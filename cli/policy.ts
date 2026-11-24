import fs from "node:fs";
import path from "node:path";
import { stateDir } from "./private";

export type ContextRule = {
  mode: "normal" | "guarded";
  confirm: "default" | "typed";
  deny: string[];
};

export type AgentRule = {
  allow: string[];
  writeRequires: "approved-plan" | "none";
  matchNonTty: boolean;
};

export type Policy = {
  contexts: { pattern: string; rule: ContextRule }[];
  agents: AgentRule | null;
};

const EMPTY_RULE: ContextRule = { mode: "normal", confirm: "default", deny: [] };

export function parsePolicy(text: string): Policy | { error: string } {
  let raw: unknown;
  try {
    raw = JSON.parse(text) as unknown;
  } catch {
    return { error: "policy is not valid JSON" };
  }
  if (!raw || typeof raw !== "object" || Array.isArray(raw))
    return { error: "policy must be an object" };
  const doc = raw as Record<string, unknown>;
  for (const key of Object.keys(doc)) {
    if (key !== "contexts" && key !== "agents") return { error: `unknown policy field ${key}` };
  }
  const contexts: Policy["contexts"] = [];
  if (doc.contexts !== undefined) {
    if (!doc.contexts || typeof doc.contexts !== "object" || Array.isArray(doc.contexts)) {
      return { error: "contexts must be an object" };
    }
    for (const [pattern, value] of Object.entries(doc.contexts as Record<string, unknown>)) {
      const rule = contextRule(value);
      if ("error" in rule) return rule;
      contexts.push({ pattern, rule });
    }
  }
  let agents: AgentRule | null = null;
  if (doc.agents !== undefined) {
    const parsed = agentRule(doc.agents);
    if ("error" in parsed) return parsed;
    agents = parsed;
  }
  return { contexts, agents };
}

function contextRule(value: unknown): ContextRule | { error: string } {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return { error: "a context rule must be an object" };
  }
  const doc = value as Record<string, unknown>;
  for (const key of Object.keys(doc)) {
    if (key !== "mode" && key !== "confirm" && key !== "deny") {
      return { error: `unknown context field ${key}` };
    }
  }
  const mode = doc.mode === undefined ? "normal" : doc.mode;
  if (mode !== "normal" && mode !== "guarded") return { error: "mode must be normal or guarded" };
  const confirm = doc.confirm === undefined ? "default" : doc.confirm;
  if (confirm !== "default" && confirm !== "typed") return { error: "confirm must be typed" };
  const deny = doc.deny === undefined ? [] : doc.deny;
  if (!Array.isArray(deny) || deny.some((item) => typeof item !== "string")) {
    return { error: "deny must be a list of verbs" };
  }
  return { mode, confirm, deny };
}

function agentRule(value: unknown): AgentRule | { error: string } {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return { error: "agents must be an object" };
  }
  const doc = value as Record<string, unknown>;
  for (const key of Object.keys(doc)) {
    if (key !== "allow" && key !== "write_requires" && key !== "match_non_tty") {
      return { error: `unknown agents field ${key}` };
    }
  }
  const allow = doc.allow === undefined ? [] : doc.allow;
  if (!Array.isArray(allow) || allow.some((item) => typeof item !== "string")) {
    return { error: "agents.allow must be a list of verbs" };
  }
  const writeRequires = doc.write_requires === undefined ? "none" : doc.write_requires;
  if (writeRequires !== "none" && writeRequires !== "approved-plan") {
    return { error: "write_requires must be approved-plan" };
  }
  const matchNonTty = doc.match_non_tty === undefined ? false : doc.match_non_tty;
  if (typeof matchNonTty !== "boolean") return { error: "match_non_tty must be true or false" };
  return { allow, writeRequires, matchNonTty };
}

export function ruleFor(policy: Policy | null, context: string): ContextRule {
  if (!policy) return EMPTY_RULE;
  const exact = policy.contexts.find((item) => item.pattern === context);
  if (exact) return exact.rule;
  const wild = policy.contexts
    .filter((item) => item.pattern.endsWith("*") && context.startsWith(item.pattern.slice(0, -1)))
    .sort((a, b) => b.pattern.length - a.pattern.length);
  return wild[0]?.rule ?? EMPTY_RULE;
}

export function policyFile(options: { agent: boolean; flag?: string; envPath?: string }): string {
  if (options.agent) return path.join(stateDir(), "policy.json");
  return options.flag || options.envPath || path.join(stateDir(), "policy.json");
}

export function readPolicy(file: string): Policy | null | { error: string } {
  if (!fs.existsSync(file)) return null;
  return parsePolicy(fs.readFileSync(file, "utf8"));
}

export function promptLabel(guarded: boolean, body: string): string {
  if (!guarded) return body;
  return `[GUARDED] ${body}`;
}
