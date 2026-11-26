# Packaging

Release and install artifacts are built from the Go module. Node is only used for packaging helpers and Homebrew formula rendering.

## Version

| Source           | Role                                      |
| ---------------- | ----------------------------------------- |
| `VERSION`        | Single source of truth (`0.6.0`, …)       |
| `package.json`   | Must match `VERSION` (checked by `make`)  |
| Git tag `vX.Y.Z` | Must match `VERSION` at release           |
| `main.version`   | Embedded at link time (`-X main.version`) |

Bump by editing `VERSION` and `package.json` together, then tag `v$(cat VERSION)`.

## Make targets

```bash
make build    # bin/kube with embedded version
make test     # go test ./cmd/kube ./internal/kube/...
make install  # go install with the same ldflags
make dist     # cross-compiled tarballs under dist/
make clean
```

## Layout

| Path                            | Role                                            |
| ------------------------------- | ----------------------------------------------- |
| `cmd/kube/`                     | CLI entrypoint (`kube`)                         |
| `internal/kube/`                | Shared types, state paths, fixtures             |
| `internal/kube/parse/`          | Tokenize, grammar, catalog, completion scripts  |
| `internal/kube/policy/`         | policy.json, Authorize, plans, confirm          |
| `internal/kube/redact/`         | Argv / stdout secret redaction                  |
| `internal/kube/session/`        | History, audit, runbook, shell scope            |
| `internal/kube/live/`           | kubectl process + live completion cache         |
| `internal/kube/plan/`           | PlanCommand, blast radius, ask/diagnose         |
| `internal/kube/complete/`       | Word completion                                 |
| `internal/kube/view/`           | Interactive resource table                      |
| `internal/kube/mcp/`            | MCP JSON-RPC server                             |
| `internal/kube/undo/`           | Undo snapshots                                  |
| `internal/kube/sandbox/`        | Local kind sandbox                              |
| `scripts/packaging/`            | Release tarball builder and Homebrew formula    |
| `scripts/`                      | e2e / fake-kubectl / docs checks (not shipping) |
| `.github/workflows/release.yml` | Tag `v*.*.*` → GitHub Release + tap bump        |

Put shared types/state in `internal/kube`. Put implementation in the matching `internal/kube/<area>` package.

## Homebrew

`scripts/packaging/render-formula.mjs` writes a formula that builds from the GitHub source archive with:

```text
-ldflags="-s -w -X main.version=#{version}"
```

so `kube --version` matches the formula version without editing Go sources per release.
