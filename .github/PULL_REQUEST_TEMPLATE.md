## Summary

<!-- What changed and why. Conventional commit title (fix:/feat:/docs:/ci:/…) -->

## Test plan

- [ ] `go test ./cmd/kube ./internal/kube`
- [ ] `npx prettier --check .` (if docs or scripts changed)
- [ ] `go build -o bin/kube ./cmd/kube` and relevant e2e when behavior changes

## Notes

<!-- Breaking changes, security impact, follow-ups -->
