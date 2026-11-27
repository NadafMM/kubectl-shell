package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/NadafMM/kubectl-shell/internal/kube"
	"github.com/NadafMM/kubectl-shell/internal/kube/live"
	"github.com/NadafMM/kubectl-shell/internal/kube/parse"
	"github.com/NadafMM/kubectl-shell/internal/kube/plan"
	kpolicy "github.com/NadafMM/kubectl-shell/internal/kube/policy"
	"github.com/NadafMM/kubectl-shell/internal/kube/redact"
	ksession "github.com/NadafMM/kubectl-shell/internal/kube/session"
)

const mcpProtocolVersion = "2024-11-05"

var regexpContentLength = regexp.MustCompile(`(?i)Content-Length:\s*(\d+)`)

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

// McpOptions scopes or restricts an MCP server session.
type McpOptions struct {
	ReadOnly  bool
	Context   string
	Namespace string
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

func mcpPolicy() *kpolicy.Policy {
	policy, errMsg := kpolicy.ReadPolicy(kpolicy.PolicyFile(true, "", ""))
	if errMsg != "" {
		return nil
	}
	return policy
}

func mcpRecord(policy *kpolicy.Policy, cluster kube.Cluster, argv []string, risk, decision string, exitCode *int, started time.Time, approver string) {
	if !ksession.AuditEnabled(policy) {
		return
	}
	ksession.AppendAudit(ksession.AuditEntry{
		ID:         ksession.NewAuditId(),
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Actor:      "mcp",
		Context:    cluster.Current,
		Namespace:  kube.NamespaceOf(cluster),
		Argv:       redact.RedactArgv(argv),
		Risk:       risk,
		Decision:   decision,
		Approver:   approver,
		ExitCode:   exitCode,
		DurationMs: int(time.Since(started).Milliseconds()),
	}, 0)
}

func mcpPlanned(command string, cluster kube.Cluster) (kube.Plan, []string) {
	view := parse.ToEngine(command, len(command), "repl")
	planned := plan.PlanCommand(view.Line, cluster)
	argv := ksession.CommandArgs(view.Line, nil)
	return planned, argv
}

func readOnlyAllow(allow []string) []string {
	out := make([]string, 0, len(allow))
	for _, verb := range allow {
		switch verb {
		case "get", "describe", "logs", "plan", "explain", "top", "api-resources", "api-versions", "version", "cluster-info", "diff":
			out = append(out, verb)
		}
	}
	if len(out) == 0 {
		return kpolicy.McpAllow(nil)
	}
	return out
}

func hasJSONOutput(argv []string) bool {
	for i := 0; i < len(argv); i++ {
		tok := argv[i]
		if (tok == "-o" || tok == "--output") && i+1 < len(argv) {
			if argv[i+1] == "json" {
				return true
			}
		}
		if tok == "-o=json" || tok == "--output=json" {
			return true
		}
	}
	return false
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
			"description": "Current context, namespace, readOnly mode, and policy summary",
			"inputSchema": emptyObject,
		},
		{
			"name":        "kube_plan",
			"description": "Plan a command and mint a single-use plan id (unless --read-only)",
			"inputSchema": map[string]any{
				"type":       "object",
				"properties": map[string]any{"command": commandProp},
				"required":   []string{"command"},
			},
		},
		{
			"name":        "kube_diagnose",
			"description": "Diagnose failing pods, 503 services, or stuck rollouts without sending data to an external LLM",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"question": map[string]any{
						"type":        "string",
						"description": "Symptom or target (for example: why is svc/checkout returning 503, or why is pod/api crashing)",
					},
					"namespace": map[string]any{
						"type":        "string",
						"description": "Optional namespace override",
					},
				},
				"required": []string{"question"},
			},
		},
		{
			"name":        "kube_namespace_health",
			"description": "Deterministic namespace health sweep (failing pods, broken services, stuck deployments, exhausted quotas)",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"namespace": map[string]any{
						"type":        "string",
						"description": "Target namespace (defaults to current context namespace)",
					},
				},
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
func HandleMcpRpc(message McpRpc, cluster kube.Cluster, version string) any {
	return HandleMcpRpcWithOptions(message, cluster, version, McpOptions{})
}

