import { sensitiveStdoutKind } from "./reveal-output";

export function wantsRedactedStdout(args: string[]): boolean {
  if (process.env.KUBE_SHELL_REDACT_STDOUT !== "1") return false;
  return sensitiveStdoutKind(args) !== null;
}

function redactJsonMap(block: Record<string, string>): Record<string, string> {
  const out: Record<string, string> = {};
  for (const key of Object.keys(block)) out[key] = "***";
  return out;
}

function redactJsonValue(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(redactJsonValue);
  if (value && typeof value === "object") {
    const record = value as Record<string, unknown>;
    const out: Record<string, unknown> = {};
    let changed = false;
    for (const [key, child] of Object.entries(record)) {
      if (key === "data" || key === "stringData" || key === "binaryData") {
        if (child && typeof child === "object" && !Array.isArray(child)) {
          out[key] = redactJsonMap(child as Record<string, string>);
          changed = true;
          continue;
        }
      }
      if (
        key === "token" ||
        key === "client-certificate-data" ||
        key === "client-key-data" ||
        key === "password"
      ) {
        out[key] = "***";
        changed = true;
        continue;
      }
      const next = redactJsonValue(child);
      out[key] = next;
      if (next !== child) changed = true;
    }
    return changed ? out : value;
  }
  return value;
}

function redactJson(stdout: string): string {
  try {
    const parsed = JSON.parse(stdout) as unknown;
    const redacted = redactJsonValue(parsed);
    if (redacted === parsed) return stdout;
    return `${JSON.stringify(redacted, null, 2)}\n`;
  } catch {
    return stdout;
  }
}

const SENSITIVE_YAML_KEYS =
  /^(data|stringData|token|client-certificate-data|client-key-data|password|client-secret)$/;

function redactYaml(stdout: string): string {
  const lines = stdout.split("\n");
  const out: string[] = [];
  let inData = false;
  let dataIndent = 0;
  for (const line of lines) {
    const trimmed = line.trimStart();
    const indent = line.length - trimmed.length;
    const keyMatch = /^([a-zA-Z0-9_.-]+):\s*(.*)$/.exec(trimmed);
    if (keyMatch && SENSITIVE_YAML_KEYS.test(keyMatch[1]!) && keyMatch[2] === "") {
      inData = keyMatch[1] === "data" || keyMatch[1] === "stringData";
      dataIndent = indent;
      out.push(line);
      continue;
    }
    if (inData && trimmed && indent <= dataIndent && !/^(data|stringData):/.test(trimmed)) {
      inData = false;
    }
    if (keyMatch && SENSITIVE_YAML_KEYS.test(keyMatch[1]!) && keyMatch[2] !== "") {
      out.push(`${line.slice(0, line.indexOf(":") + 1)} "***"`);
      continue;
    }
    if (inData && /:\s*$/.test(trimmed) === false && /:\s*.+/.test(trimmed)) {
      const colon = line.indexOf(":");
      if (colon >= 0) {
        out.push(`${line.slice(0, colon + 1)} "***"`);
        continue;
      }
    }
    out.push(line);
  }
  return out.join("\n");
}

export function redactKubectlStdout(args: string[], stdout: string): string {
  if (!sensitiveStdoutKind(args)) return stdout;
  const trimmed = stdout.trim();
  if (trimmed.startsWith("{")) return redactJson(stdout);
  if (trimmed.includes("apiVersion:") || trimmed.includes("data:") || trimmed.includes("clusters:"))
    return redactYaml(stdout);
  return stdout;
}
