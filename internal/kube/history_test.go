package kube

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRememberHistoryRedactsAndLoads(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	RememberHistory("create secret generic t --from-literal=password=secret")
	RememberHistory("get pods")
	lines := LoadHistory()
	if len(lines) < 2 {
		t.Fatal(lines)
	}
	// newest first
	if lines[0] != "get pods" {
		t.Fatal(lines)
	}
	joined := lines[1]
	if joined == "" || filepath.Base(historyPath()) != "history" {
		t.Fatal(joined)
	}
	raw, err := os.ReadFile(historyPath())
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) == "" {
		t.Fatal("empty history file")
	}
}
