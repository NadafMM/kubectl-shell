import fs from "node:fs";
import path from "node:path";
import readline from "node:readline";
import { complete, type Candidate } from "../src/lib/kube/complete";
import { allowYes, readConfirm } from "./confirm";
import { spliceToken, toEngine } from "./engine";
import { completionValue, plainText, refreshLine } from "./terminal";
import { runUserLine, type LineResult } from "./commands";
import { metaOf } from "../src/lib/kube/shell-meta";
import { stateDir, writePrivate } from "./private";
import { redactLine } from "./redact";
import { liveCluster, probeLive } from "./live";
import { openPromptInput, readSyncRetry } from "./tty";
import { promptOf, type Cluster } from "../src/lib/kube/model";
import { quoteArg } from "../src/lib/kube/parse";

declare const __KUBE_VERSION__: string | undefined;
const VERSION = typeof __KUBE_VERSION__ === "string" ? __KUBE_VERSION__ : "0.0.0-dev";
const HISTORY = path.join(stateDir(), "history");

type Session = { yes: boolean };
const session: Session = { yes: false };

type RL = readline.Interface & {
  line: string;
  cursor: number;
  input: NodeJS.ReadableStream;
  output: NodeJS.WritableStream;
};

function loadHistory(): string[] {
  try {
    return fs
      .readFileSync(HISTORY, "utf8")
      .split("\n")
      .map((line) => line.trimEnd())
      .filter(Boolean)
      .slice(-200)
      .reverse();
  } catch {
    return [];
  }
}

const HISTORY_LIMIT = 200;

function trimHistoryFile() {
  try {
    const lines = fs.readFileSync(HISTORY, "utf8").split("\n").filter(Boolean);
    if (lines.length <= HISTORY_LIMIT) return;
    writePrivate(HISTORY, `${lines.slice(-HISTORY_LIMIT).join("\n")}\n`);
  } catch {
    /* no history yet */
  }
}

function remember(line: string) {
  try {
    const trimmed = redactLine(line.trim());
    if (!trimmed) return;
    if (!fs.existsSync(HISTORY)) writePrivate(HISTORY, "");
    else fs.chmodSync(HISTORY, 0o600);
    fs.appendFileSync(HISTORY, `${trimmed}\n`);
    trimHistoryFile();
  } catch {
    /* history is best-effort */
  }
}

function cliHelp(): string {
  return `kube ${VERSION}

Tab completes against the current cluster. kube exits if kubectl cannot reach it.

  kube                         shell. Tab fills the next word
  kube get pods                one command, then exit
  kube view                    full-screen pods, logs, and describe
  kube plan delete pod <name>  what it will change, before it runs
  kube ask "why is it crashing"
  kube plan --json get pods    same plan, for an agent
  kube --yes delete pod <name> skip the confirmation

Inside the shell the kubectl prefix is optional. A line starting with ? is ask.
\`view\` and \`ui\` open the resource screen. Delete, drain, and scale-to-zero ask before they run.
\`live\` reconnects. \`reset\` refreshes names from the cluster.
Up and down walk history. Ctrl+C clears the line, Ctrl+D exits.

Install: brew tap NadafMM/tap && brew install kubectl-shell`;
}

function suggest(line: string, cursor: number, mode: "repl" | "shell", fallback: Cluster) {
  const view = toEngine(line, cursor, mode);
  const cluster = view.skip ? fallback : (liveCluster(view.line) ?? fallback);
  if (view.skip)
    return { view, candidates: [] as Candidate[], start: cursor, end: cursor, cluster };
  const found = complete(view.line, view.cursor, cluster);
  return {
    view,
    candidates: found.candidates,
    start: Math.max(0, found.start - view.delta),
    end: Math.max(0, found.end - view.delta),
    cluster,
  };
}

function confirm(prompt: string): boolean {
  if (!process.stdin.isTTY || session.yes) return session.yes;
  process.stderr.write(prompt);
  const input = openPromptInput();
  try {
    return readConfirm(input.fd, readSyncRetry, (text) => process.stderr.write(text));
  } finally {
    input.close();
  }
}

