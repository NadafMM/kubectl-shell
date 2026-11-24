# kubectl shell

[![MIT license](https://img.shields.io/github/license/NadafMM/kubectl-shell)](LICENSE)
[![CI](https://github.com/NadafMM/kubectl-shell/actions/workflows/test.yml/badge.svg)](https://github.com/NadafMM/kubectl-shell/actions/workflows/test.yml)
[![Latest release](https://img.shields.io/github/v/release/NadafMM/kubectl-shell)](https://github.com/NadafMM/kubectl-shell/releases/latest)

**`kube`** is a terminal companion for **`kubectl`**: live Tab completion, interactive **`view`**, **`plan`**, and **`ask`** helpers, all against your current kubeconfig.

![kube demo](docs/demo.gif)

## Requirements

- **Node.js 22+** (Homebrew installs its own Node dependency)
- **`kubectl`** on `PATH`
- A **reachable cluster** in the current kubeconfig (no offline or practice mode)

## Install

**Homebrew (macOS / Linux):**

```bash
brew tap NadafMM/tap
brew install kubectl-shell
kube --version
```

Upgrade: `brew update && brew upgrade kubectl-shell`

**From source (any OS, including Windows):**

```bash
git clone https://github.com/NadafMM/kubectl-shell.git
cd kubectl-shell
npm install -g .
kube --version
```

The bundle in `cli/kube.cjs` is committed; no build step is required to install. Release assets also include `kubectl-shell-*.tar.gz` and a standalone `kube.cjs`.

`help`, `--version`, and `completion` work without a cluster; other commands need a live API.

## Everyday use

```bash
kube                    # interactive shell
kube get pods           # one-shot
kube view               # full-screen list (plain table without a TTY)
kube plan delete pod x  # name matching pods and Services that would lose Ready pods
kube ask "why is checkout crashing"
```

**`ask`** reads a failing pod, its events, and previous logs through kubectl, and still maps other questions with local pattern rules. It makes no AI-model calls of its own.

**Shell meta commands:** `live`, `reset`, `view` / `ui`, `plan`, `ask` / `?`, `help`, `exit`.

**`kube view` keys (TTY):** `1`–`5` resource kinds, `j`/`k`, Enter describe, `l` logs (pods), `d` delete (confirmed), `/` filter, `n` namespace, `a` all namespaces, `r` refresh, `q` quit.

High-risk verbs (delete, writes, `port-forward`, `run`, `expose`, pod `exec`/`cp`/…) prompt **`y`** on a TTY; use **`kube --yes`** in scripts. **`kube --yes` alone does not open a pre-confirmed shell.**

`plan delete`, `plan drain`, and `plan scale --replicas=0` — and the interactive confirmation for those commands — look up matching objects with read-only `kubectl get -o json` calls. They flag Services that would lose Ready pods and PodDisruptionBudgets that allow 0 disruptions. `delete -f` reads local YAML or JSON only. `kube --yes` skips that lookup. A delete also tries `kubectl delete --dry-run=server` and still prints the object list if the API denies it.

A policy file at `~/.kube-shell/policy.json` can mark a context `guarded` or `confirm: typed`, and can deny verbs. Inside the shell, `ctx` and `ns` change context and namespace for that shell only. Agents (`KUBE_SHELL_AGENT=1`) that require an approved plan use `kube plan --json`, then `kube --yes --plan-id <id>`. That id works once.

`kube audit tail` prints recent redacted records from `~/.kube-shell/audit.jsonl`. `KUBE_SHELL_AUDIT=0` turns that file off unless the policy says `audit: required`.

`kube mcp` speaks JSON-RPC on stdin for `kube_context`, `kube_plan`, and `kube_execute`. It reads only `~/.kube-shell/policy.json`. Secret and ConfigMap output is redacted in those responses. A delete needs a plan id from `kube_plan`.

`kube undo` restores the previous saved object for a named `set`, `apply`, `patch`, `scale`, `label`, or `annotate`. It asks before it writes. Secrets, ConfigMaps, and Pods are refused.

Agents: `kube plan --json …`, then `--yes` when the plan matches intent. `__complete --json` for completion without running the line.

## Shell completion

`kube completion bash|zsh|fish` prints a script for that shell. One-line install examples:

```bash
# bash (~/.bashrc)
eval "$(kube completion bash)"

# zsh (~/.zshrc)
source <(kube completion zsh)

# fish (~/.config/fish/completions/kube.fish)
kube completion fish | source
```

## Configuration

| Name / path                  | Purpose                                                     |
| ---------------------------- | ----------------------------------------------------------- |
| `KUBE_SHELL_REDACT_STDOUT=1` | Redact Secret/ConfigMap bodies when stdout is piped         |
| `KUBE_SHELL_KUBECTL`         | Path to an alternate `kubectl` binary                       |
| `~/.kube-shell/history`      | Redacted command history (last 200 lines, file mode `0600`) |
| `~/.kube-shell/live.json`    | ~12-second completion cache (no Secret/ConfigMap data)      |

## Security and privacy

Summary: projected completion snapshot (no Secret/ConfigMap bodies in cache), redacted history, sanitized Tab output, optional piped stdout redaction (`KUBE_SHELL_REDACT_STDOUT=1`).

Details: [docs/security/threat-model.md](docs/security/threat-model.md). Report vulnerabilities via [private vulnerability reporting](https://github.com/NadafMM/kubectl-shell/security/advisories/new) — see [SECURITY.md](SECURITY.md).

## Develop

```bash
npm ci
npm run typecheck
npm run build && npm test
npm run coverage
npm run lint
npm run e2e:tty   # Linux/macOS; requires Python 3
```

CI overview: [docs/ci.md](docs/ci.md). The demo GIF is produced from [`docs/demo.tape`](docs/demo.tape) (VHS) against a real cluster.

## Contributing and license

Contributions welcome — read [CONTRIBUTING.md](CONTRIBUTING.md). Use [Conventional Commits](https://www.conventionalcommits.org/) in PR titles. Questions: [Discussions](https://github.com/NadafMM/kubectl-shell/discussions). This project is under the [MIT License](LICENSE).
