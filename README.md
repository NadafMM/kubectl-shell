# kubectl shell

[![MIT license](https://img.shields.io/github/license/NadafMM/kubectl-shell)](LICENSE)
[![CI](https://github.com/NadafMM/kubectl-shell/actions/workflows/test.yml/badge.svg)](https://github.com/NadafMM/kubectl-shell/actions/workflows/test.yml)
[![Latest release](https://img.shields.io/github/v/release/NadafMM/kubectl-shell)](https://github.com/NadafMM/kubectl-shell/releases/latest)

**`kube`** is a terminal companion for **`kubectl`**: live Tab completion, interactive **`view`**, **`plan`**, and **`ask`** helpers, all against your current kubeconfig.

![kube demo](docs/demo.gif)

## Requirements

- **Go 1.22+** to build from source (Homebrew installs Go as a build dependency)
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

**From a release binary:** download the matching `kubectl-shell-<version>-<os>-<arch>.tar.gz` from [Releases](https://github.com/NadafMM/kubectl-shell/releases), extract, and put `kube` on your `PATH`.

**From source:**

```bash
git clone https://github.com/NadafMM/kubectl-shell.git
cd kubectl-shell
make build
./bin/kube --version
```

Or: `go install github.com/NadafMM/kubectl-shell/cmd/kube@latest` (version string is the module version when installed that way).

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

High-risk verbs prompt **`y`** on a TTY (unless `--yes`): `delete`, `drain`, `cordon`/`taint`, `apply`/`create`/`replace`/`patch`, `set`/`label`/`annotate`/`edit`, `scale`/`autoscale`/`uncordon`, `run`/`expose`, `exec`/`cp`/`attach`/`debug`, `port-forward`/`proxy`, and similar. Dry-runs stay low risk. Unknown verbs (plugins, etc.) are treated as read and do not prompt. **`kube --yes` alone does not open a pre-confirmed shell.**

`plan delete`, `plan drain`, and `plan scale --replicas=0` — and the interactive confirmation for those commands — look up matching objects with read-only `kubectl get -o json` calls. They flag Services that would lose Ready pods and PodDisruptionBudgets that allow 0 disruptions. `delete -f` reads local YAML or JSON only. `kube --yes` skips that lookup. A delete also tries `kubectl delete --dry-run=server` and still prints the object list if the API denies it.

A policy file at `~/.kube-shell/policy.json` can mark a context `guarded` or `confirm: typed`, and can deny verbs. `agents.allow` lists every verb an agent or MCP client may run (reads and writes). With `write_requires: approved-plan`, mutating verbs also need a single-use `--plan-id` from `kube plan --json` (and `--confirm-context` when the context is `confirm: typed`). `match_non_tty: true` applies those agent rules to non-TTY clients. Inside the shell, `ctx` and `ns` change context and namespace for that shell only.

`kube audit tail` prints recent redacted records from `~/.kube-shell/audit.jsonl`. `plan --json` and MCP `kube_plan` are audited as `planned`. `KUBE_SHELL_AUDIT=0` turns that file off unless the policy says `audit: required`.

Secret, ConfigMap, and raw kubeconfig reads print a short warning on stderr. `kube mcp` speaks JSON-RPC on stdin for `kube_context`, `kube_plan`, and `kube_execute`. It reads only `~/.kube-shell/policy.json`. Secret/ConfigMap-shaped output is redacted in MCP responses (including mutating commands). A write needs the verb in `agents.allow` and, when `write_requires` is `approved-plan`, a plan id from `kube_plan`.

`kube undo` restores the previous saved object for a named `set`, `apply`, `patch`, `scale`, `label`, or `annotate`. It asks before it writes. Secrets, ConfigMaps, and Pods are refused.

`kube sandbox up` starts a local kind cluster named `kube-shell-sandbox` and writes only `~/.kube-shell/sandbox/kubeconfig`. `kube --sandbox` and `kube sandbox shell` use that file. `kube sandbox down` deletes the cluster. `~/.kube/config` is not modified.

`record on` inside the shell, or `kube --record session.md`, writes a redacted Markdown runbook of commands that actually ran. A refused confirmation is left out. The default file is `~/.kube-shell/sessions/`.

Agents: `kube plan --json …`, then `--yes` when the plan matches intent. `__complete --json` for completion without running the line.

## Shell completion

`kube completion bash|zsh|fish|powershell` prints a script for that shell. One-line install examples:

```bash
# bash (~/.bashrc)
eval "$(kube completion bash)"

# zsh (~/.zshrc)
source <(kube completion zsh)

# fish (~/.config/fish/completions/kube.fish)
kube completion fish | source
```

```powershell
# PowerShell profile
kube completion powershell | Out-String | Invoke-Expression
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
make test
make build             # embeds VERSION; see docs/packaging.md
npm ci                 # prettier + docs helpers
npm run docs:check
npm run e2e:tty        # Linux/macOS; needs bin/kube and node for fake-kubectl
```

CI overview: [docs/ci.md](docs/ci.md). Packaging: [docs/packaging.md](docs/packaging.md). The demo GIF is produced from [`docs/demo.tape`](docs/demo.tape) (VHS) against a real cluster.

## Contributing and license

Contributions welcome — read [CONTRIBUTING.md](CONTRIBUTING.md). Use [Conventional Commits](https://www.conventionalcommits.org/) in PR titles. Questions: [Discussions](https://github.com/NadafMM/kubectl-shell/discussions). This project is under the [MIT License](LICENSE).
