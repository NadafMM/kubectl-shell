# kube threat model

`kube` is a terminal wrapper around **your** `kubectl` binary and **your** kubeconfig. It does not add a network service or a second identity on your real cluster.

## Trust assumptions

- You trust the machine, your kubeconfig, and cluster RBAC the same way you would for plain `kubectl`.
- Anyone who can run `kube` as you can run `kubectl` as you, including reading Secrets allowed by RBAC.
- A parent process that sets `KUBE_SHELL_KUBECTL` or `KUBE_SHELL_NODE_SCRIPT` (test hooks) can substitute the child binary.

## What kube adds

| Control                       | Purpose                                                                                                                                                                                                                                                                                                                                    |
| ----------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| **Risk gate**                 | High-risk verbs need `y` on a TTY or `--yes` in pipes (delete, writes, pod access, port-forward, run, expose, …).                                                                                                                                                                                                                          |
| **Snapshot projection**       | Tab completion does not pull Secret/ConfigMap `.data` into the live cache.                                                                                                                                                                                                                                                                 |
| **History redaction**         | `~/.kube-shell/history` masks common secret flags.                                                                                                                                                                                                                                                                                         |
| **Private cache files**       | `0600` / `0700` on POSIX.                                                                                                                                                                                                                                                                                                                  |
| **Completion sanitization**   | Terminal controls stripped; unsafe values dropped.                                                                                                                                                                                                                                                                                         |
| **`plan` warnings**           | Revealing `get secret` / `get configmap` output is called out before run.                                                                                                                                                                                                                                                                  |
| **Delete blast radius**       | `plan delete`, `plan drain`, `plan scale --replicas=0`, and their confirmation prompts read objects, Services, PodDisruptionBudgets, and HPAs with `kubectl get -o json` under the caller's RBAC. `delete -f` reads local files. A delete may also call `kubectl delete --dry-run=server`. None of these writes or caches object payloads. |
| **Policy and plan ids**       | `~/.kube-shell/policy.json` is the only policy an agent reads. Human sessions may point at another file with `--policy` or `KUBE_SHELL_POLICY`. `plan --json` writes a 5-minute single-use record under `~/.kube-shell/plans/` (`0600`). The shell `ctx` and `ns` commands do not modify `~/.kube/config`.                                 |
| **Audit log**                 | `~/.kube-shell/audit.jsonl` (`0600`) records the redacted argv, risk, and decision for commands `kube` evaluates. It does not record stdout or Secret bodies. `KUBE_SHELL_AUDIT=0` disables it unless policy sets `audit: required`.                                                                                                       |
| **MCP**                       | `kube mcp` accepts JSON-RPC on stdin only. It loads `~/.kube-shell/policy.json` and ignores `--policy` and `KUBE_SHELL_POLICY`. Secret and ConfigMap stdout in tool results is redacted. Writes require a single-use plan id. Calls are audited as `actor: mcp`.                                                                           |
| **Undo snapshots**            | Before a named `set`, `apply`, `patch`, `scale`, `label`, or `annotate`, `kube` stores a cleaned object under `~/.kube-shell/undo/` (`0600`). Secrets and ConfigMaps are not snapshotted. Server fields such as `resourceVersion` are removed before a restore.                                                                            |
| **Pod diagnosis**             | `ask` about a failing pod reads that pod, its events, and previous logs with kubectl. The printed cause redacts password-style assignments. Those reads stay on this machine.                                                                                                                                                              |
| **Practice cluster**          | `kube sandbox up` creates kind cluster `kube-shell-sandbox` and writes only `~/.kube-shell/sandbox/kubeconfig`. It does not modify `~/.kube/config`. `kube --sandbox` points kubectl at that file.                                                                                                                                         |
| **Session runbook**           | `record on` and `kube --record` append commands that ran to a Markdown file under `~/.kube-shell/sessions/` (`0600`). Arguments are redacted. Refused commands are omitted.                                                                                                                                                                |
| **Optional stdout redaction** | `KUBE_SHELL_REDACT_STDOUT=1` redacts bodies on **non-TTY** stdout for those gets (pipes, not interactive terminals).                                                                                                                                                                                                                       |

See README **Security and privacy** for user-facing detail.

## Explicit non-goals

| ID  | Non-goal                                                                                       |
| --- | ---------------------------------------------------------------------------------------------- |
| S1  | No second identity in `~/.kube/config`. `kube sandbox` is a separate local kind cluster.       |
| S2  | No proxy between `kubectl` and the apiserver.                                                  |
| S3  | No default buffering of all kubectl stdout (breaks `attach`, `exec`, `port-forward`, follows). |
| S4  | Secret **names** still complete; removing them would be a product tradeoff, not a silent fix.  |
| S5  | Test env hooks are not hardened against a hostile parent.                                      |

## Guidance for agents

1. Call `kube plan --json …` before mutating or opening pods or ports.
2. Treat `risk: "high"` as requiring explicit `--yes` in non-interactive runs.
3. Do not assume kube redacts kubectl output unless `KUBE_SHELL_REDACT_STDOUT=1` **and** stdout is not a TTY.
