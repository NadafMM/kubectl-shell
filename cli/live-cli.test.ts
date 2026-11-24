import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import test from "node:test";

const root = process.cwd();
const fake = path.join(root, "scripts", "fake-kubectl.mjs");
const kube = path.join(root, "cli", "kube.cjs");

function spawnKube(args: string[], env: Record<string, string>, input?: string) {
  const { KUBE_TEST_HOME, ...rest } = env;
  const home = KUBE_TEST_HOME || fs.mkdtempSync(path.join(os.tmpdir(), "kube-home-"));
  const log = path.join(home, `kubectl-${process.hrtime.bigint()}.log`);
  const result = spawnSync(process.execPath, [kube, ...args], {
    cwd: root,
    encoding: "utf8",
    input,
    env: {
      ...process.env,
      HOME: home,
      USERPROFILE: home,
      KUBE_SHELL_KUBECTL: process.execPath,
      KUBE_SHELL_NODE_SCRIPT: fake,
      KUBE_FAKE_LOG: log,
      ...rest,
    },
  });
  const lines = fs.existsSync(log) ? fs.readFileSync(log, "utf8").split("\n").filter(Boolean) : [];
  return { ...result, home, log, lines };
}

function logHasDelete(lines: string[]): boolean {
  return lines.some((line) => {
    const argv = JSON.parse(line) as string[];
    return argv[0] === "delete";
  });
}

test("unreachable cluster exits 1 and does not mention sample mode as available", () => {
  const result = spawnKube(["get", "pods"], { KUBE_FAKE_FIXTURE: "down" });
  assert.equal(result.status, 1);
  assert.match(result.stderr ?? "", /kubectl could not reach a cluster/);
  assert.doesNotMatch(result.stderr ?? "", /--sample to practice/);
  assert.equal(result.log.startsWith(result.home), true);
});

test("--sample is rejected before any kubectl call", () => {
  const result = spawnKube(["--sample"], { KUBE_FAKE_FIXTURE: "up" });
  assert.equal(result.status, 1);
  assert.match(result.stderr ?? "", /sample mode was removed/);
  assert.equal(result.lines.length, 0);
});

test("--help exits 0 without kubectl", () => {
  const result = spawnKube(["--help"], { KUBE_FAKE_FIXTURE: "down" });
  assert.equal(result.status, 0);
  assert.match(result.stdout ?? "", /kube exits if kubectl cannot reach it/);
  assert.equal(result.lines.length, 0);
});

function logHasVerb(lines: string[], verb: string): boolean {
  return lines.some((line) => {
    const argv = JSON.parse(line) as string[];
    return argv[0] === verb;
  });
}

test("view prints a table when stdout is not a terminal", () => {
  const result = spawnKube(["view"], { KUBE_FAKE_FIXTURE: "up" });
  assert.equal(result.status, 0);
  assert.match(result.stdout ?? "", /NAME {2}STATUS/);
});

test("view deploy lists deployments from the live snapshot", () => {
  const result = spawnKube(["view", "deploy"], { KUBE_FAKE_FIXTURE: "up" });
  assert.equal(result.status, 0);
  assert.match(result.stdout ?? "", /web/);
});

test("view all lists pods in every namespace", () => {
  const result = spawnKube(["view", "all"], { KUBE_FAKE_FIXTURE: "up" });
  assert.equal(result.status, 0);
  assert.match(result.stdout ?? "", /NAMESPACE {2}NAME {2}STATUS/);
});

test("non-tty exec and cp are not run unless --yes", () => {
  const execRefused = spawnKube(["exec", "pod/api", "--", "date"], { KUBE_FAKE_FIXTURE: "up" });
  assert.equal(execRefused.status, 1);
  assert.match(`${execRefused.stdout ?? ""}\n${execRefused.stderr ?? ""}`, /not run/);
  assert.equal(logHasVerb(execRefused.lines, "exec"), false);
  const execAllowed = spawnKube(["--yes", "exec", "pod/api", "--", "date"], {
    KUBE_FAKE_FIXTURE: "up",
  });
  assert.equal(logHasVerb(execAllowed.lines, "exec"), true);
  const copied = spawnKube(["cp", "pod/api:/tmp/a", "./a"], { KUBE_FAKE_FIXTURE: "up" });
  assert.equal(copied.status, 1);
  assert.equal(logHasVerb(copied.lines, "cp"), false);
});

