#!/usr/bin/env python3
"""Drive the built `kube` CLI inside a real pseudo-terminal.

Usage: python3 scripts/e2e-tty.py [scenario ...]   (all scenarios when none are given)

Needs only `cli/kube.cjs` (run `npm run build` first), `scripts/fake-kubectl.mjs`, and `node`
on PATH. Standard library only; skips on Windows, which has no `pty` module.
"""

import json
import os
import shutil
import sys
import tempfile
import time

if os.name == "nt":
    print("e2e-tty: skipped (no pty on Windows)")
    sys.exit(0)

import pty  # noqa: E402
import select  # noqa: E402
import signal  # noqa: E402

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
KUBE = os.path.join(REPO, "cli", "kube.cjs")
FAKE_KUBECTL = os.path.join(REPO, "scripts", "fake-kubectl.mjs")
NODE = shutil.which("node")


class Failure(Exception):
    pass


class Result:
    def __init__(self, status, elapsed, output, calls, alive_at, output_at):
        self.status = status
        self.elapsed = elapsed
        self.output = output
        self.calls = calls
        self.verbs = [call[0] for call in calls if call]
        self.alive_at = alive_at
        self.output_at = output_at

    def describe(self):
        if self.status is None:
            return "still running at limit (killed by harness)"
        if os.WIFSIGNALED(self.status):
            return f"killed by signal {os.WTERMSIG(self.status)}"
        return f"exit {os.WEXITSTATUS(self.status)}"

    def exit_code(self):
        if self.status is None or not os.WIFEXITED(self.status):
            return None
        return os.WEXITSTATUS(self.status)

    def called_after(self, first, second):
        """True when kubectl argv `second` was logged after kubectl argv `first`."""
        if first not in self.calls:
            return False
        return second in self.calls[self.calls.index(first) + 1:]

    def tail(self, size=240):
        text = self.output.replace("\x1b", "^[").replace("\r", "")
        return text[-size:].replace("\n", " | ")


def environment(home):
    return dict(
        os.environ,
        HOME=home,
        USERPROFILE=home,
        TERM="xterm",
        KUBE_SHELL_KUBECTL=NODE,
        KUBE_SHELL_NODE_SCRIPT=FAKE_KUBECTL,
        KUBE_FAKE_FIXTURE="up",
        KUBE_FAKE_LOG=os.path.join(home, "kubectl.log"),
    )


def run(argv, steps=(), limit=8.0, probes=()):
    """Run `kube <argv>` in a pty.

    steps:  (delay_seconds, bytes) pairs; each delay counts from the previous write.
    probes: seconds after start at which to record whether the process is still alive
            (alive_at) and the output seen so far (output_at).
    """
    home = tempfile.mkdtemp(prefix="kube-tty-")
    pid = fd = None
    reaped = False
    try:
        env = environment(home)
        pid, fd = pty.fork()
        if pid == 0:
            try:
                os.execve(NODE, ["node", KUBE, *argv], env)
            finally:
                os._exit(127)
        start = time.time()
        output = b""
        pending = list(steps)
        next_at = start + pending[0][0] if pending else None
        checks = sorted(probes)
        alive_at = {}
        output_at = {}
        status = None
        while time.time() - start < limit:
            if next_at is not None and time.time() >= next_at:
                os.write(fd, pending.pop(0)[1])
                next_at = time.time() + pending[0][0] if pending else None
            ready, _, _ = select.select([fd], [], [], 0.05)
            if ready:
                try:
                    chunk = os.read(fd, 4096)
                except OSError:
                    chunk = b""
                output += chunk
            checked = time.time() - start
            done, st = os.waitpid(pid, os.WNOHANG)
            if done:
                status = st
                reaped = True
                break
            while checks and checked >= checks[0]:
                at = checks.pop(0)
                alive_at[at] = True
                output_at[at] = output.decode("utf8", "replace")
        elapsed = time.time() - start
        if not reaped:
            stop(pid)
            reaped = True
        output += drain(fd)
        text = output.decode("utf8", "replace")
        for at in checks:
            alive_at[at] = False
            output_at[at] = text
        return Result(status, elapsed, text, calls(env), alive_at, output_at)
    finally:
        if pid and not reaped:
            stop(pid)
        if fd is not None:
            os.close(fd)
        shutil.rmtree(home, ignore_errors=True)


def stop(pid):
    """Kill the child's whole process group (pty.fork made it the leader) and reap it."""
    try:
        os.killpg(pid, signal.SIGKILL)
    except ProcessLookupError:
        pass
    try:
        os.waitpid(pid, 0)
    except ChildProcessError:
        pass


