package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersion(t *testing.T) {
	var out, err bytes.Buffer
	if code := run([]string{"--version"}, &out, &err); code != 0 || strings.TrimSpace(out.String()) != version {
		t.Fatalf("code %d out %q err %q", code, out.String(), err.String())
	}
}

func TestCompletionBash(t *testing.T) {
	var out, err bytes.Buffer
	if code := run([]string{"completion", "bash"}, &out, &err); code != 0 || !strings.Contains(out.String(), "mapfile -t COMPREPLY") {
		t.Fatalf("code %d out %q", code, out.String())
	}
}

func TestCompletionUsage(t *testing.T) {
	var out, err bytes.Buffer
	if code := run([]string{"completion"}, &out, &err); code != 1 || !strings.Contains(err.String(), "bash|zsh|fish") {
		t.Fatalf("code %d err %q", code, err.String())
	}
}

func TestCompletePods(t *testing.T) {
	line := "kube get po"
	var out, err bytes.Buffer
	code := run([]string{"__complete", "--cursor", "10", "--line", line}, &out, &err)
	if code != 0 || !strings.Contains(out.String(), "pods") {
		t.Fatalf("code %d out %q err %q", code, out.String(), err.String())
	}
}

func TestCompleteLiveDeployment(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("KUBE_SHELL_KUBECTL", "node")
	t.Setenv("KUBE_SHELL_NODE_SCRIPT", filepath.Join("..", "..", "scripts", "fake-kubectl.mjs"))
	t.Setenv("KUBE_FAKE_FIXTURE", "up")
	t.Setenv("KUBE_FAKE_LOG", filepath.Join(home, "kubectl.log"))
	t.Setenv("KUBE_FAKE_FAIL", "")
	line := "kube get deploy "
	var out, err bytes.Buffer
	code := run([]string{"__complete", "--cursor", "16", "--line", line}, &out, &err)
	if code != 0 || !strings.Contains(out.String(), "web") {
		t.Fatalf("code %d out %q err %q", code, out.String(), err.String())
	}
}

func TestPlanDelete(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("KUBE_SHELL_KUBECTL", "node")
	t.Setenv("KUBE_SHELL_NODE_SCRIPT", filepath.Join("..", "..", "scripts", "fake-kubectl.mjs"))
	t.Setenv("KUBE_FAKE_FIXTURE", "up")
	t.Setenv("KUBE_FAKE_FAIL", "")
	var out, err bytes.Buffer
	code := run([]string{"plan", "delete", "pod", "checkout"}, &out, &err)
	if code != 0 || !strings.Contains(out.String(), "risk high") || !strings.Contains(out.String(), "pod/checkout") {
		t.Fatalf("code %d out %q err %q", code, out.String(), err.String())
	}
	out.Reset()
	err.Reset()
	code = run([]string{"plan", "--json", "get", "pods"}, &out, &err)
	if code != 0 || !strings.Contains(out.String(), `"risk":"read"`) || !strings.Contains(out.String(), `"planId"`) {
		t.Fatalf("code %d out %q err %q", code, out.String(), err.String())
	}
}

func TestPlanBlastSkippedForGet(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("KUBE_SHELL_KUBECTL", "node")
	t.Setenv("KUBE_SHELL_NODE_SCRIPT", filepath.Join("..", "..", "scripts", "fake-kubectl.mjs"))
	t.Setenv("KUBE_FAKE_FIXTURE", "up")
	t.Setenv("KUBE_FAKE_FAIL", "")
	var out, err bytes.Buffer
	code := run([]string{"plan", "--json", "get", "pods"}, &out, &err)
	if code != 0 || strings.Contains(out.String(), `"blast"`) {
		t.Fatalf("code %d out %q err %q", code, out.String(), err.String())
	}
}

func TestAskContext(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("KUBE_SHELL_KUBECTL", "node")
	t.Setenv("KUBE_SHELL_NODE_SCRIPT", filepath.Join("..", "..", "scripts", "fake-kubectl.mjs"))
	t.Setenv("KUBE_FAKE_FIXTURE", "up")
	t.Setenv("KUBE_FAKE_FAIL", "")
	var out, err bytes.Buffer
	code := run([]string{"ask", "which", "cluster"}, &out, &err)
	if code != 0 || !strings.Contains(out.String(), "minikube") {
		t.Fatalf("code %d out %q err %q", code, out.String(), err.String())
	}
}

func TestUnportedCommand(t *testing.T) {
	var out, err bytes.Buffer
	if code := run([]string{"get", "pods"}, &out, &err); code != 2 || !strings.Contains(err.String(), "not implemented") {
		t.Fatalf("code %d err %q", code, err.String())
	}
}
