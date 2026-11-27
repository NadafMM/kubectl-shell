# Comparing `kubectl-shell` (`kube`) with Open-Source Kubernetes Tools

`kube` is built to sit in front of `kubectl` as a **safe execution, preflight blast-radius, deterministic tracing, and gated MCP layer**—not to replace your shell, `k9s`, or `kubecolor`.

## Feature Matrix

| Tool                                   | Primary Role                                          | Blast Radius Before Mutation                                                                                         | Zero-LLM Cross-Resource Tracing                                                                   | Native Shell Hook + Isolated Scopes               | MCP + Single-Use `planId` Gate                                                  |
| :------------------------------------- | :---------------------------------------------------- | :------------------------------------------------------------------------------------------------------------------- | :------------------------------------------------------------------------------------------------ | :------------------------------------------------ | :------------------------------------------------------------------------------ |
| **`kubectl-shell` (`kube`)**           | Guarded CLI, `kubectl blast`, `kube hook`, `kube mcp` | Yes (`delete`, `drain`, `scale`, `apply`, `patch` + `Ingress`/`HTTPRoute`/`PVC`/`PDB`/`drain` capacity/GitOps drift) | Yes (pods, `503`/Service/`Ingress`/`HTTPRoute`/`NetworkPolicy`, stuck rollouts, namespace health) | Yes (`kube hook`, `kube env`, `kubectl ctx`/`ns`) | Yes (NDJSON stdio, `--read-only`, Secret redaction, `managedFields` compaction) |
| **`derailed/k9s`**                     | Real-time full-screen cluster TUI                     | No cross-resource `Ingress`/`PVC`/capacity preflight                                                                 | Manual navigation (`describe`, `logs`, X-ray)                                                     | N/A (standalone TUI)                              | No                                                                              |
| **`kubecolor/kubecolor`**              | Colorized `kubectl` output wrapper                    | No                                                                                                                   | No                                                                                                | Composes inside `kube hook`                       | No                                                                              |
| **`sbstp/kubie`**                      | Isolated per-shell `KUBECONFIG` switcher              | No                                                                                                                   | No                                                                                                | Yes (dedicated context/ns switcher)               | No                                                                              |
| **`brianpursley/kubectl-confirm`**     | Confirmation prompt plugin                            | Dry-run / prompt only                                                                                                | No                                                                                                | Prompt only                                       | No                                                                              |
| **`k8sgpt-ai/k8sgpt`**                 | Cluster analyzer + LLM explanations                   | No                                                                                                                   | Rule analyzers + LLM synthesis                                                                    | No                                                | MCP mode, LLM-backed                                                            |
| **`containers/kubernetes-mcp-server`** | Native Go Kubernetes API MCP server                   | No two-step `planId` blast gate                                                                                      | Raw API queries (`--read-only`, toolsets)                                                         | N/A                                               | Native MCP (`--read-only`, OTel), no `planId` workflow                          |

## Better Together (Recommended Stack)

`kube` does not try to out-TUI `k9s` or out-colorize `kubecolor`. They compose cleanly in one shell:

```bash
# ~/.zshrc (or ~/.bashrc)
eval "$(kube hook zsh)"      # reads pass through kubecolor (if installed); writes go through kube
eval "$(kube completion zsh)"
```

- **Daily terminal CLI:** Keep your `zsh`/`bash`/`fish`/PowerShell pipes, `jq`, and `fzf`. Reads run straight through `kubecolor` or `kubectl` with zero overhead; `kubectl ctx <name>` / `kubectl ns <name>` isolate `KUBECONFIG` per terminal under `~/.kube-shell/scopes/`; mutations show blast radius, GitOps warnings, and `confirm: typed` prompts.
- **Full-screen browsing:** Use `k9s --readonly` (or `k9s`) whenever you want live watches, logs, and resource trees; keep `kube view` as a zero-config plain/TTY table fallback.
- **AI coding assistants (Cursor, Claude Desktop, VS Code):** Run `kube mcp --read-only` for safe cluster exploration (`kube_diagnose`, `kube_namespace_health`, compacted `managedFields`-free reads), or omit `--read-only` with `~/.kube-shell/policy.json` to require `kube_plan` → single-use `planId` → `kube_execute` for writes.
- **Cluster-wide enforcement:** Pair `kube` with server-side RBAC, Kyverno, or OPA Gatekeeper—client-side CLI guards protect human/agent workflows before hitting the API, while admission controllers enforce invariants across all callers.
