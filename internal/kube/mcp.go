package kube

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
)

const mcpProtocolVersion = "2024-11-05"

var (
	mcpDefaultAllow     = []string{"get", "describe", "logs", "plan"}
	regexpContentLength = regexp.MustCompile(`(?i)Content-Length:\s*(\d+)`)
)

// McpRpc is one JSON-RPC request on the MCP stream.
type McpRpc struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id"`
	Method  string `json:"method"`
	Params  *struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	} `json:"params"`
}

func mcpToolResult(id any, text string, isError bool) map[string]any {
	return map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result": map[string]any{
			"content": []map[string]string{{"type": "text", "text": text}},
			"isError": isError,
		},
	}
}

func mcpPolicy() *Policy {
	policy, errMsg := ReadPolicy(PolicyFile(true, "", ""))
	if errMsg != "" {
		return nil
	}
	return policy
}

func mcpRecord(policy *Policy, cluster Cluster, argv []string, risk, decision string, exitCode *int, started time.Time, approver string) {
	if !AuditEnabled(policy) {
		return
	}
	AppendAudit(AuditEntry{
		ID:         NewAuditId(),
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Actor:      "mcp",
		Context:    cluster.Current,
		Namespace:  NamespaceOf(cluster),
		Argv:       RedactArgv(argv),
		Risk:       risk,
		Decision:   decision,
		Approver:   approver,
		ExitCode:   exitCode,
		DurationMs: int(time.Since(started).Milliseconds()),
	}, 0)
}

func mcpPlanned(command string, cluster Cluster) (Plan, []string) {
	view := ToEngine(command, len(command), "repl")
	planned := PlanCommand(view.Line, cluster)
	argv := CommandArgs(view.Line, nil)
	return planned, argv
}

func mcpAllow(policy *Policy) []string {
	if policy != nil && policy.Agents != nil && len(policy.Agents.Allow) > 0 {
		return policy.Agents.Allow
	}
	return append([]string{}, mcpDefaultAllow...)
}

