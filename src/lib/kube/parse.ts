import { flagCatalog, takesOf, type Cluster, currentContext } from "./model";

export { takesOf };

export type Token = { text: string; start: number; end: number };

export function tokenize(line: string): Token[] {
  const tokens: Token[] = [];
  let i = 0;
  while (i < line.length) {
    while (i < line.length && line[i] === " ") i += 1;
    if (i >= line.length) break;
    const start = i;
    let text = "";
    const quote = line[i] === '"' || line[i] === "'" ? line[i] : "";
    if (quote) {
      text += line[i];
      i += 1;
      while (i < line.length && line[i] !== quote) {
        text += line[i];
        i += 1;
      }
      if (i < line.length) {
        text += line[i];
        i += 1;
      }
    } else {
      while (i < line.length && line[i] !== " ") {
        text += line[i];
        i += 1;
      }
    }
    tokens.push({ text, start, end: i });
  }
  return tokens;
}

export function locate(
  line: string,
  cursor: number,
): { start: number; end: number; index: number; tokens: Token[] } {
  const tokens = tokenize(line);
  const c = Math.max(0, Math.min(cursor, line.length));
  for (let i = 0; i < tokens.length; i += 1) {
    const token = tokens[i]!;
    if (c < token.start) return { start: c, end: c, index: i, tokens };
    if (c <= token.end) return { start: token.start, end: token.end, index: i, tokens };
  }
  return { start: c, end: c, index: tokens.length, tokens };
}

export type Parsed = {
  words: string[];
  afterDashDash: boolean;
  dashWords: string[];
};

/** Drop flags (and the values they consume) so only positionals remain. */
export function positionalWords(tokens: string[]): Parsed {
  const words: string[] = [];
  const dashWords: string[] = [];
  let after = false;
  for (let i = 0; i < tokens.length; i += 1) {
    const token = tokens[i]!;
    if (after) {
      dashWords.push(token);
      continue;
    }
    if (token === "--") {
      after = true;
      continue;
    }
    if (token.startsWith("-") && token !== "-") {
      if (!token.includes("=") && takesOf(token, words[0])) i += 1;
      continue;
    }
    words.push(token);
  }
  return { words, afterDashDash: after, dashWords };
}

export type Scope = { ns: string; all: boolean; context: string };

export function scopeOf(tokens: string[], cluster: Cluster): Scope {
  let context = cluster.current;
  let ns = currentContext(cluster).namespace;
  let all = false;
  for (let i = 0; i < tokens.length; i += 1) {
    const token = tokens[i]!;
    if (token === "-A" || token === "--all-namespaces") {
      all = true;
      continue;
    }
    if (token === "--context" && tokens[i + 1]) {
      context = tokens[i + 1]!;
      i += 1;
      const found = cluster.contexts.find((c) => c.name === context);
      if (found) ns = found.namespace;
      continue;
    }
    if (token.startsWith("--context=")) {
      context = token.slice("--context=".length);
      const found = cluster.contexts.find((c) => c.name === context);
      if (found) ns = found.namespace;
      continue;
    }
    if ((token === "-n" || token === "--namespace") && tokens[i + 1]) {
      ns = tokens[i + 1]!;
      i += 1;
      continue;
    }
    if (token.startsWith("--namespace=")) ns = token.slice("--namespace=".length);
  }
  return { ns, all, context };
}

export function verbOf(args: string[]): string | undefined {
  return positionalWords(args).words[0];
}

export function flagsForVerb(verb?: string) {
  return flagCatalog(verb);
}

export function quoteArg(text: string): string | null {
  const hasSingle = text.includes("'");
  const hasDouble = text.includes('"');
  if (!text.includes(" ") && !hasSingle && !hasDouble) return text;
  if (hasSingle && hasDouble) return null;
  if (hasDouble) return `'${text}'`;
  return `"${text}"`;
}

export function unquote(text: string): string {
  if (
    text.length >= 2 &&
    ((text.startsWith('"') && text.endsWith('"')) || (text.startsWith("'") && text.endsWith("'")))
  ) {
    return text.slice(1, -1);
  }
  return text;
}

export function flagValue(tokens: string[], names: string[]): string | undefined {
  for (let i = 0; i < tokens.length; i += 1) {
    const token = tokens[i]!;
    for (const name of names) {
      if (token === name) return tokens[i + 1];
      if (token.startsWith(`${name}=`)) return token.slice(name.length + 1);
    }
  }
  return undefined;
}
