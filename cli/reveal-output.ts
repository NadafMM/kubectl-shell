import { resolveResource } from "../src/lib/kube/model";
import { flagValue, positionalWords } from "../src/lib/kube/parse";

function outputFormatToken(token: string): string | undefined {
  if (token === "-o" || token === "--output") return undefined;
  if (token.startsWith("-o=")) return token.slice(3);
  if (token.startsWith("--output=")) return token.slice("--output=".length);
  return undefined;
}

function isRevealingFormat(fmt: string): boolean {
  if (fmt === "yaml" || fmt === "json" || fmt === "wide") return true;
  if (fmt.startsWith("jsonpath")) return true;
  if (fmt === "template" || fmt.startsWith("go-template")) return true;
  if (fmt.startsWith("custom-columns")) return true;
  return false;
}

function revealingOutputFormat(tokens: string[]): boolean {
  const output = flagValue(tokens, ["-o", "--output"]);
  if (output && isRevealingFormat(output)) return true;
  return tokens.some((token) => {
    const inline = outputFormatToken(token);
    if (inline) return isRevealingFormat(inline);
    return token === "-o=wide" || token === "--output=wide";
  });
}

function hasFlag(tokens: string[], name: string): boolean {
  return tokens.some((token) => token === name || token.startsWith(`${name}=`));
}

function resourceTokenKind(token: string): "secret" | "configmap" | null {
  const resource = resolveResource(token);
  if (resource?.kind === "secret" || token === "secret" || token === "secrets") return "secret";
  if (resource?.kind === "configmap" || token === "configmap" || token === "configmaps")
    return "configmap";
  return null;
}

/** Whether kubectl may print Secret or ConfigMap payloads (plan warnings and optional redaction). */
export function sensitiveStdoutKind(
  tokens: string[],
): "secret" | "configmap" | "kubeconfig" | null {
  const { words } = positionalWords(tokens);
  const verb = words[0] ?? "";

  if (verb === "get" && revealingOutputFormat(tokens)) {
    for (let i = 1; i < words.length; i += 1) {
      const kind = resourceTokenKind(words[i]!);
      if (kind) return kind;
    }
    return null;
  }

  if (verb === "describe") {
    return resourceTokenKind(words[1] ?? "");
  }

  if (verb === "config" && words[1] === "view" && hasFlag(tokens, "--raw")) return "kubeconfig";

  return null;
}
