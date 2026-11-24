export type FindingRule = "oom" | "crash" | "image" | "config" | "pending" | "probe";

export type Finding = {
  rule: FindingRule;
  cause: string;
  commands: string[];
};

export type Diagnosis = {
  pod: string;
  namespace: string;
  findings: Finding[];
};

type Dict = Record<string, unknown>;

const SENSITIVE = /(password|token|secret|apikey|api-key|client-secret|client_secret)=\S+/gi;

function obj(value: unknown): Dict | null {
  if (!value || typeof value !== "object" || Array.isArray(value)) return null;
  return value as Dict;
}

function str(value: unknown): string {
  return typeof value === "string" ? value : "";
}

function num(value: unknown): number | undefined {
  return typeof value === "number" ? value : undefined;
}

function list(value: unknown): Dict[] {
  if (!Array.isArray(value)) return [];
  return value.flatMap((item) => {
    const row = obj(item);
    return row ? [row] : [];
  });
}

function logTail(text: string): string {
  const lines = text
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);
  return lines
    .slice(-30)
    .map((line) => line.replace(SENSITIVE, (match) => `${match.slice(0, match.indexOf("="))}=***`))
    .join(" | ");
}

function containersOf(pod: Dict): Dict[] {
  const spec = obj(pod.spec) ?? {};
  return [...list(spec.containers), ...list(spec.initContainers)];
}

function statusesOf(pod: Dict): Dict[] {
  const status = obj(pod.status) ?? {};
  return [...list(status.containerStatuses), ...list(status.initContainerStatuses)];
}

function containerNamed(pod: Dict, name: string): Dict | undefined {
  return containersOf(pod).find((item) => str(item.name) === name);
}

function terminated(status: Dict): Dict | null {
  return obj(obj(status.lastState)?.terminated) ?? obj(obj(status.state)?.terminated);
}

function waiting(status: Dict): Dict | null {
  return obj(obj(status.state)?.waiting);
}

function isOom(term: Dict | null): boolean {
  if (!term) return false;
  if (str(term.reason) === "OOMKilled") return true;
  return num(term.exitCode) === 137;
}

function memoryLimit(pod: Dict, name: string): string {
  const limits = obj(obj(containerNamed(pod, name)?.resources)?.limits);
  return str(limits?.memory) || "none";
}

function imageOf(pod: Dict, name: string): string {
  return str(containerNamed(pod, name)?.image) || "unknown";
}

function describe(pod: string, ns: string): string {
  return `describe pod ${pod} -n ${ns}`;
}

function eventRows(events: unknown[], podName: string): Dict[] {
  return events.flatMap((event) => {
    const item = obj(event);
    if (!item) return [];
    const involved = str(obj(item.involvedObject)?.name);
    if (involved && involved !== podName) return [];
    return [item];
  });
}

function messagesOf(rows: Dict[]): string[] {
  return rows.map((row) => str(row.message)).filter(Boolean);
}

function pullKind(message: string): "auth" | "tag not found" | "registry timeout" | "pull failed" {
  if (/unauthorized|authentication|forbidden|\b401\b|\b403\b/i.test(message)) return "auth";
  if (/not found|manifest unknown|\b404\b/i.test(message)) return "tag not found";
  if (/timed out|timeout|i\/o timeout/i.test(message)) return "registry timeout";
  return "pull failed";
}

function shortMessage(message: string): string {
  const line =
    message
      .split(/\r?\n/)
      .map((item) => item.trim())
      .find(Boolean) ?? "";
  return line.slice(0, 160);
}

type Ref = { kind: "Secret" | "ConfigMap"; name: string; key?: string };

function specRef(pod: Dict, containerName: string): Ref | null {
  const container = containerNamed(pod, containerName);
  for (const env of list(container?.env)) {
    const from = obj(env.valueFrom);
    const secret = obj(from?.secretKeyRef);
    if (str(secret?.name))
      return { kind: "Secret", name: str(secret?.name), key: str(secret?.key) || undefined };
    const config = obj(from?.configMapKeyRef);
    if (str(config?.name)) {
      return { kind: "ConfigMap", name: str(config?.name), key: str(config?.key) || undefined };
    }
  }
  for (const from of list(container?.envFrom)) {
    if (str(obj(from.secretRef)?.name))
      return { kind: "Secret", name: str(obj(from.secretRef)?.name) };
    if (str(obj(from.configMapRef)?.name)) {
      return { kind: "ConfigMap", name: str(obj(from.configMapRef)?.name) };
    }
  }
  for (const volume of list(obj(pod.spec)?.volumes)) {
    if (str(obj(volume.secret)?.secretName))
      return { kind: "Secret", name: str(obj(volume.secret)?.secretName) };
    if (str(obj(volume.configMap)?.name))
      return { kind: "ConfigMap", name: str(obj(volume.configMap)?.name) };
  }
  return null;
}