// HandleMcpRpc answers one MCP JSON-RPC message. Notifications (null id) return nil.
func HandleMcpRpc(message McpRpc, cluster Cluster, version string) any {
	if message.ID == nil {
		return nil
	}
	id := message.ID
	switch message.Method {
	case "initialize":
		return map[string]any{
			"jsonrpc": "2.0",
			"id":      id,
			"result": map[string]any{
				"protocolVersion": mcpProtocolVersion,
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]any{"name": "kube", "version": version},
			},
		}
	case "tools/list":
		return map[string]any{
			"jsonrpc": "2.0",
			"id":      id,
			"result": map[string]any{
				"tools": []map[string]string{
					{"name": "kube_context", "description": "Current context, namespace, and policy summary"},
					{"name": "kube_plan", "description": "Plan a command and mint a single-use plan id"},
					{"name": "kube_execute", "description": "Run an allowed read, or a write with a plan id"},
				},
			},
		}
	case "tools/call":
	default:
		return map[string]any{
			"jsonrpc": "2.0",
			"id":      id,
			"error":   map[string]any{"code": -32601, "message": "method not found"},
		}
	}

	name := ""
	args := map[string]any{}
	if message.Params != nil {
		name = message.Params.Name
		if message.Params.Arguments != nil {
			args = message.Params.Arguments
		}
	}
	active := mcpPolicy()
	rule := RuleFor(active, cluster.Current)
	allow := mcpAllow(active)
	if name == "kube_context" {
		payload, _ := json.Marshal(map[string]any{
			"context":   cluster.Current,
			"namespace": NamespaceOf(cluster),
			"guarded":   rule.Mode == "guarded",
			"confirm":   rule.Confirm,
			"allow":     allow,
		})
		return mcpToolResult(id, string(payload), false)
	}
	command, _ := args["command"].(string)
	if command == "" {
		return mcpToolResult(id, "command is required", true)
	}
	planned, argv := mcpPlanned(command, cluster)
	started := time.Now()
	if name == "kube_plan" {
		blast := BlastRadius(planned, argv, func(kubectl []string) KubectlJSONResult {
			return RunJson(kubectl, 3000)
		}, func() int64 { return time.Now().UnixMilli() })
		planID, _ := MintPlan(PlanExpect{Context: planned.Context, Namespace: planned.Namespace, Argv: argv}, time.Now())
		zero := 0
		mcpRecord(active, cluster, argv, planned.Risk, "executed", &zero, started, "none")
		type planPayload struct {
			Plan
			Blast  Blast  `json:"blast"`
			PlanID string `json:"planId"`
			Text   string `json:"text"`
		}
		raw, _ := json.Marshal(planPayload{Plan: planned, Blast: blast, PlanID: planID, Text: FormatBlast(blast)})
		return mcpToolResult(id, string(raw), false)
	}
	if name != "kube_execute" {
		return mcpToolResult(id, "unknown tool", true)
	}
	if containsString(rule.Deny, planned.Verb) {
		mcpRecord(active, cluster, argv, planned.Risk, "denied-policy", nil, started, "none")
		return mcpToolResult(id, "refused: "+planned.Verb+" is denied by policy", true)
	}
	if planned.Mutating {
		typedBlocked := rule.Confirm == "typed" && !containsString(allow, planned.Verb)
		planID, _ := args["planId"].(string)
		if typedBlocked || planID == "" {
			mcpRecord(active, cluster, argv, planned.Risk, "denied-policy", nil, started, "none")
			return mcpToolResult(id, "refused: mutating commands need a plan id", true)
		}
		ok, reason := ConsumePlan(planID, PlanExpect{Context: planned.Context, Namespace: planned.Namespace, Argv: argv}, time.Now())
		if !ok {
			mcpRecord(active, cluster, argv, planned.Risk, "denied-policy", nil, started, "none")
			return mcpToolResult(id, "refused: "+reason, true)
		}
		code, stdout, stderr := CaptureLive(argv)
		mcpRecord(active, cluster, argv, planned.Risk, "executed", &code, started, "plan-id:"+planID)
		text := stdout
		if strings.TrimSpace(text) == "" {
			text = stderr
		}
		return mcpToolResult(id, text, code != 0)
	}
	if !containsString(allow, planned.Verb) {
		mcpRecord(active, cluster, argv, planned.Risk, "denied-policy", nil, started, "none")
		return mcpToolResult(id, "refused: "+planned.Verb+" is not allowed", true)
	}
	code, stdout, _ := CaptureLive(argv)
	if SensitiveStdoutKind(argv) != "" {
		stdout = RedactKubectlStdout(argv, stdout)
	}
	mcpRecord(active, cluster, argv, planned.Risk, "executed", &code, started, "none")
	return mcpToolResult(id, stdout, code != 0)
}

// ServeMcp reads Content-Length framed JSON-RPC from in and writes framed replies to out.
func ServeMcp(cluster Cluster, version string, in io.Reader, out io.Writer) error {
	raw, err := io.ReadAll(in)
	if err != nil {
		return err
	}
	offset := 0
	for offset+4 < len(raw) {
		rel := bytes.Index(raw[offset:], []byte("\r\n\r\n"))
		if rel < 0 {
			break
		}
		headerEnd := offset + rel
		header := string(raw[offset:headerEnd])
		match := regexpContentLength.FindStringSubmatch(header)
		if match == nil {
			break
		}
		var length int
		if _, err := fmt.Sscanf(match[1], "%d", &length); err != nil {
			break
		}
		start := headerEnd + 4
		if start+length > len(raw) {
			break
		}
		body := raw[start : start+length]
		offset = start + length
		var message McpRpc
		if err := json.Unmarshal(body, &message); err != nil {
			continue
		}
		response := HandleMcpRpc(message, cluster, version)
		if response == nil {
			continue
		}
		payload, err := json.Marshal(response)
		if err != nil {
			continue
		}
		if _, err := fmt.Fprintf(out, "Content-Length: %d\r\n\r\n", len(payload)); err != nil {
			return err
		}
		if _, err := out.Write(payload); err != nil {
			return err
		}
	}
	return nil
}
