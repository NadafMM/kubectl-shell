package kube

import (
	"encoding/json"
	"testing"
)

func TestParsePolicyProdGuard(t *testing.T) {
	parsed, errMsg := ParsePolicy(`{"contexts":{"prod-*":{"mode":"guarded","confirm":"typed","deny":["exec"]}}}`)
	if errMsg != "" {
		t.Fatal(errMsg)
	}
	rule := RuleFor(&parsed, "prod-main")
	if rule.Confirm != "typed" || rule.Mode != "guarded" || len(rule.Deny) != 1 || rule.Deny[0] != "exec" {
		t.Fatal(rule)
	}
	if RuleFor(&parsed, "staging").Confirm != "default" {
		t.Fatal("staging")
	}
	if PromptLabel(true, "prod-main:default$ ") != "[GUARDED] prod-main:default$ " {
		t.Fatal(PromptLabel(true, "prod-main:default$ "))
	}
}

func TestParsePolicyRejectsUnknown(t *testing.T) {
	_, errMsg := ParsePolicy(`{"nope":1}`)
	if errMsg == "" {
		t.Fatal("expected error")
	}
	_ = json.RawMessage(nil)
}