function configRef(pod: Dict, containerName: string, message: string): Ref | null {
  const keySecret = message.match(/key\s+(\S+)\s+in\s+Secret\s+(?:\S+\/)?([A-Za-z0-9.-]+)/i);
  if (keySecret?.[1] && keySecret[2]) {
    return { kind: "Secret", name: keySecret[2], key: keySecret[1].replace(/[.,]$/, "") };
  }
  const keyConfig = message.match(/key\s+(\S+)\s+in\s+ConfigMap\s+(?:\S+\/)?([A-Za-z0-9.-]+)/i);
  if (keyConfig?.[1] && keyConfig[2]) {
    return { kind: "ConfigMap", name: keyConfig[2], key: keyConfig[1].replace(/[.,]$/, "") };
  }
  const secretMissing = message.match(/secret\s+"([^"]+)"\s+not found/i);
  if (secretMissing?.[1]) return { kind: "Secret", name: secretMissing[1] };
  const configMissing = message.match(/configmap\s+"([^"]+)"\s+not found/i);
  if (configMissing?.[1]) return { kind: "ConfigMap", name: configMissing[1] };
  return specRef(pod, containerName);
}

function isPending(pod: Dict): boolean {
  const status = obj(pod.status);
  if (str(status?.phase) === "Pending") return true;
  return list(status?.conditions).some(
    (condition) => str(condition.type) === "PodScheduled" && str(condition.status) === "False",
  );
}

function claimName(pod: Dict, messages: string[]): string {
  for (const volume of list(obj(pod.spec)?.volumes)) {
    const name = str(obj(volume.persistentVolumeClaim)?.claimName);
    if (name) return name;
  }
  const blob = messages.join("\n");
  const quoted = blob.match(/persistentvolumeclaim\s+"([^"]+)"/i);
  return quoted?.[1] || "unknown";
}

function pendingCause(pod: Dict, messages: string[]): string {
  const blob = messages.join("\n");
  if (/insufficient cpu/i.test(blob)) return "Pending: insufficient cpu";
  if (/insufficient memory/i.test(blob)) return "Pending: insufficient memory";
  if (/persistentvolumeclaim|unbound immediate/i.test(blob)) {
    return `Pending: unbound PersistentVolumeClaim ${claimName(pod, messages)}`;
  }
  if (/taint/i.test(blob)) return "Pending: node taint";
  if (/affinity/i.test(blob)) return "Pending: node affinity mismatch";
  return "Pending: unschedulable";
}

function probeTarget(pod: Dict, kind: "readiness" | "liveness"): string {
  const field = kind === "readiness" ? "readinessProbe" : "livenessProbe";
  const containers = containersOf(pod);
  const container = containers.find((item) => obj(item[field])) ?? containers[0];
  const probe = obj(container?.[field]);
  if (!probe) return "probe";
  const http = obj(probe.httpGet);
  if (http) {
    const path = str(http.path) || "/";
    return `http://:${http.port ?? ""}${path.startsWith("/") ? path : `/${path}`}`;
  }
  const tcp = obj(probe.tcpSocket);
  if (tcp) return `tcp:${tcp.port ?? ""}`;
  const execProbe = obj(probe.exec);
  if (execProbe) {
    const command = Array.isArray(execProbe.command)
      ? execProbe.command.map(str).filter(Boolean).join(" ")
      : "";
    return command ? `exec ${command}` : "exec";
  }
  const grpc = obj(probe.grpc);
  if (grpc) return `grpc:${grpc.port ?? ""}`;
  return "probe";
}

