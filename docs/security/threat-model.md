# kube threat model

`kube` is a terminal wrapper around **your** `kubectl` binary and **your** kubeconfig. It does not add a network service, a second identity, or a sandbox.

## Trust assumptions

- You trust the machine, your kubeconfig, and cluster RBAC the same way you would for plain `kubectl`.
- Anyone who can run `kube` as you can run `kubectl` as you, including reading Secrets allowed by RBAC.
- A parent process that sets `KUBE_SHELL_KUBECTL` or `KUBE_SHELL_NODE_SCRIPT` (test hooks) can substitute the child binary.

## What kube adds

| Control                       | Purpose                                                                                                              |
| ----------------------------- | -------------------------------------------------------------------------------------------------------------------- |
| **Risk gate**                 | High-risk verbs need `y` on a TTY or `--yes` in pipes (delete, writes, pod access, port-forward, run, expose, …).    |
| **Snapshot projection**       | Tab completion does not pull Secret/ConfigMap `.data` into the live cache.                                           |
| **History redaction**         | `~/.kube-shell/history` masks common secret flags.                                                                   |
| **Private cache files**       | `0600` / `0700` on POSIX.                                                                                            |
| **Completion sanitization**   | Terminal controls stripped; unsafe values dropped.                                                                   |
| **`plan` warnings**           | Revealing `get secret` / `get configmap` output is called out before run.                                            |
| **Delete blast radius**       | `plan delete`, `plan drain`, `plan scale --replicas=0`, and their confirmation prompts read objects, Services, PodDisruptionBudgets, and HPAs with `kubectl get -o json` under the caller's RBAC. `delete -f` reads local files. A delete may also call `kubectl delete --dry-run=server`. None of these writes or caches object payloads. |
| **Policy and plan ids**       | `~/.kube-shell/policy.json` is the only policy an agent reads. Human sessions may point at another file with `--policy` or `KUBE_SHELL_POLICY`. `plan --json` writes a 5-minute single-use record under `~/.kube-shell/plans/` (`0600`). The shell `ctx` and `ns` commands do not modify `~/.kube/config`. |
| **Audit log**                 | `~/.kube-shell/audit.jsonl` (`0600`) records the redacted argv, risk, and decision for commands `kube` evaluates. It does not record stdout or Secret bodies. `KUBE_SHELL_AUDIT=0` disables it unless policy sets `audit: required`. |
| **Optional stdout redaction** | `KUBE_SHELL_REDACT_STDOUT=1` redacts bodies on **non-TTY** stdout for those gets (pipes, not interactive terminals). |

See README **Security and privacy** for user-facing detail.

## Explicit non-goals

| ID  | Non-goal                                                                                       |
| --- | ---------------------------------------------------------------------------------------------- |
| S1  | No sandbox, no alternate Kubernetes identity.                                                  |
| S2  | No proxy between `kubectl` and the apiserver.                                                  |
| S3  | No default buffering of all kubectl stdout (breaks `attach`, `exec`, `port-forward`, follows). |
| S4  | Secret **names** still complete; removing them would be a product tradeoff, not a silent fix.  |
| S5  | Test env hooks are not hardened against a hostile parent.                                      |

## Guidance for agents

1. Call `kube plan --json …` before mutating or opening pods or ports.
2. Treat `risk: "high"` as requiring explicit `--yes` in non-interactive runs.
3. Do not assume kube redacts kubectl output unless `KUBE_SHELL_REDACT_STDOUT=1` **and** stdout is not a TTY.