test("non-tty delete is not run unless --yes", () => {
  const refused = spawnKube(["delete", "pod", "checkout"], { KUBE_FAKE_FIXTURE: "up" });
  assert.equal(refused.status, 1);
  assert.match(refused.stderr ?? "", /not run/);
  assert.equal(refused.stdout, "");
  assert.equal(logHasDelete(refused.lines), false);
  const allowed = spawnKube(["--yes", "delete", "pod", "checkout"], { KUBE_FAKE_FIXTURE: "up" });
  assert.equal(logHasDelete(allowed.lines), true);
});

test("bare plan prints usage on stderr and exits 1", () => {
  const result = spawnKube(["plan"], { KUBE_FAKE_FIXTURE: "up" });
  assert.equal(result.status, 1);
  assert.equal(result.stdout, "");
  assert.match(result.stderr ?? "", /usage: kube plan/);
});

test("completion without a shell prints usage on stderr", () => {
  const result = spawnKube(["completion"], { KUBE_FAKE_FIXTURE: "down" });
  assert.equal(result.status, 1);
  assert.equal(result.stdout, "");
  assert.match(result.stderr ?? "", /usage: kube completion bash\|zsh\|fish/);
});

test("non-tty apply is not run unless --yes", () => {
  const refused = spawnKube(["apply", "-f", "deploy.yaml"], { KUBE_FAKE_FIXTURE: "up" });
  assert.equal(refused.status, 1);
  assert.match(`${refused.stdout ?? ""}\n${refused.stderr ?? ""}`, /not run/);
  assert.equal(logHasVerb(refused.lines, "apply"), false);
  const allowed = spawnKube(["--yes", "apply", "-f", "deploy.yaml"], { KUBE_FAKE_FIXTURE: "up" });
  assert.equal(logHasVerb(allowed.lines, "apply"), true);
});

test("non-tty port-forward and run are not run unless --yes", () => {
  const pf = spawnKube(["port-forward", "pod/api", "8080:80"], { KUBE_FAKE_FIXTURE: "up" });
  assert.equal(pf.status, 1);
  assert.equal(logHasVerb(pf.lines, "port-forward"), false);
  assert.equal(
    logHasVerb(
      spawnKube(["--yes", "port-forward", "pod/api", "8080:80"], { KUBE_FAKE_FIXTURE: "up" }).lines,
      "port-forward",
    ),
    true,
  );
  const runRefused = spawnKube(["run", "tmp", "--image=nginx"], { KUBE_FAKE_FIXTURE: "up" });
  assert.equal(runRefused.status, 1);
  assert.equal(logHasVerb(runRefused.lines, "run"), false);
});

function fileHome(): string {
  const file = path.join(fs.mkdtempSync(path.join(os.tmpdir(), "kube-file-home-")), "not-a-dir");
  fs.writeFileSync(file, "");
  return file;
}

test("an unwritable state directory does not stop the command", () => {
  const home = fileHome();
  const result = spawnKube(["get", "pods"], {
    KUBE_FAKE_FIXTURE: "up",
    HOME: home,
    USERPROFILE: home,
  });
  assert.equal(result.status, 0);
  assert.doesNotMatch(result.stderr ?? "", /ENOTDIR|EEXIST|at Object\./);
});

test("__complete works with an unwritable state directory", () => {
  const home = fileHome();
  const result = spawnKube(["__complete", "--line", "kube get po"], {
    KUBE_FAKE_FIXTURE: "up",
    HOME: home,
    USERPROFILE: home,
  });
  assert.equal(result.status, 0);
  assert.match(result.stdout ?? "", /^pods$/m);
});

