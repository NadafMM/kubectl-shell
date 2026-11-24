export type ShellCommand = { value: string; detail: string; weight: number; aliases?: string[] };

export const SHELL_COMMANDS: ShellCommand[] = [
  { value: "view", detail: "Full-screen resource list", weight: 98, aliases: ["ui"] },
  { value: "ctx", detail: "Switch context for this shell only", weight: 97 },
  { value: "ns", detail: "Switch namespace for this shell only", weight: 96 },
  { value: "plan", detail: "What a command will change", weight: 92 },
  { value: "ask", detail: "Turn a question into kubectl lines", weight: 90 },
  { value: "live", detail: "Reconnect to the cluster", weight: 86 },
  { value: "reset", detail: "Refresh names from the cluster", weight: 84 },
  { value: "audit", detail: "Show recent redacted command records", weight: 70 },
  { value: "undo", detail: "Restore the previous saved object", weight: 68 },
  { value: "help", detail: "List shell commands", weight: 50 },
  { value: "clear", detail: "Clear the screen", weight: 40 },
  { value: "exit", detail: "Leave the shell", weight: 30, aliases: ["quit"] },
];

export const VIEW_ARGS = [
  { value: "pods", detail: "Pods board", kind: "command", weight: 100 },
  { value: "deploy", detail: "Deployments board", kind: "command", weight: 96 },
  { value: "svc", detail: "Services board", kind: "command", weight: 92 },
  { value: "nodes", detail: "Nodes board", kind: "command", weight: 88 },
  { value: "ns", detail: "Namespaces board", kind: "command", weight: 84 },
  { value: "all", detail: "Every namespace", kind: "command", weight: 80 },
  { value: "-A", detail: "Every namespace", kind: "command", weight: 80 },
  { value: "--all-namespaces", detail: "Every namespace", kind: "command", weight: 78 },
] as const;

function namesOf(command: ShellCommand): string[] {
  return [command.value, ...(command.aliases ?? [])];
}

const CLI_WORDS = new Set(["--help", "-h", "--version", "-v", "completion", "sample"]);

/** Shell command or CLI flag that must not be sent to kubectl. */
export function metaOf(line: string): string | undefined {
  const trimmed = line.trim();
  if (trimmed.startsWith("?")) return "ask";
  const word = trimmed.split(/\s+/)[0];
  if (!word) return undefined;
  if (CLI_WORDS.has(word)) return word;
  for (const command of SHELL_COMMANDS) {
    if (namesOf(command).includes(word)) return word;
  }
  return undefined;
}

/** True when the REPL line should not get a kubectl prefix for completion. */
export function shellMetaFirstWord(first: string): boolean {
  if (!first) return false;
  const lower = first.toLowerCase();
  for (const command of SHELL_COMMANDS) {
    for (const name of namesOf(command)) {
      if (lower === name) return true;
      if (name.startsWith(lower) && lower.length >= 2) return true;
    }
  }
  return false;
}

export function shellCommandCandidates(
  prefixToken: string,
): { value: string; detail: string; kind: string; weight: number; aliases?: string[] }[] {
  return SHELL_COMMANDS.map((command) => ({
    value: command.value,
    detail: command.detail,
    kind: "command",
    weight: command.weight,
    aliases: command.aliases,
  })).filter((item) => {
    const p = prefixToken.toLowerCase();
    const names = [item.value, ...(item.aliases ?? [])];
    return names.some((name) => !p || name === p || name.startsWith(p) || p.startsWith(name));
  });
}
