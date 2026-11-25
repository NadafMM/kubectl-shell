package kube

import "strings"

// AuthRequest is the shared policy input for CLI, shell, view, and MCP.
type AuthRequest struct {
	Plan           Plan
	Argv           []string
	Policy         *Policy
	Agent          bool
	Yes            bool
	ConfirmContext string
	PlanID         string
	// PolicyContext is the context name used for RuleFor (effective kubectl context).
	PolicyContext string
	// MCP forces plan ids for mutating calls when write_requires is approved-plan.
	MCP bool
}

// AuthResult is the shared policy decision before kubectl runs.
type AuthResult struct {
	OK             bool
	Refuse         string
	AuditDecision  string
	Approver       string
	RequireConfirm bool
	NeedsPlan      bool
	Rule           ContextRule
}

func containsVerb(list []string, verb string) bool {
	for _, item := range list {
		if item == verb {
			return true
		}
	}
	return false
}

func agentAllow(policy *Policy) []string {
	if policy == nil || policy.Agents == nil {
		return nil
	}
	return policy.Agents.Allow
}

func writeRequires(policy *Policy) string {
	if policy == nil || policy.Agents == nil {
		return "none"
	}
	if policy.Agents.WriteRequires == "" {
		return "none"
	}
	return policy.Agents.WriteRequires
}

// effectiveAllow is the verb list enforced for agents/MCP. MCP falls back to
// get/describe/logs/plan when agents.allow is empty or missing.
func effectiveAllow(policy *Policy, mcp bool) []string {
	if mcp {
		return mcpAllow(policy)
	}
	return agentAllow(policy)
}

// Authorize applies deny / agent allow / plan / typed-confirm / risk rules once.
// When RequireConfirm is true the caller must prompt; plan consumption happens here when NeedsPlan.
func Authorize(req AuthRequest, nowConsume func(planID string, expect PlanExpect) (bool, string)) AuthResult {
	ctxName := req.PolicyContext
	if ctxName == "" {
		ctxName = req.Plan.Context
	}
	rule := RuleFor(req.Policy, ctxName)
	result := AuthResult{Rule: rule, Approver: "prompt-y"}

	if containsVerb(rule.Deny, req.Plan.Verb) {
		result.Refuse = "refused: " + req.Plan.Verb + " is denied by policy"
		result.AuditDecision = "denied-policy"
		return result
	}

	requiresPlan := req.Agent && writeRequires(req.Policy) == "approved-plan" && req.Plan.Mutating
	if req.MCP && writeRequires(req.Policy) == "approved-plan" && req.Plan.Mutating {
		requiresPlan = true
	}
	if req.MCP && writeRequires(req.Policy) == "none" && req.Plan.Mutating {
		requiresPlan = false
	}

	allow := effectiveAllow(req.Policy, req.MCP)
	// Allow gates every agent/MCP verb, including writes that also need a plan id.
	if (req.Agent || req.MCP) && allow != nil && !containsVerb(allow, req.Plan.Verb) {
		msg := "refused: " + req.Plan.Verb + " is not allowed for agents"
		if req.MCP {
			msg = "refused: " + req.Plan.Verb + " is not allowed"
		}
		result.Refuse = msg
		result.AuditDecision = "denied-policy"
		return result
	}

	if requiresPlan {
		result.NeedsPlan = true
		if req.PlanID == "" {
			if req.MCP {
				result.Refuse = "refused: mutating commands need a plan id"
			} else {
				result.Refuse = "refused: agent writes need --plan-id from plan --json"
			}
			result.AuditDecision = "denied-policy"
			return result
		}
		ok, reason := nowConsume(req.PlanID, PlanExpect{
			Context: req.Plan.Context, Namespace: req.Plan.Namespace, Argv: req.Argv,
		})
		if !ok {
			result.Refuse = "refused: " + reason
			result.AuditDecision = "denied-policy"
			return result
		}
		if rule.Confirm == "typed" && req.ConfirmContext != ctxName {
			result.Refuse = "refused: pass --confirm-context " + ctxName
			result.AuditDecision = "refused-confirm"
			return result
		}
		result.OK = true
		result.Approver = "plan-id:" + req.PlanID
		result.AuditDecision = "executed"
		return result
	}

	if rule.Confirm == "typed" && req.Yes && req.ConfirmContext != ctxName {
		result.Refuse = "refused: pass --confirm-context " + ctxName
		result.AuditDecision = "refused-confirm"
		return result
	}

	needsConfirm := (req.Plan.Risk == "high" || (rule.Mode == "guarded" && req.Plan.Risk == "low")) && !req.Yes
	if needsConfirm {
		result.OK = true
		result.RequireConfirm = true
		result.AuditDecision = "executed"
		return result
	}

	result.OK = true
	result.AuditDecision = "executed"
	if rule.Confirm == "typed" && req.Yes {
		result.Approver = "typed-context"
	} else if req.Yes {
		result.Approver = "flag-yes"
	}
	return result
}

// EffectiveContext is the context kubectl will use (shell override or plan/cluster).
func EffectiveContext(planContext, scopeContext string) string {
	if strings.TrimSpace(scopeContext) != "" {
		return scopeContext
	}
	return planContext
}

// MatchNonTtyAgent is true when policy asks non-TTY clients to follow agent rules.
func MatchNonTtyAgent(policy *Policy, stdinTTY bool) bool {
	if stdinTTY || policy == nil || policy.Agents == nil {
		return false
	}
	return policy.Agents.MatchNonTty
}