function host() {
  return { yes: session.yes, help: cliHelp(), version: VERSION, confirm };
}

function writeResult(result: Pick<LineResult, "output" | "ok">): void {
  if (!result.output) return;
  const text = result.output.endsWith("\n") ? result.output : `${result.output}\n`;
  if (result.ok) {
    process.stdout.write(text);
  } else {
    process.stderr.write(text);
  }
}

function visible(candidates: Candidate[]): Candidate[] {
  return candidates.flatMap((item) => {
    const value = completionValue(item.value);
    if (value === null) return [];
    return [{ ...item, value, detail: plainText(item.detail) }];
  });
}

function printComplete(line: string, cursor: number, cluster: Cluster, asJson: boolean) {
  const candidates = visible(suggest(line, cursor, "shell", cluster).candidates);
  if (asJson) {
    process.stdout.write(
      `${JSON.stringify(candidates.map(({ value, detail, kind }) => ({ value, detail, kind })))}\n`,
    );
    return;
  }
  process.stdout.write(candidates.map((item) => item.value).join("\n"));
  if (candidates.length) process.stdout.write("\n");
}

function fillTab(rl: RL, getCluster: () => Cluster, setCluster: (cluster: Cluster) => void) {
  const found = suggest(rl.line, rl.cursor, "repl", getCluster());
  setCluster(found.cluster);
  const candidates = visible(found.candidates);
  const { start, end } = found;
  if (candidates.length === 0) {
    rl.output.write("\x07");
    return;
  }
  const top = candidates[0]!;
  const next = spliceToken(rl.line, start, end, top.value, true);
  if (next.line === rl.line && next.cursor === rl.cursor) {
    if (candidates.length > 1) {
      const width = Math.max(...candidates.map((item) => item.value.length));
      const body = candidates
        .map((item) => `  ${item.value.padEnd(width)}  ${item.detail}`)
        .join("\r\n");
      rl.output.write(`\r\n${body}\r\n`);
    } else {
      rl.output.write("\x07");
    }
    return;
  }
  rl.line = next.line;
  rl.cursor = next.cursor;
}

function startRepl(clusterIn: Cluster) {
  // Raw readline reads Ctrl+C as a key; this fires only while a child (also signalled) or y/N runs.
  process.on("SIGINT", () => {});
  let cluster = clusterIn;
  const rl = readline.createInterface({
    input: process.stdin,
    output: process.stdout,
    terminal: true,
    history: loadHistory(),
    historySize: 200,
  }) as RL;

  // Node skips its completer when TERM=dumb, and a normal terminal inserts a
  // raw tab unless a completer is set. Strip that tab and fill the word here
  // so both kinds of terminal behave the same.
  rl.input.on(
    "keypress",
    (_str: string, key: { name?: string; ctrl?: boolean; meta?: boolean }) => {
      if (!key || key.name !== "tab" || key.ctrl || key.meta) return;
      if (rl.cursor > 0 && rl.line[rl.cursor - 1] === "\t") {
        rl.line = rl.line.slice(0, rl.cursor - 1) + rl.line.slice(rl.cursor);
        rl.cursor -= 1;
      }
      fillTab(
        rl,
        () => cluster,
        (next) => {
          cluster = next;
        },
      );
      refreshLine(rl);
    },
  );

  const prompt = () => {
    rl.setPrompt(`${promptOf(cluster)}$ `);
  };
  process.stdout.write(
    `kube  live cluster ${promptOf(cluster)}. Tab completes real names. \`help\` lists commands.\n`,
  );
  prompt();
  rl.prompt();

  rl.on("line", (input) => {
    remember(input);
    rl.pause();
    if (process.stdin.isTTY) process.stdin.setRawMode(false);
    const result = runUserLine(input, cluster, host());
    if (process.stdin.isTTY) process.stdin.setRawMode(true);
    rl.resume();
    cluster = result.cluster;
    if (result.output === "\x1bc") {
      readline.cursorTo(process.stdout, 0, 0);
      readline.clearScreenDown(process.stdout);
    } else {
      writeResult(result);
    }
    if (result.exit) {
      rl.close();
      return;
    }
    prompt();
    rl.prompt();
  });

  rl.on("close", () => {
    process.stdout.write("\n");
    process.exit(0);
  });

  rl.on("SIGINT", () => {
    if (rl.line) {
      rl.line = "";
      rl.cursor = 0;
      process.stdout.write("\n");
      prompt();
      rl.prompt();
      return;
    }
    process.stdout.write("\n");
    process.exit(0);
  });
}

