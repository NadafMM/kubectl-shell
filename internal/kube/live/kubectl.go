package live

import (
	"os"
	"path/filepath"
	"strings"
)

// KubectlBin returns the kubectl executable (or KUBE_SHELL_KUBECTL override).
func KubectlBin() string {
	if v := os.Getenv("KUBE_SHELL_KUBECTL"); v != "" {
		return v
	}
	return "kubectl"
}

func kubectlBinary(allowColorWrapper bool) string {
	name := KubectlBin()
	if !allowColorWrapper && os.Getenv("KUBE_SHELL_NODE_SCRIPT") == "" {
		base := strings.TrimSuffix(strings.ToLower(filepath.Base(name)), ".exe")
		if base == "kubecolor" {
			return "kubectl"
		}
	}
	return name
}
