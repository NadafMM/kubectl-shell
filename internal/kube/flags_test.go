package kube

import "testing"

func TestParseLeadingFlagsYes(t *testing.T) {
	parsed := ParseLeadingFlags([]string{"--yes", "get", "pods", "-n", "app"})
	if !parsed.Yes || parsed.Error != "" {
		t.Fatal(parsed)
	}
	if len(parsed.Rest) != 4 || parsed.Rest[0] != "get" || parsed.Rest[3] != "app" {
		t.Fatal(parsed.Rest)
	}
}

func TestParseLeadingFlagsUnknown(t *testing.T) {
	parsed := ParseLeadingFlags([]string{"--bogus", "get"})
	if parsed.Error == "" {
		t.Fatal("expected error")
	}
}

func TestParseLeadingFlagsRecord(t *testing.T) {
	parsed := ParseLeadingFlags([]string{"--record", "out.md", "get", "pods"})
	if parsed.Error != "" || parsed.Record != "out.md" {
		t.Fatal(parsed)
	}
	if len(parsed.Rest) != 2 || parsed.Rest[0] != "get" {
		t.Fatal(parsed.Rest)
	}
}
