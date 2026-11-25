package kube

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestRunbookKeepsExecutedGet(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	file := filepath.Join(home, "session.md")
	StartRecord(file, "minikube", "default", "2026-11-25T00:00:00.000Z")
	t.Cleanup(func() { StopRecord() })

	NoteRan(RunbookEntry{
		Decision: "refused-confirm", Verb: "delete",
		Timestamp: "2026-11-25T00:00:01.000Z", Context: "minikube", Namespace: "default",
		Command: "kubectl delete pod worker-0", Risk: "high",
	})
	NoteRan(RunbookEntry{
		Decision: "executed", Verb: "got",
		Timestamp: "2026-11-25T00:00:02.000Z", Context: "minikube", Namespace: "default",
		Command: "kubectl got pods", Risk: "read", ExitCode: intPtr(1),
	})
	NoteRan(RunbookEntry{
		Decision: "executed", Verb: "get",
		Timestamp: "2026-11-25T00:00:03.000Z", Context: "minikube", Namespace: "default",
		Command: RedactCommand([]string{"get", "pods", "--from-literal", "password=secret"}),
		Risk:    "read", ExitCode: intPtr(0),
	})
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "context: minikube") || !strings.Contains(text, "namespace: default") {
		t.Fatal(text)
	}
	if !strings.Contains(text, "kubectl get pods --from-literal password=***") {
		t.Fatal(text)
	}
	if strings.Contains(text, "delete") || strings.Contains(text, "got pods") || strings.Contains(text, "password=secret") {
		t.Fatal(text)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(file)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm()&0o777 != 0o600 {
			t.Fatalf("mode %o", info.Mode().Perm())
		}
	}
}

func TestRunbookKeepsBlast(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	file := filepath.Join(home, "session.md")
	StartRecord(file, "minikube", "default", "2026-11-25T00:00:00.000Z")
	t.Cleanup(func() { StopRecord() })
	NoteRan(RunbookEntry{
		Decision: "executed", Verb: "delete",
		Timestamp: "2026-11-25T00:00:04.000Z", Context: "minikube", Namespace: "default",
		Command: "kubectl delete pod worker-0", Risk: "high", ExitCode: intPtr(0),
		Summary: "Deletes pod/worker-0 in default.",
		Blast:   "blast: checkout would have 0 ready pods",
	})
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "exit: 0") || !strings.Contains(text, "Deletes pod/worker-0") || !strings.Contains(text, "0 ready pods") {
		t.Fatal(text)
	}
}

func TestSessionPathNamesContext(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	file := SessionPath("prod/main", time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC))
	if !strings.Contains(file, string(filepath.Separator)+".kube-shell"+string(filepath.Separator)+"sessions"+string(filepath.Separator)) {
		t.Fatal(file)
	}
	if !strings.HasSuffix(file, "prod-main.md") {
		t.Fatal(file)
	}
	if strings.Contains(filepath.Base(file), ":") {
		t.Fatal(file)
	}
}