// HandleMcpRpcWithOptions answers one MCP JSON-RPC message under optional read-only and scope constraints.
func HandleMcpRpcWithOptions(message McpRpc, cluster kube.Cluster, version string, opts McpOptions) any {
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

	if opts.Context != "" {
		if next, ok := ksession.UseContext(cluster, nil, opts.Context); ok {
			cluster = next
		} else {
			cluster.Current = opts.Context
		}
	}
	if opts.Namespace != "" {
		cluster = ksession.UseNamespace(cluster, nil, opts.Namespace)
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
	rule := kpolicy.RuleFor(active, cluster.Current)
	allow := kpolicy.McpAllow(active)
	if opts.ReadOnly {
		allow = readOnlyAllow(allow)
	}
	if name == "kube_context" {
		payload, _ := json.Marshal(map[string]any{
			"context":   cluster.Current,
			"namespace": kube.NamespaceOf(cluster),
			"guarded":   rule.Mode == "guarded",
			"confirm":   rule.Confirm,
			"readOnly":  opts.ReadOnly,
			"allow":     allow,
		})
		return mcpToolResult(id, string(payload), false)
	}
	if name == "kube_namespace_health" {
		ns := kube.NamespaceOf(cluster)
		if reqNS, _ := args["namespace"].(string); strings.TrimSpace(reqNS) != "" {
			if opts.Namespace != "" && strings.TrimSpace(reqNS) != opts.Namespace {
				return mcpToolResult(id, "refused: mcp session is pinned to namespace "+opts.Namespace, true)
			}
			ns = strings.TrimSpace(reqNS)
		}
		var ctxArgs []string
		if opts.Context != "" {
			ctxArgs = []string{"--context", opts.Context}
		}
		withCtx := func(a []string) []string {
			if len(ctxArgs) == 0 {
				return a
			}
			return append(append([]string{}, a...), ctxArgs...)
		}
		started := time.Now()
		health := plan.DiagnoseNamespace(ns, func(a []string) (bool, any) {
			res := live.RunJson(withCtx(a), 3000)
			if !res.OK {
				return false, nil
			}
			var parsed any
			if err := json.Unmarshal(res.JSON, &parsed); err != nil {
				return false, nil
			}
			return true, parsed
		}, func(a []string) string {
			_, out, _ := live.CaptureLive(withCtx(a))
			return out
		})
		zero := 0
		mcpRecord(active, cluster, []string{"namespace-health", ns}, "read", "executed", &zero, started, "none")
		raw, _ := json.Marshal(health)
		return mcpToolResult(id, string(raw), false)
	}
	if name == "kube_diagnose" {
		question, _ := args["question"].(string)
		if strings.TrimSpace(question) == "" {
			return mcpToolResult(id, "question is required", true)
		}
		targetCluster := cluster
		if ns, _ := args["namespace"].(string); strings.TrimSpace(ns) != "" {
			if opts.Namespace != "" && strings.TrimSpace(ns) != opts.Namespace {
				return mcpToolResult(id, "refused: mcp session is pinned to namespace "+opts.Namespace, true)
			}
			targetCluster = ksession.UseNamespace(targetCluster, nil, strings.TrimSpace(ns))
		}
		var ctxArgs []string
		if opts.Context != "" {
			ctxArgs = []string{"--context", opts.Context}
		}
		started := time.Now()
		advice := plan.AskAdvice(question, targetCluster, ctxArgs)
		zero := 0
		mcpRecord(active, targetCluster, []string{"ask", question}, "read", "executed", &zero, started, "none")
		raw, _ := json.Marshal(advice)
		return mcpToolResult(id, string(raw), false)
	}
	command, _ := args["command"].(string)
	if command == "" {
		return mcpToolResult(id, "command is required", true)
	}
	planned, argv := mcpPlanned(command, cluster)
	if opts.Context != "" && planned.Context != opts.Context {
		return mcpToolResult(id, "refused: mcp session is pinned to context "+opts.Context, true)
	}
	if opts.Namespace != "" && (planned.AllNamespaces || planned.Namespace != opts.Namespace) {
		return mcpToolResult(id, "refused: mcp session is pinned to namespace "+opts.Namespace, true)
	}
	if opts.Context != "" {
		hasCtx := false
		for _, t := range argv {
			if t == "--context" || strings.HasPrefix(t, "--context=") {
				hasCtx = true
				break
			}
		}
		if !hasCtx {
			argv = append(argv, "--context", opts.Context)
		}
	}
	if opts.Namespace != "" {
		hasNS := false
		for _, t := range argv {
			if t == "-n" || t == "--namespace" || strings.HasPrefix(t, "-n=") || strings.HasPrefix(t, "--namespace=") {
				hasNS = true
				break
			}
		}
		if !hasNS {
			argv = append(argv, "-n", opts.Namespace)
		}
	}
	started := time.Now()
	if name == "kube_plan" {
		blast := plan.BlastRadiusWithOptions(planned, argv, func(kubectl []string) live.KubectlJSONResult {
			return live.RunJson(kubectl, 3000)
		}, func() int64 { return time.Now().UnixMilli() }, plan.BlastOptions{
			RunDiff: func(args []string) (int, string, string) {
				return live.RunDiff(args, 3000)
			},
		})
		planID := ""
		if !opts.ReadOnly {
			planID, _ = kpolicy.MintPlan(kpolicy.PlanExpect{Context: planned.Context, Namespace: planned.Namespace, Argv: argv}, time.Now())
		}
		zero := 0
		mcpRecord(active, cluster, argv, planned.Risk, "planned", &zero, started, "none")
		type planPayload struct {
			kube.Plan
			Blast    plan.Blast `json:"blast"`
			PlanID   string     `json:"planId,omitempty"`
			ReadOnly bool       `json:"readOnly,omitempty"`
			Text     string     `json:"text"`
		}
		raw, _ := json.Marshal(planPayload{Plan: planned, Blast: blast, PlanID: planID, ReadOnly: opts.ReadOnly, Text: plan.FormatBlast(blast)})
		return mcpToolResult(id, string(raw), false)
	}
	if name != "kube_execute" {
		return mcpToolResult(id, "unknown tool", true)
	}
	if opts.ReadOnly && planned.Mutating {
		mcpRecord(active, cluster, argv, planned.Risk, "denied-policy", nil, started, "none")
		return mcpToolResult(id, "refused: server started in --read-only mode", true)
	}
	planID, _ := args["planId"].(string)
	auth := kpolicy.Authorize(kpolicy.AuthRequest{
		Plan: planned, Argv: argv, Policy: active, Agent: true, MCP: true,
		PlanID: planID, PolicyContext: cluster.Current, Yes: true,
	}, func(id string, expect kpolicy.PlanExpect) (bool, string) {
		return kpolicy.ConsumePlan(id, expect, time.Now())
	})
	if !auth.OK {
		mcpRecord(active, cluster, argv, planned.Risk, auth.AuditDecision, nil, started, "none")
		return mcpToolResult(id, auth.Refuse, true)
	}
	var code int
	var stdout, stderr string
	if !planned.Mutating && hasJSONOutput(argv) {
		if res, handled := live.RunJsonDirect(argv, 4000); handled {
			if res.OK {
				code = 0
				stdout = string(res.JSON) + "\n"
			} else {
				code = 1
				stderr = res.Error
			}
		} else {
			code, stdout, stderr = live.CaptureLive(argv)
		}
	} else {
		code, stdout, stderr = live.CaptureLive(argv)
	}
	if redact.SensitiveStdoutKind(argv) != "" {
		stdout = redact.RedactKubectlStdout(argv, stdout)
	} else if planned.Mutating {
		stdout = redact.RedactStdoutPayload(stdout)
	}
	if !planned.Mutating {
		stdout = DistillMcpJSON(stdout)
	}
	mcpRecord(active, cluster, argv, planned.Risk, "executed", &code, started, auth.Approver)
	text := stdout
	if strings.TrimSpace(text) == "" {
		text = stderr
	}
	return mcpToolResult(id, text, code != 0)
}

// DistillMcpJSON removes high-noise fields (managedFields, last-applied-configuration)
// from Kubernetes JSON payloads returned over MCP.
func DistillMcpJSON(stdout string) string {
	trimmed := strings.TrimSpace(stdout)
	if !strings.HasPrefix(trimmed, "{") && !strings.HasPrefix(trimmed, "[") {
		return stdout
	}
	var parsed any
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return stdout
	}
	cleaned := stripNoisyKubeFields(parsed)
	encoded, err := json.Marshal(cleaned)
	if err != nil {
		return stdout
	}
	return string(encoded) + "\n"
}

