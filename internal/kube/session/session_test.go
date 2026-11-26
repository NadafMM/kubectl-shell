package session

import (
	"github.com/NadafMM/kubectl-shell/internal/kube"
	"strings"
	"testing"
)

func TestCommandArgsAppliesShellNamespace(t *testing.T) {
	scope := &ShellScope{Namespace: "payments"}
	args := CommandArgs("kubectl get pods", scope)
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "-n payments") {
		t.Fatal(args)
	}
	args = CommandArgs("kubectl get pods -n other", scope)
	if strings.Contains(strings.Join(args, " "), "payments") {
		t.Fatal(args)
	}
}

func TestUseNamespaceUpdatesPromptContext(t *testing.T) {
	cluster := kube.Cluster{
		Current:  "minikube",
		Contexts: []kube.Context{{Name: "minikube", Namespace: "default"}},
	}
	scope := &ShellScope{}
	next := UseNamespace(cluster, scope, "payments")
	if kube.NamespaceOf(next) != "payments" || scope.Namespace != "payments" {
		t.Fatalf("%+v scope=%+v", next, scope)
	}
	if kube.NamespaceOf(cluster) != "default" {
		t.Fatal("original cluster mutated")
	}
}

func TestUseContext(t *testing.T) {
	cluster := kube.Cluster{
		Current: "minikube",
		Contexts: []kube.Context{
			{Name: "minikube", Namespace: "default"},
			{Name: "staging", Namespace: "app"},
		},
	}
	scope := &ShellScope{}
	next, ok := UseContext(cluster, scope, "staging")
	if !ok || next.Current != "staging" || scope.Context != "staging" {
		t.Fatalf("%v %+v %+v", ok, next, scope)
	}
	if _, ok := UseContext(cluster, scope, "missing"); ok {
		t.Fatal("expected miss")
	}
}
