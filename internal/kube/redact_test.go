package kube

import (
	"strings"
	"testing"
)

func TestRedactLineLiteral(t *testing.T) {
	got := RedactLine("create secret generic t --from-literal=password=secret")
	want := "create secret generic t --from-literal=password=***"
	if got != want {
		t.Fatal(got)
	}
}

func TestRedactArgvKeepsSpacedSecretValue(t *testing.T) {
	got := RedactArgv([]string{"create", "secret", "generic", "t", "--from-literal", "password=one two"})
	joined := strings.Join(got, " ")
	if strings.Contains(joined, "one") || strings.Contains(joined, "two") {
		t.Fatal(got)
	}
	if len(got) != 6 || got[5] != "password=***" {
		t.Fatal(got)
	}
}
