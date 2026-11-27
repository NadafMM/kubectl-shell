package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWriteScopedKubeconfigPreservesOriginal(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	base := filepath.Join(home, "base-config")
	original := `apiVersion: v1
kind: Config
current-context: dev
contexts:
  - name: dev
    context:
      cluster: dev-c
      user: dev-u
      namespace: default
  - name: prod-eu
    context:
      cluster: prod-c
      user: prod-u
      namespace: payments
clusters:
  - name: dev-c
    cluster:
      server: https://127.0.0.1:6443
  - name: prod-c
    cluster:
      server: https://127.0.0.1:6444
users:
  - name: dev-u
    user:
      token: dev-tok
  - name: prod-u
    user:
      token: prod-tok
`
	if err := os.WriteFile(base, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KUBECONFIG", base)
	t.Setenv("KUBE_SHELL_BASE_KUBECONFIG", "")

	p1, ctx1, ns1, err := WriteScopedKubeconfig("prod-eu", "checkout", "tab-1", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if ctx1 != "prod-eu" || ns1 != "checkout" {
		t.Fatalf("unexpected ctx=%s ns=%s", ctx1, ns1)
	}
	rawOrig, _ := os.ReadFile(base)
	if string(rawOrig) != original {
		t.Fatal("original kubeconfig was modified")
	}

	// Verify second isolated tab scope resolves from KUBE_SHELL_BASE_KUBECONFIG cleanly.
	t.Setenv("KUBECONFIG", p1)
	t.Setenv("KUBE_SHELL_BASE_KUBECONFIG", base)
	p2, ctx2, ns2, err := WriteScopedKubeconfig("dev", "auth", "tab-2", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if ctx2 != "dev" || ns2 != "auth" || p1 == p2 {
		t.Fatalf("ctx2=%s ns2=%s p1=%s p2=%s", ctx2, ns2, p1, p2)
	}

	sh := FormatScopeEnv("zsh", p2, base)
	if !strings.Contains(sh, "export KUBECONFIG=") || !strings.Contains(sh, "KUBE_SHELL_BASE_KUBECONFIG") {
		t.Fatalf("unexpected zsh env output: %s", sh)
	}
	ps := FormatScopeEnv("powershell", p2, base)
	if !strings.Contains(ps, "$env:KUBECONFIG =") {
		t.Fatalf("unexpected ps env output: %s", ps)
	}
}
