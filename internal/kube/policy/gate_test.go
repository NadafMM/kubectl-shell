package policy

import (
	"github.com/NadafMM/kubectl-shell/internal/kube"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestConsumePlanRejectsPathEscape(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	policyPath := filepath.Join(home, ".kube-shell", "policy.json")
	if err := os.MkdirAll(filepath.Dir(policyPath), 0o700); err != nil {
		t.Fatal(err)
	}
	body := `{"contexts":{"prod":{"deny":["delete"]}}}`
	if err := os.WriteFile(policyPath, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	expect := PlanExpect{Context: "minikube", Namespace: "default", Argv: []string{"delete", "pod", "x"}}
	if ok, _ := ConsumePlan("../policy", expect, time.Now()); ok {
		t.Fatal("escaped plan id accepted")
	}
	if ok, _ := ConsumePlan("../../../etc/passwd", expect, time.Now()); ok {
		t.Fatal("escaped plan id accepted")
	}
	raw, err := os.ReadFile(policyPath)
	if err != nil || string(raw) != body {
		t.Fatalf("policy file was damaged: %v %q", err, raw)
	}
}

func TestAuthorizeDenyAndAgentAllow(t *testing.T) {
	policy, errMsg := ParsePolicy(`{"contexts":{"prod":{"deny":["delete"]}},"agents":{"allow":["get"],"write_requires":"approved-plan"}}`)
	if errMsg != "" {
		t.Fatal(errMsg)
	}
	plan := kube.Plan{Verb: "delete", Mutating: true, Risk: "high", Context: "prod", Namespace: "default"}
	auth := Authorize(AuthRequest{
		Plan: plan, Argv: []string{"delete", "pod", "x"}, Policy: &policy,
		PolicyContext: "prod", Agent: true,
	}, nil)
	if auth.OK || auth.Refuse == "" {
		t.Fatalf("%#v", auth)
	}

	read := kube.Plan{Verb: "get", Mutating: false, Risk: "read", Context: "prod", Namespace: "default"}
	auth = Authorize(AuthRequest{
		Plan: read, Argv: []string{"get", "pods"}, Policy: &policy,
		PolicyContext: "prod", Agent: true,
	}, nil)
	if !auth.OK {
		t.Fatal(auth.Refuse)
	}
	auth = Authorize(AuthRequest{
		Plan: kube.Plan{Verb: "describe", Risk: "read", Context: "prod"}, Argv: []string{"describe", "pod", "x"},
		Policy: &policy, PolicyContext: "prod", Agent: true,
	}, nil)
	if auth.OK {
		t.Fatal("describe should be denied by allow list")
	}
}

func TestAuthorizeRequiresPlanID(t *testing.T) {
	policy, errMsg := ParsePolicy(`{"agents":{"allow":["get","delete"],"write_requires":"approved-plan"}}`)
	if errMsg != "" {
		t.Fatal(errMsg)
	}
	auth := Authorize(AuthRequest{
		Plan: kube.Plan{Verb: "delete", Mutating: true, Risk: "high", Context: "c", Namespace: "default"},
		Argv: []string{"delete", "pod", "x"}, Policy: &policy, Agent: true, PolicyContext: "c",
	}, func(string, PlanExpect) (bool, string) { return false, "unused" })
	if auth.OK || auth.Refuse == "" {
		t.Fatalf("%#v", auth)
	}
}

func TestAuthorizeAllowGatesPlanWrites(t *testing.T) {
	policy, errMsg := ParsePolicy(`{"agents":{"allow":["get"],"write_requires":"approved-plan"}}`)
	if errMsg != "" {
		t.Fatal(errMsg)
	}
	auth := Authorize(AuthRequest{
		Plan: kube.Plan{Verb: "delete", Mutating: true, Risk: "high", Context: "c", Namespace: "default"},
		Argv: []string{"delete", "pod", "x"}, Policy: &policy, Agent: true, PolicyContext: "c",
		PlanID: "aabbccddeeff00112233445566778899",
	}, func(string, PlanExpect) (bool, string) { return true, "" })
	if auth.OK || !strings.Contains(auth.Refuse, "not allowed") {
		t.Fatalf("delete outside allow should fail: %#v", auth)
	}
}

func TestAuthorizePlanStillNeedsTypedContext(t *testing.T) {
	policy, errMsg := ParsePolicy(`{"contexts":{"prod":{"confirm":"typed"}},"agents":{"allow":["delete"],"write_requires":"approved-plan"}}`)
	if errMsg != "" {
		t.Fatal(errMsg)
	}
	auth := Authorize(AuthRequest{
		Plan: kube.Plan{Verb: "delete", Mutating: true, Risk: "high", Context: "prod", Namespace: "default"},
		Argv: []string{"delete", "pod", "x"}, Policy: &policy, Agent: true, PolicyContext: "prod",
		PlanID: "aabbccddeeff00112233445566778899", Yes: true,
	}, func(string, PlanExpect) (bool, string) { return true, "" })
	if auth.OK || !strings.Contains(auth.Refuse, "confirm-context") {
		t.Fatalf("typed confirm should still apply: %#v", auth)
	}
	auth = Authorize(AuthRequest{
		Plan: kube.Plan{Verb: "delete", Mutating: true, Risk: "high", Context: "prod", Namespace: "default"},
		Argv: []string{"delete", "pod", "x"}, Policy: &policy, Agent: true, PolicyContext: "prod",
		PlanID: "aabbccddeeff00112233445566778899", Yes: true, ConfirmContext: "prod",
	}, func(string, PlanExpect) (bool, string) { return true, "" })
	if !auth.OK {
		t.Fatal(auth.Refuse)
	}
}
