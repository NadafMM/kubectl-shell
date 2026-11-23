# Security policy

## Supported versions

Security fixes are applied to **the latest release** (tags `v*.*.*` on `main`).

## Reporting a vulnerability

**Do not** open a public GitHub issue for security problems.

Report privately through [GitHub private vulnerability reporting](https://github.com/NadafMM/kubectl-shell/security/advisories/new). Include:

- Description and impact
- Steps to reproduce
- `kube` / kubectl version and OS

We aim to acknowledge within **72 hours** and share a fix or mitigation timeline when possible.

## Scope

**In scope:** the `kube` CLI, its use of kubeconfig/history/cache, confirmation gates, completion output, and documented env vars (`KUBE_SHELL_*`).

**Out of scope:** vulnerabilities in upstream **kubectl**, the Kubernetes API, or cluster RBAC misconfiguration that `kube` does not worsen beyond what plain `kubectl` would allow. See [docs/security/threat-model.md](docs/security/threat-model.md).

## Safe harbor

Good-faith research that avoids privacy violations, data destruction, and service disruption is welcome. Coordinated disclosure is appreciated.
