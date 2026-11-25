package kube

import "os"

// KubectlBin returns the kubectl executable (or KUBE_SHELL_KUBECTL override).
func KubectlBin() string {
	if v := os.Getenv("KUBE_SHELL_KUBECTL"); v != "" {
		return v
	}
	return "kubectl"
}