/** Rank up to three causes for one pod. No kubectl and no network. */
export function diagnosePod(
  pod: unknown,
  events: unknown[] = [],
  previousLogs: Record<string, string> = {},
): Diagnosis | null {
  const body = obj(pod);
  const name = str(obj(body?.metadata)?.name);
  if (!body || !name) return null;
  const ns = str(obj(body.metadata)?.namespace) || "default";
  const rows = eventRows(events, name);
  const findings: Finding[] = [];

  for (const status of statusesOf(body)) {
    const container = str(status.name) || "container";
    const term = terminated(status);
    if (!isOom(term)) continue;
    const exit = num(term?.exitCode) ?? 137;
    findings.push({
      rule: "oom",
      cause: `container ${container} was OOMKilled (exit ${exit}); memory limit ${memoryLimit(body, container)}`,
      commands: [describe(name, ns), `logs ${name} -c ${container} --previous -n ${ns}`],
    });
  }

  for (const status of statusesOf(body)) {
    const container = str(status.name) || "container";
    const term = terminated(status);
    if (isOom(term)) continue;
    const reason = str(waiting(status)?.reason);
    const exit = num(term?.exitCode);
    const crashed = reason === "CrashLoopBackOff" || (exit !== undefined && exit !== 0);
    if (!crashed) continue;
    const tail = logTail(previousLogs[container] ?? "");
    const code = exit === undefined ? "unknown" : String(exit);
    findings.push({
      rule: "crash",
      cause: tail
        ? `container ${container} exited ${code}; ${tail}`
        : `container ${container} exited ${code}`,
      commands: [`logs ${name} -c ${container} --previous -n ${ns}`],
    });
  }

  for (const status of statusesOf(body)) {
    const container = str(status.name) || "container";
    const wait = waiting(status);
    const reason = str(wait?.reason);
    if (reason !== "ImagePullBackOff" && reason !== "ErrImagePull") continue;
    const eventMessage = messagesOf(rows).find((message) =>
      /pull|manifest|unauthorized|timeout|not found/i.test(message),
    );
    const message = eventMessage || str(wait?.message);
    const shown = shortMessage(message);
    findings.push({
      rule: "image",
      cause: `image ${imageOf(body, container)} failed to pull (${pullKind(message)})${shown ? `; ${shown}` : ""}`,
      commands: [describe(name, ns)],
    });
  }

  for (const status of statusesOf(body)) {
    const container = str(status.name) || "container";
    const wait = waiting(status);
    if (str(wait?.reason) !== "CreateContainerConfigError") continue;
    const ref = configRef(body, container, `${str(wait?.message)}\n${messagesOf(rows).join("\n")}`);
    const kind = ref?.kind ?? "Secret";
    const resource = kind === "Secret" ? "secret" : "configmap";
    const what = ref
      ? `missing ${kind} ${ref.name}${ref.key ? ` key ${ref.key}` : ""}`
      : "missing Secret or ConfigMap";
    findings.push({
      rule: "config",
      cause: what,
      commands: [describe(name, ns), ...(ref ? [`get ${resource} ${ref.name} -n ${ns}`] : [])],
    });
  }

  const unscheduled = list(obj(body.status)?.conditions).some(
    (condition) => str(condition.type) === "PodScheduled" && str(condition.status) === "False",
  );
  const schedulingEvent = rows.some((row) => str(row.reason) === "FailedScheduling");
  if (unscheduled || schedulingEvent) {
    const status = obj(body.status);
    const conditionMessages = list(status?.conditions)
      .filter((condition) => str(condition.type) === "PodScheduled")
      .map((condition) => str(condition.message))
      .filter(Boolean);
    findings.push({
      rule: "pending",
      cause: pendingCause(body, [...messagesOf(rows), ...conditionMessages]),
      commands: [describe(name, ns), `get events -n ${ns}`],
    });
  }

  const unhealthy = rows.filter((row) => str(row.reason) === "Unhealthy");
  if (unhealthy.length > 0) {
    const messages = messagesOf(unhealthy);
    const live = messages.filter((message) => /liveness/i.test(message)).length;
    const ready = messages.filter((message) => /readiness/i.test(message)).length;
    const kind = live > ready ? "liveness" : "readiness";
    const label =
      unhealthy.length === 1 ? "1 Unhealthy warning" : `${unhealthy.length} Unhealthy warnings`;
    findings.push({
      rule: "probe",
      cause: `${kind} probe ${probeTarget(body, kind)} failed (${label})`,
      commands: [describe(name, ns)],
    });
  }

  const kept = findings.slice(0, 3);
  if (kept.length === 0) return null;
  return { pod: name, namespace: ns, findings: kept };
}

const HEALTH =
  /crash|backoff|restart|oom|not starting|failing|pending|unschedul|cannot schedule|can't schedule|insufficient|imagepull|errimage|pull back|registry|not ready|unhealthy|readiness|liveness|probe|createcontainer|configerror/i;

const STOP = new Set([
  "why",
  "is",
  "the",
  "a",
  "an",
  "pod",
  "pods",
  "how",
  "do",
  "i",
  "what",
  "my",
  "this",
  "that",
  "in",
  "on",
  "for",
  "to",
  "and",
  "or",
  "it",
  "its",
  "of",
  "with",
  "was",
  "are",
  "not",
  "from",
  "crashing",
  "crash",
  "pending",
  "failing",
  "failed",
  "unhealthy",
  "restarting",
  "down",
  "stuck",
  "oom",
  "backoff",
  "imagepull",
  "probe",
]);

