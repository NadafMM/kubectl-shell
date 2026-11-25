# Continuous integration

Workflows under `.github/workflows/` run on every PR to `main`.

| Workflow         | Purpose                                                                                                       |
| ---------------- | ------------------------------------------------------------------------------------------------------------- |
| **test**         | Node **22** / **24** on Ubuntu; smoke on Ubuntu 22, macOS, and Windows; bundle drift check on Ubuntu 22 only. |
| **lint**         | `tsc` + ESLint                                                                                                |
| **coverage**     | Node `--experimental-test-coverage` via `npm run coverage`                                                    |
| **e2e-tty**      | `npm run e2e:tty` on Ubuntu and macOS (Python 3; pty scenarios)                                               |
| **e2e-kind**     | kind + real `kubectl`; `npm run e2e:kind`                                                                     |
| **e2e-minikube** | Manual (`workflow_dispatch`) only                                                                             |
| **audit**        | `npm audit --audit-level=high`                                                                                |
| **codeql**       | GitHub CodeQL for JavaScript/TypeScript; uploads SARIF to code scanning                                       |
| **docs**         | `npm run docs:check` when README/docs change                                                                  |
| **semantic-pr**  | Conventional Commits PR titles                                                                                |
| **labeler**      | Auto labels from paths (`.github/labeler.yml`)                                                                |
| **release**      | Tag `v*.*.*` → Go binaries on the GitHub Release, Homebrew tap bump (builds from source), brew smoke on macOS |

The **test** workflow also runs `go test ./cmd/kube ./internal/kube` on Ubuntu. That job needs Node because the tests drive `scripts/fake-kubectl.mjs`.

## Local commands

```bash
go test ./cmd/kube ./internal/kube
go build -o bin/kube ./cmd/kube
npm run build && npm test   # transitional TypeScript suite
npm run coverage
npm run docs:check
npm run e2e:tty          # Linux/macOS, Python 3
KUBECONFIG=~/.kube/config npm run e2e:kind   # with kind/minikube running (e2e pins KUBECONFIG when it uses a temp HOME)
```

## Release and Homebrew

On push of tag `vX.Y.Z`:

1. **release** workflow runs the TypeScript and Go test suites, cross-compiles static `kube` binaries (`scripts/package-release.mjs`), and attaches `kubectl-shell-*-<os>-<arch>.tar.gz` to the GitHub Release.
2. **homebrew-tap** checks out **NadafMM/homebrew-tap**, renders `Formula/kubectl-shell.rb` from the GitHub source archive for that tag (Go build, no Node runtime), commits, and pushes when `HOMEBREW_TAP_TOKEN` is set.
3. **brew-smoke** runs `brew install NadafMM/tap/kubectl-shell`, `brew test kubectl-shell`, and `kube --version` on macOS when the tap job ran.

Secret **HOMEBREW_TAP_TOKEN**: a PAT with **`contents: write`** on **`NadafMM/homebrew-tap`** (not the main kubectl-shell repo).

The formula source of truth is **NadafMM/homebrew-tap**; this repo only renders it during release.

## Fork PR notes

**semantic-pr** and **labeler** use `pull_request_target`. Only maintainers should enable them for trusted forks; GitHub still runs them with default token permissions as configured.
