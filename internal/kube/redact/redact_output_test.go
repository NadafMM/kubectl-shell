package redact

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

func TestSensitiveReadWarning(t *testing.T) {
	if SensitiveReadWarning([]string{"get", "secret", "token", "-o", "yaml"}) == "" {
		t.Fatal("expected warning")
	}
	if SensitiveReadWarning([]string{"get", "pods"}) != "" {
		t.Fatal("pods should not warn")
	}
}

func TestRedactStdoutPayloadMutating(t *testing.T) {
	raw := "{\n  \"apiVersion\": \"v1\",\n  \"kind\": \"Secret\",\n  \"data\": {\n    \"password\": \"c2VjcmV0\"\n  }\n}\n"
	got := RedactStdoutPayload(raw)
	if strings.Contains(got, "c2VjcmV0") {
		t.Fatal(got)
	}
	if !strings.Contains(got, "***") {
		t.Fatal(got)
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
