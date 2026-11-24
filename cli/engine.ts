import { shellMetaFirstWord } from "../src/lib/kube/shell-meta.ts";

export type EngineView = { line: string; cursor: number; delta: number; skip: boolean };

export function toEngine(line: string, cursor: number, mode: "repl" | "shell"): EngineView {
  const c = Math.max(0, Math.min(cursor, line.length));
  const match = /^(\s*)(\S*)/.exec(line);
  const lead = match?.[1] ?? "";
  const first = match?.[2] ?? "";
  const firstEnd = lead.length + first.length;

  if (mode === "shell" && (first === "kube" || first === "kubectl-shell")) {
    if (c <= firstEnd) return { line, cursor: c, delta: 0, skip: true };
    const delta = "kubectl".length - first.length;
    return {
      line: `${lead}kubectl${line.slice(firstEnd)}`,
      cursor: c + delta,
      delta,
      skip: false,
    };
  }

  if (first === "kubectl" || first === "k" || first === "help") {
    return { line, cursor: c, delta: 0, skip: false };
  }

  if (shellMetaFirstWord(first)) {
    return { line, cursor: c, delta: 0, skip: false };
  }

  if (!first) {
    const engineLine = `${lead}kubectl `;
    return {
      line: engineLine,
      cursor: engineLine.length,
      delta: engineLine.length - lead.length,
      skip: false,
    };
  }

  const delta = "kubectl ".length;
  return {
    line: `${lead}kubectl ${line.slice(lead.length)}`,
    cursor: c <= lead.length ? c : c + delta,
    delta,
    skip: false,
  };
}

export function spliceToken(
  line: string,
  start: number,
  end: number,
  value: string,
  pad: boolean,
): { line: string; cursor: number } {
  const after = line.slice(end);
  const spaced = pad && !/[=/]$/.test(value);
  if (!spaced || after.startsWith(" ")) {
    const next = line.slice(0, start) + value + after;
    const cursor = start + value.length + (spaced && after.startsWith(" ") ? 1 : 0);
    return { line: next, cursor };
  }
  return {
    line: line.slice(0, start) + value + " " + after,
    cursor: start + value.length + 1,
  };
}
