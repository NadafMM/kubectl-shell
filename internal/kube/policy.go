package kube

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type ContextRule struct {
	Mode    string   `json:"mode"`
	Confirm string   `json:"confirm"`
	Deny    []string `json:"deny"`
}

type AgentRule struct {
	Allow         []string `json:"allow"`
	WriteRequires string   `json:"write_requires"`
	MatchNonTty   bool     `json:"match_non_tty"`
}

type Policy struct {
	Contexts []struct {
		Pattern string
		Rule    ContextRule
	}
	Agents *AgentRule
	Audit  string
}

var emptyRule = ContextRule{Mode: "normal", Confirm: "default", Deny: []string{}}

// ParsePolicy loads a policy document from JSON text.
func ParsePolicy(text string) (Policy, string) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(text), &raw); err != nil {
		return Policy{}, "policy is not valid JSON"
	}
	for key := range raw {
		if key != "contexts" && key != "agents" && key != "audit" {
			return Policy{}, "unknown policy field " + key
		}
	}
	policy := Policy{Audit: "optional", Contexts: []struct {
		Pattern string
		Rule    ContextRule
	}{}}
	if rawContexts, ok := raw["contexts"]; ok {
		var contexts map[string]json.RawMessage
		if err := json.Unmarshal(rawContexts, &contexts); err != nil || contexts == nil {
			return Policy{}, "contexts must be an object"
		}
		for pattern, value := range contexts {
			rule, errMsg := parseContextRule(value)
			if errMsg != "" {
				return Policy{}, errMsg
			}
			policy.Contexts = append(policy.Contexts, struct {
				Pattern string
				Rule    ContextRule
			}{Pattern: pattern, Rule: rule})
		}
	}
	if rawAgents, ok := raw["agents"]; ok {
		agents, errMsg := parseAgentRule(rawAgents)
		if errMsg != "" {
			return Policy{}, errMsg
		}
		policy.Agents = &agents
	}
	if rawAudit, ok := raw["audit"]; ok {
		var audit string
		if err := json.Unmarshal(rawAudit, &audit); err != nil || (audit != "optional" && audit != "required") {
			return Policy{}, "audit must be required or optional"
		}
		policy.Audit = audit
	}
	return policy, ""
}

func parseContextRule(raw json.RawMessage) (ContextRule, string) {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		return ContextRule{}, "a context rule must be an object"
	}
	for key := range doc {
		if key != "mode" && key != "confirm" && key != "deny" {
			return ContextRule{}, "unknown context field " + key
		}
	}
	rule := ContextRule{Mode: "normal", Confirm: "default", Deny: []string{}}
	if rawMode, ok := doc["mode"]; ok {
		var mode string
		if err := json.Unmarshal(rawMode, &mode); err != nil || (mode != "normal" && mode != "guarded") {
			return ContextRule{}, "mode must be normal or guarded"
		}
		rule.Mode = mode
	}
	if rawConfirm, ok := doc["confirm"]; ok {
		var confirm string
		if err := json.Unmarshal(rawConfirm, &confirm); err != nil || (confirm != "default" && confirm != "typed") {
			return ContextRule{}, "confirm must be typed"
		}
		rule.Confirm = confirm
	}
	if rawDeny, ok := doc["deny"]; ok {
		var deny []string
		if err := json.Unmarshal(rawDeny, &deny); err != nil {
			return ContextRule{}, "deny must be a list of verbs"
		}
		rule.Deny = deny
	}
	return rule, ""
}

func parseAgentRule(raw json.RawMessage) (AgentRule, string) {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		return AgentRule{}, "agents must be an object"
	}
	for key := range doc {
		if key != "allow" && key != "write_requires" && key != "match_non_tty" {
			return AgentRule{}, "unknown agents field " + key
		}
	}
	agents := AgentRule{Allow: []string{}, WriteRequires: "none", MatchNonTty: false}
	if rawAllow, ok := doc["allow"]; ok {
		var allow []string
		if err := json.Unmarshal(rawAllow, &allow); err != nil {
			return AgentRule{}, "agents.allow must be a list of verbs"
		}
		agents.Allow = allow
	}
	if rawWrite, ok := doc["write_requires"]; ok {
		var write string
		if err := json.Unmarshal(rawWrite, &write); err != nil || (write != "none" && write != "approved-plan") {
			return AgentRule{}, "write_requires must be approved-plan"
		}
		agents.WriteRequires = write
	}
	if rawMatch, ok := doc["match_non_tty"]; ok {
		var match bool
		if err := json.Unmarshal(rawMatch, &match); err != nil {
			return AgentRule{}, "match_non_tty must be true or false"
		}
		agents.MatchNonTty = match
	}
	return agents, ""
}

// RuleFor picks the context rule for a kubeconfig context name.
func RuleFor(policy *Policy, context string) ContextRule {
	if policy == nil {
		return emptyRule
	}
	for _, item := range policy.Contexts {
		if item.Pattern == context {
			return item.Rule
		}
	}
	var wild []struct {
		Pattern string
		Rule    ContextRule
	}
	for _, item := range policy.Contexts {
		if strings.HasSuffix(item.Pattern, "*") && strings.HasPrefix(context, strings.TrimSuffix(item.Pattern, "*")) {
			wild = append(wild, item)
		}
	}
	sort.Slice(wild, func(i, j int) bool { return len(wild[i].Pattern) > len(wild[j].Pattern) })
	if len(wild) > 0 {
		return wild[0].Rule
	}
	return emptyRule
}

// PolicyFile returns the path used for the agent or the explicit/env/default file.
func PolicyFile(agent bool, flag, envPath string) string {
	if agent {
		return filepath.Join(StateDir(), "policy.json")
	}
	if flag != "" {
		return flag
	}
	if envPath != "" {
		return envPath
	}
	return filepath.Join(StateDir(), "policy.json")
}

// ReadPolicy loads a policy file. A missing file returns a nil policy.
func ReadPolicy(file string) (*Policy, string) {
	raw, err := os.ReadFile(file)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ""
		}
		return nil, err.Error()
	}
	policy, errMsg := ParsePolicy(string(raw))
	if errMsg != "" {
		return nil, errMsg
	}
	return &policy, ""
}

// PromptLabel prefixes a guarded prompt.
func PromptLabel(guarded bool, body string) string {
	if !guarded {
		return body
	}
	return "[GUARDED] " + body
}
