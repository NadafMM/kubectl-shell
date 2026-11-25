package kube

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestServeMcpInitialize(t *testing.T) {
	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
	})
	frame := append([]byte("Content-Length: "+strconv.Itoa(len(body))+"\r\n\r\n"), body...)
	var out bytes.Buffer
	cluster := Cluster{Current: "minikube", Contexts: []Context{{Name: "minikube", Namespace: "default"}}}
	if err := ServeMcp(cluster, "0.5.1", bytes.NewReader(frame), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"protocolVersion":"2024-11-05"`) {
		t.Fatal(out.String())
	}
}

func TestHandleMcpExecuteRequiresPlanID(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("KUBE_SHELL_KUBECTL", "node")
	t.Setenv("KUBE_SHELL_NODE_SCRIPT", filepath.Join("..", "..", "scripts", "fake-kubectl.mjs"))
	t.Setenv("KUBE_FAKE_FIXTURE", "up")
	_ = os.MkdirAll(filepath.Join(home, ".kube-shell"), 0o700)
	_ = os.WriteFile(filepath.Join(home, ".kube-shell", "policy.json"), []byte(`{"agents":{"allow":["get","describe","logs","plan"],"write_requires":"approved-plan"}}`), 0o600)
	InvalidateLive()
	t.Cleanup(InvalidateLive)

	cluster := Cluster{Current: "minikube", Contexts: []Context{{Name: "minikube", Namespace: "default"}}}
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
	t.Setenv("KUBE_SHELL_NODE_SCRIPT", filepath.Join("..", "..", "scripts", "fake-kubectl.mjs"))
	t.Setenv("KUBE_FAKE_FIXTURE", "up")
	logPath := filepath.Join(home, "kubectl.log")
	t.Setenv("KUBE_FAKE_LOG", logPath)
	_ = os.MkdirAll(filepath.Join(home, ".kube-shell"), 0o700)
	_ = os.WriteFile(filepath.Join(home, ".kube-shell", "policy.json"), []byte(`{"agents":{"allow":["get","describe","logs","plan"],"write_requires":"approved-plan"}}`), 0o600)
	InvalidateLive()
	t.Cleanup(InvalidateLive)

	cluster := Cluster{Current: "minikube", Contexts: []Context{{Name: "minikube", Namespace: "default"}}, Objects: []KObject{
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