func stripNoisyKubeFields(value any) any {
	switch v := value.(type) {
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = stripNoisyKubeFields(item)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, child := range v {
			if k == "managedFields" {
				continue
			}
			if k == "annotations" {
				if ann, ok := child.(map[string]any); ok {
					filtered := make(map[string]any, len(ann))
					for ak, av := range ann {
						if ak == "kubectl.kubernetes.io/last-applied-configuration" {
							continue
						}
						filtered[ak] = av
					}
					if len(filtered) > 0 {
						out[k] = filtered
					}
					continue
				}
			}
			out[k] = stripNoisyKubeFields(child)
		}
		return out
	default:
		return value
	}
}

func flushWriter(out io.Writer) {
	if flusher, ok := out.(interface{ Flush() error }); ok {
		_ = flusher.Flush()
	}
}

func writeMcpFrame(out io.Writer, payload []byte, contentLength bool) error {
	if contentLength {
		if _, err := fmt.Fprintf(out, "Content-Length: %d\r\n\r\n", len(payload)); err != nil {
			return err
		}
		if _, err := out.Write(payload); err != nil {
			return err
		}
		flushWriter(out)
		return nil
	}
	if _, err := out.Write(append(payload, '\n')); err != nil {
		return err
	}
	flushWriter(out)
	return nil
}

