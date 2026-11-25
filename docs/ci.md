# Continuous integration

Workflows under `.github/workflows/` run on every PR to `main`.

| Workflow         | Purpose                                                                                  |
| ---------------- | ---------------------------------------------------------------------------------------- |
| **test**         | `go test` as jobs named `go` and `unit (os, node …)` (labels kept for branch protection) |
| **coverage**     | Placeholder required-check job; runs `go test` (TS coverage removed with the CLI)        |
| **lint**         | Prettier check                                                                           |
| **e2e-tty**      | `scripts/e2e-tty.py` on Ubuntu and macOS (Python 3; pty scenarios against `bin/kube`)    |
| **e2e-kind**     | kind + real `kubectl`; `scripts/e2e-kind.mjs` against `bin/kube`                         |
| **e2e-minikube** | Manual (`workflow_dispatch`) only                                                        |
| **audit**        | `npm audit --audit-level=high` on remaining Node tooling                                 |
| **codeql**       | GitHub CodeQL for Go and JavaScript; uploads SARIF to code scanning                      |
| **docs**         | `npm run docs:check` when README/docs change                                             |
| **semantic-pr**  | Conventional Commits PR titles                                                           |
| **labeler**      | Auto labels from paths (`.github/labeler.yml`)                                           |
| **release**      | Tag `v*.*.*` → Go binaries on the GitHub Release, Homebrew tap bump, brew smoke on macOS |

## Local commands

```bash
make test
make build               # embeds VERSION; see docs/packaging.md
npm ci                   # prettier + docs helpers only
npm run docs:check
npm run e2e:tty          # Linux/macOS, Python 3; needs bin/kube and node for fake-kubectl
KUBECONFIG=~/.kube/config npm run e2e:kind   # with kind/minikube running
```

## Release and Homebrew

On push of tag `vX.Y.Z`:

1. **release** workflow runs Go tests, then `make dist` (`scripts/packaging/package-release.mjs`), and attaches `kubectl-shell-*-<os>-<arch>.tar.gz` to the GitHub Release.
2. **homebrew-tap** checks out **NadafMM/homebrew-tap**, renders `Formula/kubectl-shell.rb` via `scripts/packaging/render-formula.mjs` from the GitHub source archive for that tag (Go build, no Node runtime), commits, and pushes when `HOMEBREW_TAP_TOKEN` is set.
3. **brew-smoke** runs `brew install NadafMM/tap/kubectl-shell`, `brew test kubectl-shell`, and `kube --version` on macOS when the tap job ran.

Secret **HOMEBREW_TAP_TOKEN**: a PAT with **`contents: write`** on **`NadafMM/homebrew-tap`** (not the main kubectl-shell repo).

The formula source of truth is **NadafMM/homebrew-tap**; this repo only renders it during release.

## Fork PR notes

**semantic-pr** and **labeler** use `pull_request_target`. Only maintainers should enable them for trusted forks; GitHub still runs them with default token permissions as configured.
