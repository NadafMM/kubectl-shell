# Contributing to kubectl-shell

Thanks for helping improve **`kube`**. This project is a terminal wrapper around real `kubectl` — patches should stay small, tested, and safe for cluster operators.

## Before you start

- Read the [README](README.md) and [threat model](docs/security/threat-model.md).
- Security issues: see [SECURITY.md](SECURITY.md) (please do not open public issues for exploitable bugs).
- By participating, you agree to the [Code of Conduct](CODE_OF_CONDUCT.md).

## Development setup

Requirements: **Node.js 22+**, **npm**, **kubectl** (for optional e2e), and **Python 3** for `npm run e2e:tty`.

```bash
git clone https://github.com/NadafMM/kubectl-shell.git
cd kubectl-shell
npm ci
npm run typecheck
npm run build && npm test
git diff --exit-code -- cli/kube.cjs   # bundle must match source before you push
```

Optional:

```bash
npm run lint
npm run coverage
npm run docs:check
npm run e2e:tty          # Linux/macOS
# With kind or minikube running:
npm run e2e:kind
```

CI details: [docs/ci.md](docs/ci.md).

## Making changes

1. **Fork** and branch from `main` (`feat/…`, `fix/…`, `docs/…`).
2. **Edit** `cli/` or `src/lib/kube/` TypeScript — not hand-edit `cli/kube.cjs`; run `npm run build`.
3. **Add or update tests** in `*.test.ts` beside the code (tests use a fake kubectl; no real cluster required for unit tests).
4. **Keep scope tight** — one logical change per PR.
5. **Run** `npm run build && npm test && npm run typecheck && npm run lint` locally.

## Pull requests

- Use a **[Conventional Commits](https://www.conventionalcommits.org/)** style title (e.g. `fix: gate port-forward in non-tty pipes`, `docs: clarify coverage`). The **semantic-pr** workflow enforces this.
- Describe **what** and **why**. Link an issue if one exists.
- PRs need a green CI matrix (see [docs/ci.md](docs/ci.md)). Kind e2e runs on Ubuntu only.
- Maintainers merge with **merge commit** (no force-push to `main`).

## Project layout

| Path            | Role                                                     |
| --------------- | -------------------------------------------------------- |
| `cli/`          | CLI entry (`main.ts`), live snapshot, plan/ask, view TUI |
| `src/lib/kube/` | Completion, parsing, resource model (shared with CLI)    |
| `scripts/`      | Test runner, build, fake kubectl, e2e helpers            |
| `cli/kube.cjs`  | Bundled binary (committed; CI checks drift)              |

Do not restore sample/offline cluster mode.

## Questions

Open a [Discussion](https://github.com/NadafMM/kubectl-shell/discussions) or an issue labeled `question` if you are unsure about direction before a large change.