test("KUBE_SHELL_REDACT_STDOUT redacts piped get secret -o json", () => {
  const result = spawnKube(["get", "secret", "token", "-o", "json"], {
    KUBE_FAKE_FIXTURE: "up",
    KUBE_SHELL_REDACT_STDOUT: "1",
  });
  assert.equal(result.status, 0);
  assert.match(result.stdout ?? "", /"\*\*\*"/);
  assert.doesNotMatch(result.stdout ?? "", /cGFzcw==/);
});

function isBlastGet(line: string): boolean {
  const argv = JSON.parse(line) as string[];
  const output = argv.indexOf("-o");
  return argv[0] === "get" && argv.includes("--request-timeout=4s") && argv[output + 1] === "json";
}

test("piped delete prints blast before the confirmation prompt", () => {
  const refused = spawnKube(["delete", "pod", "worker-0"], { KUBE_FAKE_FIXTURE: "up" });
  assert.equal(refused.status, 1);
  assert.equal(refused.stdout, "");
  const stderr = refused.stderr ?? "";
  const blastAt = stderr.indexOf("blast:");
  const promptAt = stderr.indexOf("Run it? [y/N]");
  assert.ok(blastAt >= 0 && promptAt > blastAt);
  assert.equal(logHasDelete(refused.lines), false);
});

test("plan --json delete includes blast", () => {
  const result = spawnKube(["plan", "--json", "delete", "pod", "worker-0"], {
    KUBE_FAKE_FIXTURE: "up",
  });
  assert.equal(result.status, 0);
  const parsed = JSON.parse(result.stdout ?? "") as {
    risk?: string;
    summary?: string;
    blast?: unknown;
  };
  assert.equal(parsed.risk, "high");
  assert.match(parsed.summary ?? "", /Deletes pod\/worker-0/);
  assert.ok(parsed.blast);
});

test("--yes delete does not run blast-radius gets", () => {
  const allowed = spawnKube(["--yes", "delete", "pod", "worker-0"], { KUBE_FAKE_FIXTURE: "up" });
  assert.equal(logHasDelete(allowed.lines), true);
  assert.equal(allowed.lines.some(isBlastGet), false);
});

test("plan get does not run blast-radius gets", () => {
  const get = spawnKube(["plan", "get", "pods"], { KUBE_FAKE_FIXTURE: "up" });
  assert.equal(get.status, 0);
  assert.doesNotMatch(get.stdout ?? "", /blast/);
  assert.equal(get.lines.some(isBlastGet), false);
});

test("plan drain prints a blast block", () => {
  const drain = spawnKube(["plan", "drain", "node-1"], { KUBE_FAKE_FIXTURE: "up" });
  assert.equal(drain.status, 0);
  assert.match(drain.stdout ?? "", /blast/);
  assert.equal(drain.lines.some(isBlastGet), true);
});

function writePolicy(home: string, policy: unknown) {
  const dir = path.join(home, ".kube-shell");
  fs.mkdirSync(dir, { recursive: true });
  fs.writeFileSync(path.join(dir, "policy.json"), JSON.stringify(policy));
}

test("typed context refuses y and bare --yes", () => {
  const home = fs.mkdtempSync(path.join(os.tmpdir(), "kube-policy-"));
  writePolicy(home, { contexts: { minikube: { confirm: "typed" } } });
  const env = { KUBE_FAKE_FIXTURE: "up", KUBE_TEST_HOME: home };
  const y = spawnKube(["delete", "pod", "worker-0"], env, "y\n");
  assert.equal(y.status, 1);
  assert.equal(logHasDelete(y.lines), false);
  const yes = spawnKube(["--yes", "delete", "pod", "worker-0"], env);
  assert.equal(yes.status, 1);
  assert.match(yes.stderr ?? "", /--confirm-context/);
  assert.equal(logHasDelete(yes.lines), false);
  const named = spawnKube(
    ["--yes", "--confirm-context", "minikube", "delete", "pod", "worker-0"],
    env,
  );
  assert.equal(logHasDelete(named.lines), true);
});

