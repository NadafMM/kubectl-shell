export type LeadingFlags = {
  yes: boolean;
  policy?: string;
  planId?: string;
  confirmContext?: string;
  sandbox: boolean;
  sample: boolean;
  record?: string;
  rest: string[];
  error?: string;
};

const VALUE_FLAGS = new Set(["--policy", "--plan-id", "--confirm-context", "--record"]);

export function leadingFlags(argv: string[]): LeadingFlags {
  const parsed: LeadingFlags = { yes: false, sandbox: false, sample: false, rest: [] };
  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i]!;
    if (parsed.rest.length > 0) {
      parsed.rest.push(arg);
      continue;
    }
    if (arg === "--live") continue;
    if (arg === "--help" || arg === "-h" || arg === "--version" || arg === "-v") {
      parsed.rest.push(arg);
      continue;
    }
    if (arg === "--sample") {
      parsed.sample = true;
      continue;
    }
    if (arg === "--yes" || arg === "-y") {
      parsed.yes = true;
      continue;
    }
    if (arg === "--sandbox") {
      parsed.sandbox = true;
      parsed.error = "kube: --sandbox is not available yet";
      return parsed;
    }
    const eq = arg.indexOf("=");
    const name = eq === -1 ? arg : arg.slice(0, eq);
    if (VALUE_FLAGS.has(name)) {
      const value = eq === -1 ? argv[i + 1] : arg.slice(eq + 1);
      if (!value || (eq === -1 && value.startsWith("-"))) {
        parsed.error = `kube: ${name} needs a value`;
        return parsed;
      }
      if (eq === -1) i += 1;
      if (name === "--policy") parsed.policy = value;
      else if (name === "--plan-id") parsed.planId = value;
      else if (name === "--confirm-context") parsed.confirmContext = value;
      else parsed.record = value;
      if (name === "--record") {
        parsed.error = "kube: --record is not available yet";
        return parsed;
      }
      continue;
    }
    if (arg.startsWith("-")) {
      parsed.error = `unknown flag ${arg}; flags after the command belong to kubectl`;
      return parsed;
    }
    parsed.rest.push(arg);
  }
  return parsed;
}
