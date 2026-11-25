package mcp

import (
	"bytes"
	"encoding/json"
	"github.com/NadafMM/kubectl-shell/internal/kube"
	"github.com/NadafMM/kubectl-shell/internal/kube/live"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestServeMcpInitialize(t *testing.T) {
	frame := FrameMcpMessages(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
	})
	var out bytes.Buffer
	cluster := kube.Cluster{Current: "minikube", Contexts: []kube.Context{{Name: "minikube", Namespace: "default"}}}
	if err := ServeMcp(cluster, "0.5.1", bytes.NewReader(frame), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"protocolVersion":"2024-11-05"`) {
		t.Fatal(out.String())
	}
}

func TestServeMcpStreamsInitializeThenToolsList(t *testing.T) {
	pr, pw := io.Pipe()
	var out bytes.Buffer
	cluster := kube.Cluster{Current: "minikube", Contexts: []kube.Context{{Name: "minikube", Namespace: "default"}}}
	done := make(chan error, 1)
	go func() {
		done <- ServeMcp(cluster, "0.6.0", pr, &out)
	}()

	write := func(message map[string]any) {
		payload, _ := json.Marshal(message)
		if _, err := pw.Write([]byte("Content-Length: " + strconv.Itoa(len(payload)) + "\r\n\r\n")); err != nil {
			t.Fatal(err)
		}
		if _, err := pw.Write(payload); err != nil {
			t.Fatal(err)
		}
	}
	write(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize"})
	write(map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"})
	write(map[string]any{"jsonrpc": "2.0", "id": 2, "method": "tools/list"})
	_ = pw.Close()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, `"protocolVersion":"2024-11-05"`) {
		t.Fatal(text)
	}
	if !strings.Contains(text, `"inputSchema"`) || !strings.Contains(text, `"kube_execute"`) {
		t.Fatal(text)
	}
	// Two framed responses (initialize + tools/list); notification has no reply.
	if strings.Count(text, "Content-Length:") != 2 {
		t.Fatal(text)
	}
}

func TestHandleMcpToolsListHasInputSchema(t *testing.T) {
	reply := HandleMcpRpc(McpRpc{JSONRPC: "2.0", ID: 1, Method: "tools/list"}, kube.Cluster{}, "0.6.0")
	raw, _ := json.Marshal(reply)
	if !strings.Contains(string(raw), `"inputSchema"`) || !strings.Contains(string(raw), `"planId"`) {
		t.Fatal(string(raw))
	}
}

func TestHandleMcpExecuteRequiresPlanID(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("KUBE_SHELL_KUBECTL", "node")
	t.Setenv("KUBE_SHELL_NODE_SCRIPT", filepath.Join("..", "..", "..", "scripts", "fake-kubectl.mjs"))
	t.Setenv("KUBE_FAKE_FIXTURE", "up")
	_ = os.MkdirAll(filepath.Join(home, ".kube-shell"), 0o700)
	_ = os.WriteFile(filepath.Join(home, ".kube-shell", "policy.json"), []byte(`{"agents":{"allow":["get","describe","logs","plan","delete"],"write_requires":"approved-plan"}}`), 0o600)
	live.InvalidateLive()
	t.Cleanup(live.InvalidateLive)

	cluster := kube.Cluster{Current: "minikube", Contexts: []kube.Context{{Name: "minikube", Namespace: "default"}}}
	reply := HandleMcpRpc(McpRpc{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params: &struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}{Name: "kube_execute", Arguments: map[string]any{"command": "delete pod worker-0"}},
	}, cluster, "0.5.1")
	raw, _ := json.Marshal(reply)
	if !strings.Contains(string(raw), "plan id") || !strings.Contains(string(raw), `"isError":true`) {
		t.Fatal(string(raw))
	}
}

func TestHandleMcpPlanAndExecute(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("KUBE_SHELL_KUBECTL", "node")
	t.Setenv("KUBE_SHELL_NODE_SCRIPT", filepath.Join("..", "..", "..", "scripts", "fake-kubectl.mjs"))
	t.Setenv("KUBE_FAKE_FIXTURE", "up")
	logPath := filepath.Join(home, "kubectl.log")
	t.Setenv("KUBE_FAKE_LOG", logPath)
	_ = os.MkdirAll(filepath.Join(home, ".kube-shell"), 0o700)
	_ = os.WriteFile(filepath.Join(home, ".kube-shell", "policy.json"), []byte(`{"agents":{"allow":["get","describe","logs","plan","delete"],"write_requires":"approved-plan"}}`), 0o600)
	live.InvalidateLive()
	t.Cleanup(live.InvalidateLive)

	cluster := kube.Cluster{Current: "minikube", Contexts: []kube.Context{{Name: "minikube", Namespace: "default"}}, Objects: []kube.KObject{
		{Kind: "pod", Name: "worker-0", NS: "default", Status: "Running"},
	}}
	planned := HandleMcpRpc(McpRpc{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "tools/call",
		Params: &struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}{Name: "kube_plan", Arguments: map[string]any{"command": "delete pod worker-0"}},
	}, cluster, "0.5.1")
	raw, _ := json.Marshal(planned)
	var envelope struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatal(err, string(raw))
	}
	var body struct {
		PlanID string `json:"planId"`
	}
	if err := json.Unmarshal([]byte(envelope.Result.Content[0].Text), &body); err != nil || body.PlanID == "" {
		t.Fatal(envelope.Result.Content[0].Text)
	}
	executed := HandleMcpRpc(McpRpc{
		JSONRPC: "2.0",
		ID:      5,
		Method:  "tools/call",
		Params: &struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}{Name: "kube_execute", Arguments: map[string]any{"command": "delete pod worker-0", "planId": body.PlanID}},
	}, cluster, "0.5.1")
	raw, _ = json.Marshal(executed)
	if strings.Contains(string(raw), `"isError":true`) {
		t.Fatal(string(raw))
	}
	audit, err := os.ReadFile(filepath.Join(home, ".kube-shell", "audit.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(audit), `"actor":"mcp"`) {
		t.Fatal(string(audit))
	}
}