test("an agent plan id is single use and argv-bound", () => {
  const home = fs.mkdtempSync(path.join(os.tmpdir(), "kube-agent-"));
  writePolicy(home, {
    agents: { allow: ["get", "describe", "logs", "plan"], write_requires: "approved-plan" },
  });
  const env = {
    KUBE_FAKE_FIXTURE: "up",
    KUBE_TEST_HOME: home,
    KUBE_SHELL_AGENT: "1",
    KUBE_SHELL_POLICY: path.join(home, "ignored.json"),
  };
  const bare = spawnKube(["--yes", "delete", "pod", "worker-0"], env);
  assert.equal(bare.status, 1);
  assert.equal(logHasDelete(bare.lines), false);
  const planned = spawnKube(["plan", "--json", "delete", "pod", "worker-0"], env);
  assert.equal(planned.status, 0);
  const planId = (JSON.parse(planned.stdout ?? "") as { planId?: string }).planId ?? "";
  assert.ok(planId);
  const wrong = spawnKube(["--yes", "--plan-id", planId, "delete", "pod", "other"], env);
  assert.equal(wrong.status, 1);
  assert.equal(logHasDelete(wrong.lines), false);
  const once = spawnKube(["--yes", "--plan-id", planId, "delete", "pod", "worker-0"], env);
  assert.equal(logHasDelete(once.lines), true);
  const replay = spawnKube(["--yes", "--plan-id", planId, "delete", "pod", "worker-0"], env);
  assert.equal(replay.status, 1);
  assert.equal(logHasDelete(replay.lines), false);
});

test("ns in the shell does not rewrite kubeconfig", () => {
  const home = fs.mkdtempSync(path.join(os.tmpdir(), "kube-ns-"));
  const kubeconfig = path.join(home, "kube.config");
  fs.writeFileSync(kubeconfig, "untouched");
  const result = spawnKube(
    [],
    { KUBE_FAKE_FIXTURE: "up", KUBE_TEST_HOME: home, KUBECONFIG: kubeconfig },
    "ns payments\nget pods\n",
  );
  assert.equal(result.status, 0);
  assert.equal(fs.readFileSync(kubeconfig, "utf8"), "untouched");
  const got = result.lines.some((line) => {
    const argv = JSON.parse(line) as string[];
    return argv[0] === "get" && argv.includes("-n") && argv[argv.indexOf("-n") + 1] === "payments";
  });
  assert.equal(got, true);
});

test("a denied command is audited without an exit code", () => {
  const home = fs.mkdtempSync(path.join(os.tmpdir(), "kube-audit-deny-"));
  writePolicy(home, { contexts: { "mini*": { deny: ["exec"] } } });
  const result = spawnKube(["exec", "pod/api", "--", "date"], {
    KUBE_FAKE_FIXTURE: "up",
    KUBE_TEST_HOME: home,
  });
  assert.equal(result.status, 1);
  assert.equal(logHasVerb(result.lines, "exec"), false);
  const audit = fs.readFileSync(path.join(home, ".kube-shell", "audit.jsonl"), "utf8");
  const row = JSON.parse(audit.trim().split("\n").at(-1) ?? "{}") as {
    decision?: string;
    exitCode?: number | null;
  };
  assert.equal(row.decision, "denied-policy");
  assert.equal(row.exitCode, null);
});

test("an approved plan records its plan id and redacts literals", () => {
  const home = fs.mkdtempSync(path.join(os.tmpdir(), "kube-audit-plan-"));
  writePolicy(home, {
    agents: { allow: ["get", "describe", "logs", "plan"], write_requires: "approved-plan" },
  });
  const env = { KUBE_FAKE_FIXTURE: "up", KUBE_TEST_HOME: home, KUBE_SHELL_AGENT: "1" };
  const planned = spawnKube(
    ["plan", "--json", "create", "secret", "generic", "t", "--from-literal=password=secret"],
    env,
  );
  const planId = (JSON.parse(planned.stdout ?? "") as { planId?: string }).planId ?? "";
  const ran = spawnKube(
    [
      "--yes",
      "--plan-id",
      planId,
      "create",
      "secret",
      "generic",
      "t",
      "--from-literal=password=secret",
    ],
    env,
  );
  assert.equal(ran.status, 0);
  const audit = fs.readFileSync(path.join(home, ".kube-shell", "audit.jsonl"), "utf8");
  assert.doesNotMatch(audit, /password=secret/);
  assert.match(audit, /password=\*\*\*/);
  assert.match(audit, new RegExp(`"approver":"plan-id:${planId}"`));
});

