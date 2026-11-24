import { metaOf } from "../src/lib/kube/shell-meta";
import type { Cluster } from "../src/lib/kube/model";
import { completionScript } from "./completion";
import { toEngine } from "./engine";
import { advise, formatAdvice, formatPlan, planCommand } from "./insight";
import {
  applyLiveContext,
  invalidateLive,
  kubectlArgs,
  liveCluster,
  probeLive,
  runLive,
} from "./live";
import { boardFrom, openView } from "./view";

export type LineResult = { output: string; ok: boolean; cluster: Cluster; exit?: boolean };

export type LineHost = {
  yes: boolean;
  help: string;
  version: string;
  confirm: (prompt: string) => boolean;
};

export function runUserLine(
  line: string,
  cluster: Cluster,
  host: LineHost,
  options: { inheritStdin?: boolean } = {},
): LineResult {
  let trimmed = line.trim();
  const meta = metaOf(trimmed);
  if (meta === "exit" || meta === "quit") return { output: "", ok: true, cluster, exit: true };
  if (meta === "reset") {
    invalidateLive();
    const next = liveCluster("kubectl") ?? cluster;
    return { output: `refreshed names from ${next.current}`, ok: true, cluster: next };
  }
  if (meta === "sample") {
    return {
      output: "sample mode was removed. kube only runs against a reachable cluster.",
      ok: false,
      cluster,
    };
  }
  if (meta === "live") {
    invalidateLive();
    if (!probeLive())
      return {
        output: "No live cluster. Check kubectl and the current context.",
        ok: false,
        cluster,
      };
    const next = liveCluster("kubectl") ?? cluster;
    return { output: `live cluster ${next.current}`, ok: true, cluster: next };
  }
  if (meta === "clear") return { output: "\x1bc", ok: true, cluster };
  if (meta === "help" || meta === "--help" || meta === "-h")
    return { output: host.help, ok: true, cluster };
  if (meta === "--version" || meta === "-v") return { output: host.version, ok: true, cluster };
  if (meta === "completion") {
    const shell = trimmed.split(/\s+/)[1] ?? "";
    const script = completionScript(shell);
    if (!script) return { output: "usage: kube completion bash|zsh|fish", ok: false, cluster };
    return { output: script, ok: true, cluster };
  }
  if (meta === "view" || meta === "ui") {
    const parts = trimmed.split(/\s+/).slice(1);
    const all = parts.includes("-A") || parts.includes("--all-namespaces") || parts.includes("all");
    const word = parts.find((part) => part !== "all" && !part.startsWith("-"));
    const next = openView(cluster, {
      board: boardFrom(word),
      all,
      yes: host.yes,
      reload: (wantAll, namespace) => {
        invalidateLive();
        return liveCluster(wantAll ? "kubectl get pods -A" : `kubectl get pods -n ${namespace}`);
      },
    });
    return { output: "", ok: true, cluster: next };
  }
  if (meta === "plan") {
    const asJson = /(^|\s)--json(\s|$)/.test(trimmed);
    const body = trimmed
      .replace(/^plan\b\s*/, "")
      .replace(/(^|\s)--json(\s|$)/g, " ")
      .trim();
    if (!body) return { output: "usage: kube plan [--json] delete pod <name>", ok: false, cluster };
    const view = toEngine(body, body.length, "repl");
    const planned = planCommand(view.line, cluster);
    return { output: asJson ? JSON.stringify(planned) : formatPlan(planned), ok: true, cluster };
  }
  if (meta === "ask") {
    const asJson = /(^|\s)--json(\s|$)/.test(trimmed);
    const question = trimmed
      .replace(/^\?\s*/, "")
      .replace(/^ask\s+/, "")
      .replace(/(^|\s)--json(\s|$)/g, " ")
      .trim()
      .replace(/^["']|["']$/g, "");
    const advice = advise(question, cluster);
    if (asJson) return { output: JSON.stringify(advice), ok: true, cluster };
    if (!process.stdin.isTTY) return { output: formatAdvice(advice), ok: true, cluster };
    process.stderr.write(`${formatAdvice(advice)}\n`);
    if (!host.confirm("Run the first command? [y/N] ")) return { output: "", ok: true, cluster };
    const next = advice.commands[0];
    if (!next) return { output: "", ok: true, cluster };
    trimmed = next;
  }
  const view = toEngine(trimmed, trimmed.length, "repl");
  const planned = planCommand(view.line, cluster);
  if (planned.risk === "high" && !host.yes) {
    process.stderr.write(`${formatPlan(planned)}\n`);
    if (!host.confirm("Run it? [y/N] ")) return { output: "not run", ok: false, cluster };
  }
  const code = runLive(kubectlArgs(view.line), { inheritStdin: options.inheritStdin });
  return { output: "", ok: code === 0, cluster: applyLiveContext(cluster) };
}
