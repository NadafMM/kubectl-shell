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
make test     # go test ./cmd/kube ./internal/kube
make install  # go install with the same ldflags
make dist     # cross-compiled tarballs under dist/
make clean
```

## Layout

| Path                            | Role                                            |
| ------------------------------- | ----------------------------------------------- |
| `cmd/kube/`                     | CLI entrypoint (`kube`)                         |
| `internal/kube/`                | Library used by the CLI (completion, plan, …)   |
| `scripts/packaging/`            | Release tarball builder and Homebrew formula    |
| `scripts/`                      | e2e / fake-kubectl / docs checks (not shipping) |
| `.github/workflows/release.yml` | Tag `v*.*.*` → GitHub Release + tap bump        |

`internal/kube` stays one package today so call sites stay simple. New features should keep files grouped by concern (`view_*.go`, `redact*.go`, …) so a later split into `internal/kube/<concern>` packages stays mechanical.

## Homebrew

`scripts/packaging/render-formula.mjs` writes a formula that builds from the GitHub source archive with:

```text
-ldflags="-s -w -X main.version=#{version}"
```

so `kube --version` matches the formula version without editing Go sources per release.