def drain(fd):
    data = b""
    while True:
        try:
            ready, _, _ = select.select([fd], [], [], 0)
            if not ready:
                return data
            chunk = os.read(fd, 4096)
        except OSError:
            return data
        if not chunk:
            return data
        data += chunk


def calls(env):
    log = env["KUBE_FAKE_LOG"]
    if not os.path.exists(log):
        return []
    with open(log, encoding="utf8") as handle:
        return [json.loads(line) for line in handle if line.strip()]


def check(condition, reason, result):
    if not condition:
        raise Failure(f"{reason}; got {result.describe()} after {result.elapsed:.2f}s, "
                      f"kubectl verbs {sorted(set(result.verbs))}, output: {result.tail()}")


def confirm_waits():
    result = run(["delete", "pod", "checkout"], [(3.0, b"\r")], limit=8, probes=[2.5])
    check("Run it? [y/N]" in result.output_at[2.5], "expected the y/N prompt shown by 2.5s", result)
    check(result.alive_at[2.5], "expected the y/N prompt to still be waiting at 2.5s", result)
    check(result.exit_code() == 1, "expected exit 1 after Enter", result)
    check("not run" in result.output, "expected 'not run' in output", result)
    check("delete" not in result.verbs, "expected no kubectl delete", result)


def confirm_yes():
    result = run(["delete", "pod", "checkout"], [(1.0, b"y\r")], limit=8)
    check(result.exit_code() == 0, "expected exit 0 after answering y", result)
    check("delete" in result.verbs, "expected kubectl delete to run", result)


def view_idle():
    result = run(["view"], [(4.0, b"q")], limit=10, probes=[3.5])
    check("\x1b[?1049h" in result.output_at[3.5], "expected the view drawn by 3.5s", result)
    check(result.alive_at[3.5], "expected the view to still be open at 3.5s", result)
    check(result.exit_code() == 0, "expected exit 0 after q", result)
    check(result.output.rstrip().endswith("\x1b[?1049l"), "expected the alt screen restored",
          result)


def repl_sigint():
    steps = [(1.5, b"logs -f web\r"), (2.0, b"\x03"), (1.5, b"get ns\r"), (1.5, b"exit\r")]
    result = run([], steps, limit=12)
    check(not (result.status is not None and os.WIFSIGNALED(result.status)),
          "expected the shell to survive Ctrl+C", result)
    check(result.exit_code() == 0, "expected exit 0 after exit", result)
    check("following logs..." in result.output, "expected the logs -f output", result)
    check(result.called_after(["logs", "-f", "web"], ["get", "ns"]),
          "expected kubectl get ns to run after kubectl logs -f web", result)


def repl_prompt_ctrlc():
    steps = [(1.5, b"delete pod checkout\r"), (1.5, b"\x03"), (0.5, b"\r"), (1.5, b"exit\r")]
    result = run([], steps, limit=12)
    check(result.exit_code() == 0, "expected exit 0 after exit", result)
    check("not run" in result.output, "expected 'not run' in output", result)
    check("delete" not in result.verbs, "expected no kubectl delete", result)


def repl_tab():
    steps = [(1.5, b"get depl"), (0.5, b"\t"), (0.5, b"\r"), (1.5, b"exit\r")]
    result = run([], steps, limit=10)
    check("TypeError" not in result.output, "expected no crash on Tab", result)
    check(result.exit_code() == 0, "expected exit 0 after exit", result)
    check(["get", "deployments"] in result.calls,
          "expected Tab to complete depl to deployments before kubectl ran", result)


SCENARIOS = {
    "confirm-waits": confirm_waits,
    "confirm-yes": confirm_yes,
    "view-idle": view_idle,
    "repl-sigint": repl_sigint,
    "repl-prompt-ctrlc": repl_prompt_ctrlc,
    "repl-tab": repl_tab,
}


def main(names):
    if NODE is None:
        print("e2e-tty: node not found on PATH")
        return 1
    if not os.path.exists(KUBE):
        print(f"e2e-tty: {KUBE} is missing; run `npm run build` first")
        return 1
    unknown = [name for name in names if name not in SCENARIOS]
    if unknown:
        print(f"e2e-tty: unknown scenario(s) {', '.join(unknown)}; known: {', '.join(SCENARIOS)}")
        return 1
    for name in names or SCENARIOS:
        try:
            SCENARIOS[name]()
        except Failure as failure:
            print(f"{name}: FAIL - {failure}")
            return 1
        print(f"{name}: ok")
    print("e2e-tty: ok")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
