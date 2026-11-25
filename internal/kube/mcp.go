package kube

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

const mcpProtocolVersion = "2024-11-05"

var mcpDefaultAllow = []string{"get", "describe", "logs", "plan"}

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

func mcpTools() []map[string]any {
	emptyObject := map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
	commandProp := map[string]any{
		"type":        "string",
		"description": "kubectl-style command without the kube prefix",
	}
	return []map[string]any{
		{
			"name":        "kube_context",
			"description": "Current context, namespace, and policy summary",
			"inputSchema": emptyObject,
		},
		{
			"name":        "kube_plan",
			"description": "Plan a command and mint a single-use plan id",
			"inputSchema": map[string]any{
				"type":       "object",
				"properties": map[string]any{"command": commandProp},
				"required":   []string{"command"},
			},
		},
		{
			"name":        "kube_execute",
			"description": "Run an allowed read, or a write with a plan id",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"command": commandProp,
					"planId": map[string]any{
						"type":        "string",
						"description": "Single-use id from kube_plan for mutating commands",
					},
				},
				"required": []string{"command"},
			},
		},
	}
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
	case "ping":
		return map[string]any{"jsonrpc": "2.0", "id": id, "result": map[string]any{}}
	case "tools/list":
		return map[string]any{
			"jsonrpc": "2.0",
			"id":      id,
			"result":  map[string]any{"tools": mcpTools()},
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
		mcpRecord(active, cluster, argv, planned.Risk, "planned", &zero, started, "none")
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
	planID, _ := args["planId"].(string)
	auth := Authorize(AuthRequest{
		Plan: planned, Argv: argv, Policy: active, Agent: true, MCP: true,
		PlanID: planID, PolicyContext: cluster.Current, Yes: true,
	}, func(id string, expect PlanExpect) (bool, string) {
		return ConsumePlan(id, expect, time.Now())
	})
	if !auth.OK {
		mcpRecord(active, cluster, argv, planned.Risk, auth.AuditDecision, nil, started, "none")
		return mcpToolResult(id, auth.Refuse, true)
	}
	code, stdout, stderr := CaptureLive(argv)
	if SensitiveStdoutKind(argv) != "" || planned.Mutating {
		stdout = RedactKubectlStdout(argv, stdout)
	}
	mcpRecord(active, cluster, argv, planned.Risk, "executed", &code, started, auth.Approver)
	text := stdout
	if strings.TrimSpace(text) == "" {
		text = stderr
	}
	return mcpToolResult(id, text, code != 0)
}

func writeMcpFrame(out io.Writer, payload []byte) error {
	if _, err := fmt.Fprintf(out, "Content-Length: %d\r\n\r\n", len(payload)); err != nil {
		return err
	}
	_, err := out.Write(payload)
	return err
}

// readMcpFrame reads one Content-Length framed JSON-RPC message from r.
func readMcpFrame(r *bufio.Reader) ([]byte, error) {
	var length int
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		trimmed := strings.TrimRight(line, "\r\n")
		if trimmed == "" {
			break
		}
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "content-length:") {
			n, err := strconv.Atoi(strings.TrimSpace(trimmed[len("Content-Length:"):]))
			if err != nil || n < 0 {
				return nil, fmt.Errorf("invalid Content-Length")
			}
			length = n
		}
	}
	if length == 0 {
		return nil, fmt.Errorf("missing Content-Length")
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}
	return body, nil
}

// ServeMcp streams Content-Length framed JSON-RPC from in until EOF.
// It answers initialize before tools/list so a live MCP client can proceed.
func ServeMcp(cluster Cluster, version string, in io.Reader, out io.Writer) error {
	reader := bufio.NewReader(in)
	for {
		body, err := readMcpFrame(reader)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return nil
		}
		if err != nil {
			return err
		}
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
		if err := writeMcpFrame(out, payload); err != nil {
			return err
		}
	}
}

// FrameMcpMessages is a test helper that builds a multi-message MCP stream.
func FrameMcpMessages(messages ...any) []byte {
	var buf bytes.Buffer
	for _, message := range messages {
		payload, _ := json.Marshal(message)
		_ = writeMcpFrame(&buf, payload)
	}
	return buf.Bytes()
}
