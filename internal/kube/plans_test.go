package kube

import (
	"testing"
	"time"
)

func TestMintAndConsumePlan(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	now := time.Now()
	expect := PlanExpect{Context: "minikube", Namespace: "default", Argv: []string{"delete", "pod", "checkout"}}
	id, err := MintPlan(expect, now)
	if err != nil || len(id) != 32 {
		t.Fatal(err, id)
	}
	if ok, reason := ConsumePlan(id, expect, now); !ok {
		t.Fatal(reason)
	}
	if ok, reason := ConsumePlan(id, expect, now); ok || reason != "plan expired or unknown" {
		t.Fatalf("second consume ok=%v %s", ok, reason)
	}
	id, err = MintPlan(expect, now)
	if err != nil {
		t.Fatal(err)
	}
	other := expect
	other.Argv = []string{"get", "pods"}
	if ok, reason := ConsumePlan(id, other, now); ok || reason != "plan does not match this command" {
		t.Fatalf("mismatch ok=%v %s", ok, reason)
	}
	if ok, _ := ConsumePlan(id, expect, now.Add(6*time.Minute)); ok {
		t.Fatal("expired plan was accepted")
	}
}

func TestValidPlanID(t *testing.T) {
	if !ValidPlanID("0123456789abcdef0123456789abcdef") {
		t.Fatal("expected valid")
	}
	if ValidPlanID("../policy") || ValidPlanID("abc") || ValidPlanID("0123456789ABCDEF0123456789abcdef") {
		t.Fatal("expected invalid")
	}
}
