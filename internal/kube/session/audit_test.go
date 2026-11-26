package session

import (
	"github.com/NadafMM/kubectl-shell/internal/kube/redact"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuditRedactsAndRotates(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	argv := redact.RedactArgv([]string{"create", "secret", "generic", "t", "--from-literal=password=secret"})
	if strings.Join(argv, " ") != "create secret generic t --from-literal=password=***" {
		t.Fatal(argv)
	}
	entry := AuditEntry{
		ID: "abc", Timestamp: "2026-11-24T00:00:00.000Z", Actor: "human-cli",
		Context: "minikube", Namespace: "default", Argv: argv, Risk: "high",
		Decision: "executed", Approver: "flag-yes", ExitCode: intPtr(0), DurationMs: 1,
	}
	AppendAudit(entry, 80)
	AppendAudit(AuditEntry{
		ID: "def", Timestamp: "2026-11-24T00:00:01.000Z", Actor: "human-cli",
		Context: "minikube", Namespace: "default", Argv: []string{"get", "pods"}, Risk: "read",
		Decision: "executed", Approver: "flag-yes", ExitCode: intPtr(0), DurationMs: 1,
	}, 80)
	file := filepath.Join(home, ".kube-shell", "audit.jsonl")
	if _, err := os.Stat(file + ".1"); err != nil {
		t.Fatal("expected rotation")
	}
	last := TailAudit(1)
	if len(last) != 1 || !strings.Contains(last[0], "get") {
		t.Fatal(last)
	}
	body, _ := os.ReadFile(file)
	if strings.Contains(string(body), "password=secret") {
		t.Fatal(string(body))
	}
}

func intPtr(n int) *int { return &n }
