import { execFileSync, spawnSync } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";

export const SANDBOX_CLUSTER = "kube-shell-sandbox";

export const SANDBOX_SEED = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
  namespace: default
spec:
  replicas: 2
  selector:
    matchLabels:
      app: demo
  template:
    metadata:
      labels:
        app: demo
    spec:
      containers:
        - name: app
          image: nginx:1.27
          ports:
            - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: demo
  namespace: default
spec:
  selector:
    app: demo
  ports:
    - port: 80
      targetPort: 80
---
apiVersion: v1
kind: Pod
metadata:
  name: broken
  namespace: default
spec:
  containers:
    - name: app
      image: kube-shell/missing:none
`;

export function sandboxKubeconfig(home: string = os.homedir()): string {
  return path.join(home, ".kube-shell", "sandbox", "kubeconfig");
}

export type SandboxCommand = {
  cmd: string;
  args: string[];
  env: NodeJS.ProcessEnv;
  input?: string;
};

export type SandboxRun = (command: SandboxCommand) => {
  code: number;
  stdout: string;
  stderr: string;
};

export type SandboxHost = {
  home: string;
  run: SandboxRun;
  has: (name: string) => boolean;
};

function childEnv(kubeconfig: string): NodeJS.ProcessEnv {
  return { ...process.env, KUBECONFIG: kubeconfig };
}

function kindCmd(): string {
  return process.env.KUBE_SHELL_KIND || "kind";
}

function kubectlCmd(): string {
  return process.env.KUBE_SHELL_KUBECTL || "kubectl";
}

function tighten(file: string, mode: number): void {
  try {
    fs.chmodSync(file, mode);
  } catch {
    /* Windows permissions are not POSIX modes. */
  }
}

export function sandboxUp(host: SandboxHost): { ok: boolean; message: string } {
  if (!host.has("kind") || !host.has("docker")) {
    return { ok: false, message: "kube sandbox needs kind and Docker on PATH" };
  }
  const kubeconfig = sandboxKubeconfig(host.home);
  fs.mkdirSync(path.dirname(kubeconfig), { recursive: true, mode: 0o700 });
  tighten(path.dirname(kubeconfig), 0o700);
  tighten(path.dirname(path.dirname(kubeconfig)), 0o700);
  const created = host.run({
    cmd: kindCmd(),
    args: ["create", "cluster", "--name", SANDBOX_CLUSTER, "--kubeconfig", kubeconfig],
    env: childEnv(kubeconfig),
  });
  if (created.code !== 0) {
    return {
      ok: false,
      message: (created.stderr || created.stdout || "kind create failed").trim(),
    };
  }
  tighten(kubeconfig, 0o600);
  const applied = host.run({
    cmd: kubectlCmd(),
    args: ["apply", "-f", "-"],
    env: childEnv(kubeconfig),
    input: SANDBOX_SEED,
  });
  if (applied.code !== 0) {
    const detail = (applied.stderr || applied.stdout || "kubectl apply failed").trim();
    return {
      ok: false,
      message: `sandbox cluster was created, but the demo workload was not applied\n${detail}`,
    };
  }
  return { ok: true, message: `sandbox ${SANDBOX_CLUSTER} is up\n${kubeconfig}` };
}

function clusterGone(result: { stdout: string; stderr: string }): boolean {
  return /not found|no kind cluster|failed to find|unknown cluster/i.test(
    `${result.stderr}\n${result.stdout}`,
  );
}

export function sandboxDown(host: SandboxHost): { ok: boolean; message: string } {
  const kubeconfig = sandboxKubeconfig(host.home);
  const deleted = host.run({
    cmd: kindCmd(),
    args: ["delete", "cluster", "--name", SANDBOX_CLUSTER],
    env: childEnv(kubeconfig),
  });
  if (deleted.code !== 0 && !clusterGone(deleted)) {
    return {
      ok: false,
      message: (deleted.stderr || deleted.stdout || "kind delete failed").trim(),
    };
  }
  fs.rmSync(kubeconfig, { force: true });
  return { ok: true, message: `sandbox ${SANDBOX_CLUSTER} is down` };
}

export function commandOnPath(name: string): boolean {
  if (name === "kind" && process.env.KUBE_SHELL_KIND) return true;
  if (name === "docker" && process.env.KUBE_SHELL_DOCKER) return true;
  const tool = process.platform === "win32" ? "where.exe" : "which";
  const result = spawnSync(tool, [name], { encoding: "utf8", windowsHide: true });
  return result.status === 0;
}

function launch(command: SandboxCommand): { code: number; stdout: string; stderr: string } {
  const scripted = command.cmd.endsWith(".mjs") || command.cmd.endsWith(".cjs");
  const cmd = scripted ? process.execPath : command.cmd;
  const args = scripted ? [command.cmd, ...command.args] : [...command.args];
  const nodeScript = process.env.KUBE_SHELL_NODE_SCRIPT;
  const kubectlArgs =
    command.args[0] === "apply" && nodeScript && !scripted ? [nodeScript, ...args] : args;
  try {
    const stdout = execFileSync(cmd, kubectlArgs, {
      encoding: "utf8",
      env: command.env,
      input: command.input,
      maxBuffer: 8 * 1024 * 1024,
      stdio: ["pipe", "pipe", "pipe"],
    });
    return { code: 0, stdout, stderr: "" };
  } catch (err) {
    const failed = err as { status?: number; stdout?: string | Buffer; stderr?: string | Buffer };
    const text = (value: string | Buffer | undefined) =>
      typeof value === "string" ? value : value ? value.toString("utf8") : "";
    return {
      code: typeof failed.status === "number" ? failed.status : 1,
      stdout: text(failed.stdout),
      stderr: text(failed.stderr),
    };
  }
}

export function realSandbox(home = os.homedir()): SandboxHost {
  return { home, run: launch, has: commandOnPath };
}
