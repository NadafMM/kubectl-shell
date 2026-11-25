package kube

import (
	"strings"
	"testing"
)

func TestSensitiveStdoutKind(t *testing.T) {
	if SensitiveStdoutKind([]string{"get", "secret", "token", "-o", "json"}) != "secret" {
		t.Fatal("expected secret")
	}
	if SensitiveStdoutKind([]string{"get", "pods"}) != "" {
		t.Fatal("plain get pods")
	}
	if SensitiveStdoutKind([]string{"config", "view", "--raw"}) != "kubeconfig" {
		t.Fatal("expected kubeconfig")
	}
}

func TestRedactKubectlStdoutJSON(t *testing.T) {
	out := RedactKubectlStdout(
		[]string{"get", "secret", "token", "-o", "json"},
		`{"apiVersion":"v1","kind":"Secret","data":{"password":"cGFzcw=="}}`+"\n",
	)
	if !strings.Contains(out, "***") || strings.Contains(out, "cGFzcw==") {
		t.Fatal(out)
	}
}
