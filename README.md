# kubectl shell

[![MIT license](https://img.shields.io/github/license/NadafMM/kubectl-shell)](LICENSE)
[![CI](https://github.com/NadafMM/kubectl-shell/actions/workflows/test.yml/badge.svg)](https://github.com/NadafMM/kubectl-shell/actions/workflows/test.yml)

**`kube`** is a terminal companion for **`kubectl`**: live Tab completion, interactive **`view`**, **`plan`**, and **`ask`** helpers, all against your current kubeconfig.

![kube demo](docs/demo.gif)

## Requirements

- **Node.js 22+**
- **`kubectl`** on `PATH`
- A **reachable cluster** in the current kubeconfig (no offline or practice mode)

## Install

**Homebrew (macOS / Linux):**

```bash
brew tap NadafMM/tap
brew install kubectl-shell
```

Upgrade: `brew update && brew upgrade kubectl-shell`

**From source (any OS, including Windows):**

```bash
git clone https://github.com/NadafMM/kubectl-shell.git
cd kubectl-shell
npm install -g .
kube --version
```

The bundle in `cli/kube.cjs` is committed; no build step is required to install.

`help`, `--version`, and `completion` work without a cluster; other commands need a live API.

## Everyday use

```bash
kube                    # interactive shell
kube get pods           # one-shot
kube view               # full-screen list (plain table without a TTY)
kube plan delete pod x  # preview risk before running
kube ask "why is checkout crashing"
```

**`ask`** maps questions to kubectl commands with local pattern rules; it makes no network or AI-model calls.

**Shell meta commands:** `live`, `reset`, `view` / `ui`, `plan`, `ask` / `?`, `help`, `exit`.

**`kube view` keys (TTY):** `1`–`5` resource kinds, `j`/`k`, Enter describe, `l` logs (pods), `d` delete (confirmed), `/` filter, `n` namespace, `a` all namespaces, `r` refresh, `q` quit.

High-risk verbs (delete, writes, `port-forward`, `run`, `expose`, pod `exec`/`cp`/…) prompt **`y`** on a TTY; use **`kube --yes`** in scripts. **`kube --yes` alone does not open a pre-confirmed shell.**

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

Details: [docs/security/threat-model.md](docs/security/threat-model.md).

## Develop

```bash
npm ci
npm run typecheck
npm run build && npm test
npm run coverage
npm run lint
npm run e2e:tty   # Linux/macOS; requires Python 3
```

CI overview: [docs/ci.md](docs/ci.md).

## Contributing and license

Contributions welcome — read [CONTRIBUTING.md](CONTRIBUTING.md). Use [Conventional Commits](https://www.conventionalcommits.org/) in PR titles. This project is under the [MIT License](LICENSE).