test("undo rolls a deployment back and refuses a Secret", () => {
  const home = fs.mkdtempSync(path.join(os.tmpdir(), "kube-undo-"));
  const env = { KUBE_FAKE_FIXTURE: "up", KUBE_FAKE_UNDO: "1", KUBE_TEST_HOME: home };
  const changed = spawnKube(["set", "image", "deployment/checkout", "nginx=nginx:1.2"], env);
  assert.equal(changed.status, 0);
  const saved = fs
    .readdirSync(path.join(home, ".kube-shell", "undo"))
    .find((name) => name.endsWith(".json"));
  assert.ok(saved);
  const snapshot = fs.readFileSync(path.join(home, ".kube-shell", "undo", saved ?? ""), "utf8");
  assert.doesNotMatch(snapshot, /resourceVersion/);
  assert.doesNotMatch(snapshot, /"status"/);
  const restored = spawnKube(["--yes", "undo"], env);
  assert.equal(restored.status, 0);
  const rolled = restored.lines.some((line) => {
    const argv = JSON.parse(line) as string[];
    return argv[0] === "rollout" && argv.includes("--to-revision=3");
  });
  assert.equal(rolled, true);
  const secret = spawnKube(["label", "secret", "token", "env=prod"], env);
  assert.equal(secret.status, 0);
  const refused = spawnKube(["--yes", "undo"], env);
  assert.equal(refused.status, 1);
  assert.match(refused.stderr ?? "", /Secrets or ConfigMaps/);
  assert.equal(
    refused.lines.some((line) => (JSON.parse(line) as string[])[0] === "apply"),
    false,
  );
});

test("an unknown leading flag is an error", () => {
  const result = spawnKube(["--nope", "get", "pods"], { KUBE_FAKE_FIXTURE: "down" });
  assert.equal(result.status, 1);
  assert.match(result.stderr ?? "", /unknown flag/);
});

test("ask --json reports an OOMKilled pod from kubectl", () => {
  const result = spawnKube(["ask", "--json", "why is checkout crashing"], {
    KUBE_FAKE_FIXTURE: "up",
    KUBE_FAKE_DIAG: "oom",
  });
  assert.equal(result.status, 0);
  const body = JSON.parse(result.stdout ?? "") as {
    findings: { cause: string; commands: string[] }[];
  };
  assert.match(body.findings[0]?.cause ?? "", /OOMKilled/);
  assert.match(body.findings[0]?.cause ?? "", /memory limit 64Mi/);
  assert.equal(JSON.stringify(body).includes("password=secret"), false);
  const logged = result.lines.map((line) => JSON.parse(line) as string[]);
  assert.equal(
    logged.some((argv) => argv[0] === "get" && (argv[1] === "pod" || argv[1] === "pods")),
    true,
  );
});

test("ask about port-forward does not read pod logs", () => {
  const result = spawnKube(["ask", "how do I port-forward"], {
    KUBE_FAKE_FIXTURE: "up",
    KUBE_FAKE_DIAG: "oom",
  });
  assert.equal(result.status, 0);
  assert.match(result.stdout ?? "", /port-forward/);
  const logged = result.lines.map((line) => JSON.parse(line) as string[]);
  assert.equal(
    logged.some((argv) => argv[0] === "logs"),
    false,
  );
});

test("--version prints the package.json version", () => {
  const { version } = JSON.parse(fs.readFileSync(path.join(root, "package.json"), "utf8")) as {
    version: string;
  };
  const result = spawnKube(["--version"], { KUBE_FAKE_FIXTURE: "down" });
  assert.equal(result.status, 0);
  assert.equal((result.stdout ?? "").trim(), version);
});
