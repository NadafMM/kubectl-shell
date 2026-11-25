package main

import (
	"bytes"
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

func TestUnportedCommand(t *testing.T) {
	var out, err bytes.Buffer
	if code := run([]string{"get", "pods"}, &out, &err); code != 2 || !strings.Contains(err.String(), "not implemented") {
		t.Fatalf("code %d err %q", code, err.String())
	}
}
