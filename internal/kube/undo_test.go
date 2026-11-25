package kube

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNamedTargetSetImage(t *testing.T) {
	verb, kind, name, ok := NamedTarget([]string{"set", "image", "deployment/checkout", "nginx=nginx:1.2"})
	if !ok || verb != "set" || kind != "deployment" || name != "checkout" {
		t.Fatalf("%s %s %s %v", verb, kind, name, ok)
	}
}

func TestCleanSnapshotDropsServerFields(t *testing.T) {
	cleaned := CleanSnapshot(map[string]any{
		"kind": "Deployment",
		"metadata": map[string]any{
			"name": "checkout", "resourceVersion": "99", "uid": "u",
			"creationTimestamp": "2020-01-01T00:00:00Z", "generation": 4,
			"managedFields": []any{map[string]any{"manager": "kube"}},
			"annotations":   map[string]any{"deployment.kubernetes.io/revision": "3"},
		},
		"status": map[string]any{"replicas": 1},
	}).(map[string]any)
	if _, ok := cleaned["status"]; ok {
		t.Fatal("status kept")
	}
	meta := cleaned["metadata"].(map[string]any)
	for _, key := range []string{"resourceVersion", "uid", "creationTimestamp", "generation", "managedFields"} {
		if _, ok := meta[key]; ok {
			t.Fatal(key)
		}
	}
}

func TestRememberUndoSecretRefused(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("KUBE_SHELL_KUBECTL", "node")
	t.Setenv("KUBE_SHELL_NODE_SCRIPT", filepath.Join("..", "..", "scripts", "fake-kubectl.mjs"))
	t.Setenv("KUBE_FAKE_FIXTURE", "up")
	t.Setenv("KUBE_FAKE_UNDO", "1")
	cluster := Cluster{Current: "minikube", Contexts: []Context{{Name: "minikube", Namespace: "default"}}}
	RememberUndo([]string{"label", "secret", "token", "env=prod"}, cluster, "sec1", time.Now())
	record, ok := LatestUndo(time.Now())
	if !ok || record.Refused == "" || !strings.Contains(UndoRefusal(record), "Secrets or ConfigMaps") {
		t.Fatalf("%#v", record)
	}
}

func TestRememberUndoDeploymentRevision(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("KUBE_SHELL_KUBECTL", "node")
	t.Setenv("KUBE_SHELL_NODE_SCRIPT", filepath.Join("..", "..", "scripts", "fake-kubectl.mjs"))
	t.Setenv("KUBE_FAKE_FIXTURE", "up")
	t.Setenv("KUBE_FAKE_UNDO", "1")
	cluster := Cluster{Current: "minikube", Contexts: []Context{{Name: "minikube", Namespace: "default"}}}
	RememberUndo([]string{"set", "image", "deployment/checkout", "nginx=nginx:1.2"}, cluster, "dep1", time.Now())
	record, ok := LatestUndo(time.Now())
	if !ok || record.Revision != "3" || record.Snapshot == nil {
		t.Fatalf("%#v", record)
	}
	var snap map[string]any
	if err := json.Unmarshal(record.Snapshot, &snap); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(filepath.Join(home, ".kube-shell", "undo", "dep1.json"))
	if strings.Contains(string(body), "resourceVersion") || strings.Contains(string(body), `"status"`) {
		t.Fatal(string(body))
	}
	args := RestoreArgs(record)
	if args[0] != "rollout" || !containsString(args, "--to-revision=3") || !containsString(args, "--context") {
		t.Fatal(args)
	}
}