export function podNameFrom(question: string, knownNames: string[] = []): string | undefined {
  const slash = question.match(/\bpod\/([a-z0-9][-a-z0-9.]*)/i);
  if (slash?.[1]) return slash[1];
  const ranked = knownNames.filter((name) => name.length >= 3).sort((a, b) => b.length - a.length);
  const q = question.toLowerCase();
  for (const known of ranked) {
    if (q.includes(known.toLowerCase())) return known;
  }
  const explicit = question.match(/\bpod\s+([a-z0-9][-a-z0-9.]*)/i);
  if (explicit?.[1] && !STOP.has(explicit[1].toLowerCase())) return explicit[1];
  return undefined;
}

export function adviceFrom(diagnosis: Diagnosis): {
  why: string;
  commands: string[];
  findings: Finding[];
} {
  const commands: string[] = [];
  for (const finding of diagnosis.findings) {
    for (const command of finding.commands) {
      if (!commands.includes(command)) commands.push(command);
    }
  }
  return {
    why: diagnosis.findings.map((finding) => finding.cause).join("\n"),
    commands,
    findings: diagnosis.findings,
  };
}

export type FetchJson = (args: string[]) => { ok: boolean; json: unknown };
export type FetchText = (args: string[]) => string;

function itemsOf(json: unknown): unknown[] {
  if (Array.isArray(json)) return json;
  const body = obj(json);
  return Array.isArray(body?.items) ? body.items : [];
}

function needsPreviousLog(status: Dict): boolean {
  const term = terminated(status);
  if (isOom(term)) return false;
  const exit = num(term?.exitCode);
  return str(waiting(status)?.reason) === "CrashLoopBackOff" || (exit !== undefined && exit !== 0);
}

function candidate(pod: unknown): boolean {
  const body = obj(pod);
  if (!body) return false;
  if (isPending(body)) return true;
  return statusesOf(body).some((status) => {
    if (status.ready === false) return true;
    if (str(waiting(status)?.reason)) return true;
    const exit = num(terminated(status)?.exitCode);
    return exit !== undefined && exit !== 0;
  });
}

function eventsFor(name: string, ns: string, fetchJson: FetchJson): unknown[] {
  const result = fetchJson([
    "get",
    "events",
    "-n",
    ns,
    "--field-selector",
    `involvedObject.name=${name}`,
    "-o",
    "json",
  ]);
  if (!result.ok) return [];
  return itemsOf(result.json);
}

function previousLogs(pod: Dict, ns: string, fetchText: FetchText): Record<string, string> {
  const logs: Record<string, string> = {};
  const podName = str(obj(pod.metadata)?.name);
  for (const status of statusesOf(pod)) {
    const container = str(status.name);
    if (!podName || !container || !needsPreviousLog(status)) continue;
    logs[container] = fetchText(["logs", podName, "-n", ns, "-c", container, "--previous"]);
  }
  return logs;
}

function diagnoseFetched(
  pod: unknown,
  ns: string,
  fetchJson: FetchJson,
  fetchText: FetchText,
): Diagnosis | null {
  const body = obj(pod);
  const name = str(obj(body?.metadata)?.name);
  if (!body || !name || !candidate(body)) return null;
  return diagnosePod(body, eventsFor(name, ns, fetchJson), previousLogs(body, ns, fetchText));
}

/** Fetch at most one named pod, or the first 10 pods, and rank causes. */
export function inspectPods(input: {
  question: string;
  namespace: string;
  knownNames?: string[];
  fetchJson: FetchJson;
  fetchText: FetchText;
}): Diagnosis | null {
  if (!HEALTH.test(input.question)) return null;
  const ns = input.namespace || "default";
  const named = podNameFrom(input.question, input.knownNames ?? []);
  if (named) {
    const got = input.fetchJson(["get", "pod", named, "-n", ns, "-o", "json"]);
    if (!got.ok || got.json == null) return null;
    return diagnoseFetched(got.json, ns, input.fetchJson, input.fetchText);
  }
  const list = input.fetchJson(["get", "pods", "-n", ns, "-o", "json"]);
  if (!list.ok) return null;
  for (const pod of itemsOf(list.json).slice(0, 10)) {
    const found = diagnoseFetched(pod, ns, input.fetchJson, input.fetchText);
    if (found) return found;
  }
  return null;
}
