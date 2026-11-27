package session

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/NadafMM/kubectl-shell/internal/kube"
	"k8s.io/client-go/tools/clientcmd"
)

const scopeTTL = 24 * time.Hour

func scopesDir() string {
	return filepath.Join(kube.StateDir(), "scopes")
}

func sanitizeScopeID(id string) string {
	safe := strings.Trim(sanitizeContext(id), "-")
	if safe == "" {
		return "default"
	}
	return safe
}

func baseKubeconfigRules() *clientcmd.ClientConfigLoadingRules {
	if base := os.Getenv("KUBE_SHELL_BASE_KUBECONFIG"); base != "" {
		rules := clientcmd.NewDefaultClientConfigLoadingRules()
		rules.ExplicitPath = base
		return rules
	}
	return clientcmd.NewDefaultClientConfigLoadingRules()
}

// WriteScopedKubeconfig writes an isolated kubeconfig under ~/.kube-shell/scopes/
// with the requested context and namespace without mutating ~/.kube/config.
func WriteScopedKubeconfig(contextName, namespace, sessionID string, now time.Time) (path string, resolvedCtx string, resolvedNS string, err error) {
	cfg, err := baseKubeconfigRules().Load()
	if err != nil || cfg == nil {
		return "", "", "", fmt.Errorf("could not load kubeconfig: %v", err)
	}
	targetCtx := strings.TrimSpace(contextName)
	if targetCtx == "" {
		targetCtx = cfg.CurrentContext
	}
	ctxObj, ok := cfg.Contexts[targetCtx]
	if !ok || ctxObj == nil {
		return "", "", "", fmt.Errorf("context %q not found in kubeconfig", targetCtx)
	}
	// Copy the Context struct so we don't mutate shared pointers unexpectedly.
	copied := *ctxObj
	if ns := strings.TrimSpace(namespace); ns != "" {
		copied.Namespace = ns
	}
	if copied.Namespace == "" {
		copied.Namespace = "default"
	}
	cfg.Contexts[targetCtx] = &copied
	cfg.CurrentContext = targetCtx

	dir := scopesDir()
	if mkErr := os.MkdirAll(dir, 0o700); mkErr != nil {
		return "", "", "", mkErr
	}
	_ = os.Chmod(dir, 0o700)
	_ = os.Chmod(kube.StateDir(), 0o700)

	if sessionID == "" {
		sessionID = fmt.Sprintf("pid-%d", os.Getpid())
	}
	path = filepath.Join(dir, sanitizeScopeID(sessionID)+".yaml")
	if wErr := clientcmd.WriteToFile(*cfg, path); wErr != nil {
		return "", "", "", wErr
	}
	_ = os.Chmod(path, 0o600)
	pruneScopeFiles(dir, path, now)
	return path, targetCtx, copied.Namespace, nil
}

func pruneScopeFiles(dir, keep string, now time.Time) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		full := filepath.Join(dir, entry.Name())
		if full == keep {
			continue
		}
		if info, err := entry.Info(); err == nil && now.Sub(info.ModTime()) > scopeTTL {
			_ = os.Remove(full)
		}
	}
}

// FormatScopeEnv returns shell commands to activate a scoped kubeconfig in the caller's shell.
func FormatScopeEnv(shell, scopedPath, basePath string) string {
	switch strings.ToLower(strings.TrimSpace(shell)) {
	case "", "bash", "zsh", "sh":
		var b strings.Builder
		if basePath != "" {
			b.WriteString("export KUBE_SHELL_BASE_KUBECONFIG='" + strings.ReplaceAll(basePath, "'", "'\"'\"'") + "'\n")
		}
		b.WriteString("export KUBECONFIG='" + strings.ReplaceAll(scopedPath, "'", "'\"'\"'") + "'\n")
		return b.String()
	case "fish":
		var b strings.Builder
		if basePath != "" {
			b.WriteString("set -gx KUBE_SHELL_BASE_KUBECONFIG '" + strings.ReplaceAll(basePath, "'", "\\'") + "'\n")
		}
		b.WriteString("set -gx KUBECONFIG '" + strings.ReplaceAll(scopedPath, "'", "\\'") + "'\n")
		return b.String()
	case "powershell", "pwsh":
		var b strings.Builder
		if basePath != "" {
			b.WriteString("$env:KUBE_SHELL_BASE_KUBECONFIG = '" + strings.ReplaceAll(basePath, "'", "''") + "'\n")
		}
		b.WriteString("$env:KUBECONFIG = '" + strings.ReplaceAll(scopedPath, "'", "''") + "'\n")
		return b.String()
	default:
		return ""
	}
}