// ServeMcp streams JSON-RPC over stdio (NDJSON standard MCP framing, plus
// Content-Length framing for legacy LSP-style callers) using a fixed cluster.
func ServeMcp(cluster kube.Cluster, version string, in io.Reader, out io.Writer) error {
	return ServeMcpLive(func() kube.Cluster { return cluster }, version, in, out)
}

// ServeMcpLive streams JSON-RPC over stdio, resolving current context dynamically per call.
func ServeMcpLive(clusterFn func() kube.Cluster, version string, in io.Reader, out io.Writer) error {
	return ServeMcpOptions(clusterFn, version, in, out, McpOptions{})
}

// ServeMcpOptions streams JSON-RPC over stdio under the given McpOptions.
func ServeMcpOptions(clusterFn func() kube.Cluster, version string, in io.Reader, out io.Writer, opts McpOptions) error {
	reader := bufio.NewReader(in)
	for {
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return err
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if err == io.EOF {
				return nil
			}
			continue
		}

		// Standard MCP stdio transport: Newline-Delimited JSON (NDJSON).
		if strings.HasPrefix(trimmed, "{") {
			var message McpRpc
			if uerr := json.Unmarshal([]byte(trimmed), &message); uerr != nil {
				parseErr, _ := json.Marshal(map[string]any{
					"jsonrpc": "2.0",
					"id":      nil,
					"error":   map[string]any{"code": -32700, "message": "parse error"},
				})
				if werr := writeMcpFrame(out, parseErr, false); werr != nil {
					return werr
				}
				if err == io.EOF {
					return nil
				}
				continue
			}
			response := HandleMcpRpcWithOptions(message, clusterFn(), version, opts)
			if response != nil {
				payload, merr := json.Marshal(response)
				if merr == nil {
					if werr := writeMcpFrame(out, payload, false); werr != nil {
						return werr
					}
				}
			}
			if err == io.EOF {
				return nil
			}
			continue
		}

		// Legacy Content-Length header framing.
		headerBlock := line
		if err == nil {
			for {
				nextLine, herr := reader.ReadString('\n')
				if herr != nil && herr != io.EOF {
					return herr
				}
				if strings.TrimSpace(nextLine) == "" {
					break
				}
				headerBlock += nextLine
				if herr == io.EOF {
					break
				}
			}
		}
		match := regexpContentLength.FindStringSubmatch(headerBlock)
		if match == nil {
			if err == io.EOF {
				return nil
			}
			continue
		}
		var length int
		if _, serr := fmt.Sscanf(match[1], "%d", &length); serr != nil || length <= 0 {
			if err == io.EOF {
				return nil
			}
			continue
		}
		body := make([]byte, length)
		if _, rerr := io.ReadFull(reader, body); rerr != nil {
			return nil
		}
		var message McpRpc
		if uerr := json.Unmarshal(body, &message); uerr != nil {
			continue
		}
		response := HandleMcpRpcWithOptions(message, clusterFn(), version, opts)
		if response == nil {
			continue
		}
		payload, merr := json.Marshal(response)
		if merr != nil {
			continue
		}
		if werr := writeMcpFrame(out, payload, true); werr != nil {
			return werr
		}
	}
}

// FrameMcpMessages is a test helper that builds a Content-Length framed MCP stream.
func FrameMcpMessages(messages ...any) []byte {
	var buf bytes.Buffer
	for _, message := range messages {
		payload, _ := json.Marshal(message)
		_ = writeMcpFrame(&buf, payload, true)
	}
	return buf.Bytes()
}
