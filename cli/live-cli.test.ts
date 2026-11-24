import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import test from "node:test";

const root = process.cwd();
const fake = path.join(root, "scripts", "fake-kubectl.mjs");
const kube = path.join(root, "cli", "kube.cjs");

function spawnKube(args: string[], env: Record<string, string>) {
  const home = fs.mkdtempSync(path.join(os.tmpdir(), "kube-home-"));
  const log = path.join(home, "kubectl.log");
  const result = spawnSync(process.execPath, [kube, ...args], {
    cwd: root,
    encoding: "utf8",
    env: {
      ...process.env,
      HOME: home,
      USERPROFILE: home,
      KUBE_SHELL_KUBECTL: process.execPath,
      KUBE_SHELL_NODE_SCRIPT: fake,
      KUBE_FAKE_LOG: log,
      ...env,
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

test("plan get and plan drain do not run blast-radius gets", () => {
  const get = spawnKube(["plan", "get", "pods"], { KUBE_FAKE_FIXTURE: "up" });
  assert.equal(get.status, 0);
  assert.doesNotMatch(get.stdout ?? "", /blast/);
  assert.equal(get.lines.some(isBlastGet), false);
  const drain = spawnKube(["plan", "drain", "node-1"], { KUBE_FAKE_FIXTURE: "up" });
  assert.equal(drain.status, 0);
  assert.doesNotMatch(drain.stdout ?? "", /blast/);
  assert.equal(drain.lines.some(isBlastGet), false);
});

test("--version prints the package.json version", () => {
  const { version } = JSON.parse(fs.readFileSync(path.join(root, "package.json"), "utf8")) as {
    version: string;
  };
  const result = spawnKube(["--version"], { KUBE_FAKE_FIXTURE: "down" });
  assert.equal(result.status, 0);
  assert.equal((result.stdout ?? "").trim(), version);
});
