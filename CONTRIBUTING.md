# Contributing to kubectl-shell

Thanks for helping improve **`kube`**. This project is a terminal wrapper around real `kubectl` — patches should stay small, tested, and safe for cluster operators.

## Before you start

- Read the [README](README.md) and [threat model](docs/security/threat-model.md).
- Security issues: see [SECURITY.md](SECURITY.md) (please do not open public issues for exploitable bugs).
- By participating, you agree to the [Code of Conduct](CODE_OF_CONDUCT.md).

## Development setup

Requirements: **Go 1.22+**, **kubectl** (for optional e2e). Node.js is only needed for `scripts/fake-kubectl.mjs`, Prettier, and docs checks. Python 3 is needed for `npm run e2e:tty`.

```bash
git clone https://github.com/NadafMM/kubectl-shell.git
cd kubectl-shell
go test ./cmd/kube ./internal/kube
go build -o bin/kube ./cmd/kube
```

Optional:

```bash
npm ci
npm run docs:check
npx prettier --check .
npm run e2e:tty          # Linux/macOS
# With kind or minikube running:
npm run e2e:kind
```

CI details: [docs/ci.md](docs/ci.md).

## Making changes

1. **Fork** and branch from `main` (`feat/…`, `fix/…`, `docs/…`).
2. **Edit** `cmd/kube` and `internal/kube`.
3. **Add or update tests** beside the code (tests use `scripts/fake-kubectl.mjs`; no real cluster required for unit tests).
4. **Keep scope tight** — one logical change per PR.
5. **Run** `go test ./cmd/kube ./internal/kube` locally.

## Pull requests

- Use a **[Conventional Commits](https://www.conventionalcommits.org/)** style title (e.g. `fix: gate port-forward in non-tty pipes`, `docs: clarify coverage`). The **semantic-pr** workflow enforces this.
- Describe **what** and **why**. Link an issue if one exists.
- PRs need a green CI matrix (see [docs/ci.md](docs/ci.md)). Kind e2e runs on Ubuntu only.
- Maintainers merge with **merge commit** (no force-push to `main`).

## Project layout

| Path             | Role                                                     |
| ---------------- | -------------------------------------------------------- |
| `cmd/kube/`      | Go CLI entry (installed `kube`)                          |
| `internal/kube/` | Completion, plan/ask, view, sandbox, mcp, record, etc.   |
| `scripts/`       | Fake kubectl, release packaging, e2e helpers, docs check |

Do not restore sample/offline cluster mode.

## License

Contributions are under the [MIT License](LICENSE).
