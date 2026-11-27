package live

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/NadafMM/kubectl-shell/internal/kube"
)

func TestLoadKubeconfigCluster(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config")
	raw := `apiVersion: v1
kind: Config
current-context: prod-eu
contexts:
  - name: prod-eu
    context:
      cluster: prod-cluster
      user: admin
      namespace: checkout
  - name: dev
    context:
      cluster: dev-cluster
      user: dev
clusters:
  - name: prod-cluster
    cluster:
      server: https://127.0.0.1:6443
  - name: dev-cluster
    cluster:
      server: https://127.0.0.1:6444
`
	if err := os.WriteFile(cfg, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KUBECONFIG", cfg)
	t.Setenv("KUBE_SHELL_KUBECTL", "")
	t.Setenv("KUBE_SHELL_NODE_SCRIPT", "")

	cluster, ok := LoadKubeconfigCluster()
	if !ok || cluster.Current != "prod-eu" || kube.NamespaceOf(cluster) != "checkout" {
		t.Fatalf("ok=%v cluster=%+v", ok, cluster)
	}
	if len(cluster.Contexts) != 2 {
		t.Fatalf("expected 2 contexts, got %+v", cluster.Contexts)
	}
}

func TestRunJsonDirectSkipsNonGetAndExecHooks(t *testing.T) {
	if _, handled := RunJsonDirect([]string{"delete", "pod", "x"}, 1000); handled {
		t.Fatal("delete should fall back to kubectl")
	}
	t.Setenv("KUBE_SHELL_KUBECTL", "node")
	if !UsingExecHook() {
		t.Fatal("expected UsingExecHook true")
	}
	if _, handled := RunJsonDirect([]string{"get", "pods"}, 1000); handled {
		t.Fatal("exec hook should bypass client-go")
	}
}