function boot(): Cluster {
  if (!probeLive()) {
    process.stderr.write(
      "kubectl could not reach a cluster. kube only runs against a reachable cluster.\n",
    );
    process.exit(1);
  }
  const cluster = liveCluster("kubectl");
  if (!cluster) {
    process.stderr.write(
      "kubectl could not reach a cluster. kube only runs against a reachable cluster.\n",
    );
    process.exit(1);
  }
  return cluster;
}

function takeFlags(argv: string[]): { yes: boolean; rest: string[] } {
  let yes = false;
  const rest: string[] = [];
  for (const arg of argv) {
    if (rest.length === 0 && arg === "--sample") {
      process.stderr.write(
        "sample mode was removed. kube only runs against a reachable cluster.\n",
      );
      process.exit(1);
    }
    if (rest.length === 0 && arg === "--live") continue;
    if (rest.length === 0 && (arg === "--yes" || arg === "-y")) {
      yes = true;
      continue;
    }
    rest.push(arg);
  }
  return { yes, rest };
}

function blankCluster(): Cluster {
  return {
    current: "",
    contexts: [],
    namespaces: [],
    nodes: [],
    pvs: [],
    events: [],
    objects: [],
  };
}

export function main() {
  const argv = process.argv.slice(2);
  if (argv[0] === "__complete") {
    const args = argv.slice(1);
    let cursor: number | undefined;
    let line = "";
    let asJson = false;
    for (let i = 0; i < args.length; i += 1) {
      const arg = args[i]!;
      if (arg === "--cursor") {
        cursor = Number(args[i + 1]);
        i += 1;
      } else if (arg === "--json") asJson = true;
      else if (arg === "--line") {
        line = args[i + 1] ?? "";
        break;
      }
    }
    const cluster = boot();
    printComplete(line, Number.isFinite(cursor) ? cursor! : line.length, cluster, asJson);
    return;
  }

  const { yes, rest } = takeFlags(argv);
  if (yes && !allowYes(yes, rest.length, process.stdin.isTTY === true)) {
    process.stderr.write("`--yes` applies to one command, not the shell.\n");
    process.exit(1);
  }
  session.yes = yes;
  const quoted: string[] = [];
  for (const arg of rest) {
    const next = quoteArg(arg);
    if (next === null) {
      process.stderr.write("cannot pass an argument that contains both quote styles\n");
      process.exit(1);
    }
    quoted.push(next);
  }
  const line = quoted.join(" ");
  const meta = metaOf(line);
  if (meta === "sample") {
    process.stderr.write("sample mode was removed. kube only runs against a reachable cluster.\n");
    process.exit(1);
  }
  if (
    meta === "help" ||
    meta === "--help" ||
    meta === "-h" ||
    meta === "--version" ||
    meta === "-v" ||
    meta === "completion"
  ) {
    const result = runUserLine(line, blankCluster(), host());
    writeResult(result);
    process.exit(result.ok ? 0 : 1);
  }
  const cluster = boot();

  if (rest.length === 0) {
    if (!process.stdin.isTTY) {
      let current = cluster;
      const text = fs.readFileSync(0, "utf8");
      let code = 0;
      for (const line of text.split("\n")) {
        if (!line.trim()) continue;
        const result = runUserLine(line, current, host());
        current = result.cluster;
        if (result.output !== "\x1bc") writeResult(result);
        if (!result.ok) code = 1;
        if (result.exit) break;
      }
      process.exit(code);
    }
    startRepl(cluster);
    return;
  }

  const inheritStdin = !process.stdin.isTTY;
  const result = runUserLine(line, cluster, host(), { inheritStdin });
  writeResult(result);
  process.exit(result.ok ? 0 : 1);
}
