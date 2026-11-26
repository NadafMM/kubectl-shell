package session

import (
	"github.com/NadafMM/kubectl-shell/internal/kube"
	"strings"

	// ShellScope holds shell-only namespace and context overrides. It does not rewrite kubeconfig.
	"github.com/NadafMM/kubectl-shell/internal/kube/parse"
)

type ShellScope struct {
	Namespace string
	Context   string
}

// CommandArgs turns an engine line into kubectl argv, applying shell scope when flags are absent.
func CommandArgs(engineLine string, scope *ShellScope) []string {
	args := parse.KubectlArgs(engineLine)
	if scope == nil {
		return args
	}
	hasNS := false
	hasCtx := false
	for _, token := range args {
		if token == "-n" || token == "--namespace" || strings.HasPrefix(token, "--namespace=") || strings.HasPrefix(token, "-n=") {
			hasNS = true
		}
		if token == "--context" || strings.HasPrefix(token, "--context=") {
			hasCtx = true
		}
	}
	if scope.Namespace != "" && !hasNS {
		args = append(args, "-n", scope.Namespace)
	}
	if scope.Context != "" && !hasCtx {
		args = append(args, "--context", scope.Context)
	}
	return args
}

// UseNamespace updates the current context's namespace for prompts and sets the shell scope.
func UseNamespace(cluster kube.Cluster, scope *ShellScope, namespace string) kube.Cluster {
	next := cluster
	next.Contexts = append([]kube.Context{}, cluster.Contexts...)
	for i := range next.Contexts {
		if next.Contexts[i].Name == next.Current {
			next.Contexts[i].Namespace = namespace
		}
	}
	if scope != nil {
		scope.Namespace = namespace
	}
	return next
}

// UseContext switches the current context for this shell only.
func UseContext(cluster kube.Cluster, scope *ShellScope, name string) (kube.Cluster, bool) {
	found := false
	for _, item := range cluster.Contexts {
		if item.Name == name {
			found = true
			break
		}
	}
	if !found {
		return cluster, false
	}
	next := cluster
	next.Current = name
	if scope != nil {
		scope.Context = name
	}
	return next, true
}

// ContextArgs returns --context flag pair from shell scope when set.
func ContextArgs(scope *ShellScope) []string {
	if scope == nil || scope.Context == "" {
		return nil
	}
	return []string{"--context", scope.Context}
}
