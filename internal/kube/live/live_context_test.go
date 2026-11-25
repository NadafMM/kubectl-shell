package live

import (
	"github.com/NadafMM/kubectl-shell/internal/kube"
	"testing"
)

func TestApplyLiveContextKeepsShellCurrentWhenKubeconfigMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	cluster := kube.Cluster{
		Current:  "prod",
		Contexts: []kube.Context{{Name: "prod", Namespace: "payments"}},
	}
	next := ApplyLiveContext(cluster)
	if next.Current != "prod" {
		t.Fatalf("current changed to %q", next.Current)
	}
}
