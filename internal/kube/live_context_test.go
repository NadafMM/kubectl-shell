package kube

import "testing"

func TestApplyLiveContextKeepsShellCurrentWhenKubeconfigMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	cluster := Cluster{
		Current:  "prod",
		Contexts: []Context{{Name: "prod", Namespace: "payments"}},
	}
	next := ApplyLiveContext(cluster)
	if next.Current != "prod" {
		t.Fatalf("current changed to %q", next.Current)
	}
}
