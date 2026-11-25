package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/NadafMM/kubectl-shell/internal/kube"
)

func withFakeEnv(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("KUBE_SHELL_KUBECTL", "node")
	t.Setenv("KUBE_SHELL_NODE_SCRIPT", filepath.Join("..", "..", "scripts", "fake-kubectl.mjs"))
	t.Setenv("KUBE_FAKE_FIXTURE", "up")
	t.Setenv("KUBE_FAKE_LOG", filepath.Join(home, "kubectl.log"))
	t.Setenv("KUBE_FAKE_FAIL", "")
	kube.InvalidateLive()
	t.Cleanup(kube.InvalidateLive)
}

func TestVersion(t *testing.T) {
	var out, err bytes.Buffer
	if code := run([]string{"--version"}, &out, &err, runOptions{}); code != 0 || strings.TrimSpace(out.String()) != version {
		t.Fatalf("code %d out %q err %q", code, out.String(), err.String())
	}
}

func TestCompletionBash(t *testing.T) {
	var out, err bytes.Buffer
	if code := run([]string{"completion", "bash"}, &out, &err, runOptions{}); code != 0 || !strings.Contains(out.String(), "mapfile -t COMPREPLY") {
		t.Fatalf("code %d out %q", code, out.String())
	}
}

func TestCompletionUsage(t *testing.T) {
	var out, err bytes.Buffer
	if code := run([]string{"completion"}, &out, &err, runOptions{}); code != 1 || !strings.Contains(err.String(), "bash|zsh|fish") {
		t.Fatalf("code %d err %q", code, err.String())
	}
}

func TestCompletePods(t *testing.T) {
	line := "kube get po"
	var out, err bytes.Buffer
	code := run([]string{"__complete", "--cursor", "10", "--line", line}, &out, &err, runOptions{})
	if code != 0 || !strings.Contains(out.String(), "pods") {
		t.Fatalf("code %d out %q err %q", code, out.String(), err.String())
	}
}

func TestCompleteLiveDeployment(t *testing.T) {
	withFakeEnv(t)
	line := "kube get deploy "
	var out, err bytes.Buffer
	code := run([]string{"__complete", "--cursor", "16", "--line", line}, &out, &err, runOptions{})
	if code != 0 || !strings.Contains(out.String(), "web") {
		t.Fatalf("code %d out %q err %q", code, out.String(), err.String())
	}
}

func TestPlanDelete(t *testing.T) {
	withFakeEnv(t)
	var out, err bytes.Buffer
	code := run([]string{"plan", "delete", "pod", "checkout"}, &out, &err, runOptions{})
	if code != 0 || !strings.Contains(out.String(), "risk high") || !strings.Contains(out.String(), "pod/checkout") {
		t.Fatalf("code %d out %q err %q", code, out.String(), err.String())
	}
	out.Reset()
	err.Reset()
	code = run([]string{"plan", "--json", "get", "pods"}, &out, &err, runOptions{})
	if code != 0 || !strings.Contains(out.String(), `"risk":"read"`) || !strings.Contains(out.String(), `"planId"`) {
		t.Fatalf("code %d out %q err %q", code, out.String(), err.String())
	}
}

func TestPlanBlastSkippedForGet(t *testing.T) {
	withFakeEnv(t)
	var out, err bytes.Buffer
	code := run([]string{"plan", "--json", "get", "pods"}, &out, &err, runOptions{})
	if code != 0 || strings.Contains(out.String(), `"blast"`) {
		t.Fatalf("code %d out %q err %q", code, out.String(), err.String())
	}
}

func TestAskContext(t *testing.T) {
	withFakeEnv(t)
	var out, err bytes.Buffer
	code := run([]string{"ask", "which", "cluster"}, &out, &err, runOptions{})
	if code != 0 || !strings.Contains(out.String(), "minikube") {
		t.Fatalf("code %d out %q err %q", code, out.String(), err.String())
	}
}

func TestAskJSONReportsOOM(t *testing.T) {
	withFakeEnv(t)
	t.Setenv("KUBE_FAKE_DIAG", "oom")
	var out, err bytes.Buffer
	code := run([]string{"ask", "--json", "why is checkout crashing"}, &out, &err, runOptions{})
	if code != 0 {
		t.Fatalf("code %d err %q", code, err.String())
	}
	text := out.String()
	if !strings.Contains(text, "OOMKilled") || !strings.Contains(text, "memory limit 64Mi") {
		t.Fatalf("out %q", text)
	}
	if strings.Contains(text, "password=secret") {
		t.Fatalf("leaked secret in %q", text)
	}
}

func TestAskPortForwardSkipsLogs(t *testing.T) {
	withFakeEnv(t)
	t.Setenv("KUBE_FAKE_DIAG", "oom")
	logPath := filepath.Join(os.Getenv("HOME"), "kubectl.log")
	_ = os.Remove(logPath)
	var out, err bytes.Buffer
	code := run([]string{"ask", "how do I port-forward"}, &out, &err, runOptions{})
	if code != 0 || !strings.Contains(out.String(), "port-forward") {
		t.Fatalf("code %d out %q err %q", code, out.String(), err.String())
	}
	raw, _ := os.ReadFile(logPath)
	if strings.Contains(string(raw), `"logs"`) {
		t.Fatalf("unexpected logs call: %s", raw)
	}
}

func TestViewPrintsPlainTable(t *testing.T) {
	withFakeEnv(t)
	var out, err bytes.Buffer
	code := run([]string{"view", "pods"}, &out, &err, runOptions{
		forcePlain: true,
		probeLive:  func() bool { return true },
	})
	if code != 0 {
		t.Fatalf("code %d err %q", code, err.String())
	}
	if !strings.Contains(out.String(), "NAME  STATUS") && !strings.Contains(out.String(), "STATUS") {
		// fake cluster may be empty, but the header must print
		t.Fatalf("out %q err %q", out.String(), err.String())
	}
}

func TestGetPodsRuns(t *testing.T) {
	withFakeEnv(t)
	var out, err bytes.Buffer
	var seen []string
	code := run([]string{"get", "pods"}, &out, &err, runOptions{
		probeLive: func() bool { return true },
		runLive: func(args []string, _ kube.RunLiveOptions) int {
			seen = append(seen, args...)
			return 0
		},
	})
	if code != 0 || len(seen) < 2 || seen[0] != "get" || seen[1] != "pods" {
		t.Fatalf("code %d seen %v err %q", code, seen, err.String())
	}
}

func TestDeleteRefusedWithoutYes(t *testing.T) {
	withFakeEnv(t)
	var out, err bytes.Buffer
	ran := false
	code := run([]string{"delete", "pod", "checkout"}, &out, &err, runOptions{
		probeLive: func() bool { return true },
		confirm:   func(string) bool { return false },
		runLive: func([]string, kube.RunLiveOptions) int {
			ran = true
			return 0
		},
	})
	if code != 1 || ran || !strings.Contains(out.String(), "not run") {
		t.Fatalf("code %d out %q ran=%v err %q", code, out.String(), ran, err.String())
	}
	if !strings.Contains(err.String(), "risk high") {
		t.Fatal(err.String())
	}
}

func TestDeleteWithYesRuns(t *testing.T) {
	withFakeEnv(t)
	var out, err bytes.Buffer
	ran := false
	code := run([]string{"--yes", "delete", "pod", "checkout"}, &out, &err, runOptions{
		probeLive: func() bool { return true },
		stdinTTY:  true,
		confirm:   func(string) bool { t.Fatal("confirm"); return false },
		runLive: func(args []string, _ kube.RunLiveOptions) int {
			ran = true
			if args[0] != "delete" {
				t.Fatal(args)
			}
			return 0
		},
	})
	if code != 0 || !ran {
		t.Fatalf("code %d ran=%v err %q", code, ran, err.String())
	}
}

func TestPolicyDeniesExec(t *testing.T) {
	withFakeEnv(t)
	policyPath := filepath.Join(os.Getenv("HOME"), "policy.json")
	if err := os.WriteFile(policyPath, []byte(`{"contexts":{"*":{"deny":["exec"]}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errBuf bytes.Buffer
	ran := false
	code := run([]string{"--policy", policyPath, "exec", "pod/api", "--", "date"}, &out, &errBuf, runOptions{
		probeLive: func() bool { return true },
		runLive: func([]string, kube.RunLiveOptions) int {
			ran = true
			return 0
		},
	})
	if code != 1 || ran || !strings.Contains(out.String(), "denied by policy") {
		t.Fatalf("code %d out %q ran=%v", code, out.String(), ran)
	}
}

func TestAuditTail(t *testing.T) {
	withFakeEnv(t)
	kube.AppendAudit(kube.AuditEntry{
		ID: "1", Timestamp: "2026-11-24T00:00:00Z", Actor: "human-cli",
		Context: "minikube", Namespace: "default", Argv: []string{"get", "pods"},
		Risk: "read", Decision: "executed", Approver: "flag-yes", ExitCode: intPtr(0), DurationMs: 1,
	}, 0)
	var out, err bytes.Buffer
	code := run([]string{"audit", "tail", "-n", "1"}, &out, &err, runOptions{})
	if code != 0 || !strings.Contains(out.String(), "get") {
		t.Fatalf("code %d out %q err %q", code, out.String(), err.String())
	}
}

func intPtr(n int) *int { return &n }

func TestUndoListAndRestore(t *testing.T) {
	withFakeEnv(t)
	t.Setenv("KUBE_FAKE_UNDO", "1")
	var out, err bytes.Buffer
	code := run([]string{"--yes", "set", "image", "deployment/checkout", "nginx=nginx:1.2"}, &out, &err, runOptions{
		probeLive: func() bool { return true },
		stdinTTY:  true,
		runLive: func(args []string, _ kube.RunLiveOptions) int {
			return 0
		},
	})
	if code != 0 {
		t.Fatalf("set image code %d err %q", code, err.String())
	}
	out.Reset()
	err.Reset()
	code = run([]string{"undo", "list"}, &out, &err, runOptions{})
	if code != 0 || !strings.Contains(out.String(), "deployment/checkout") {
		t.Fatalf("list code %d out %q", code, out.String())
	}
	out.Reset()
	err.Reset()
	var restore []string
	code = run([]string{"--yes", "undo"}, &out, &err, runOptions{
		stdinTTY: true,
		runLive: func(args []string, _ kube.RunLiveOptions) int {
			restore = append([]string{}, args...)
			return 0
		},
	})
	if code != 0 || !strings.Contains(out.String(), "restored deployment/checkout") {
		t.Fatalf("undo code %d out %q", code, out.String())
	}
	if len(restore) == 0 || restore[0] != "rollout" {
		t.Fatal(restore)
	}
}

func TestUndoRefusesSecret(t *testing.T) {
	withFakeEnv(t)
	t.Setenv("KUBE_FAKE_UNDO", "1")
	var out, err bytes.Buffer
	_ = run([]string{"--yes", "label", "secret", "token", "env=prod"}, &out, &err, runOptions{
		probeLive: func() bool { return true },
		stdinTTY:  true,
		runLive:   func([]string, kube.RunLiveOptions) int { return 0 },
	})
	out.Reset()
	err.Reset()
	code := run([]string{"--yes", "undo"}, &out, &err, runOptions{stdinTTY: true})
	if code != 1 || !strings.Contains(out.String(), "Secrets or ConfigMaps") {
		t.Fatalf("code %d out %q", code, out.String())
	}
}

func TestYesAloneRefusesShell(t *testing.T) {
	var out, err bytes.Buffer
	agentFalse := false
	code := run([]string{"--yes"}, &out, &err, runOptions{stdinTTY: true, agent: &agentFalse})
	if code != 1 || !strings.Contains(err.String(), "`--yes` applies to one command") {
		t.Fatalf("code %d err %q", code, err.String())
	}
}

func TestShellScriptAppliesNsScope(t *testing.T) {
	withFakeEnv(t)
	var out, errBuf bytes.Buffer
	var seen [][]string
	code := run([]string{}, &out, &errBuf, runOptions{
		stdin:     strings.NewReader("ns payments\nget pods\n"),
		probeLive: func() bool { return true },
		runLive: func(args []string, _ kube.RunLiveOptions) int {
			seen = append(seen, append([]string{}, args...))
			return 0
		},
	})
	if code != 0 {
		t.Fatalf("code %d err %q out %q", code, errBuf.String(), out.String())
	}
	if !strings.Contains(out.String(), "namespace payments") {
		t.Fatalf("out %q", out.String())
	}
	found := false
	for _, args := range seen {
		joined := strings.Join(args, " ")
		if strings.HasPrefix(joined, "get pods") && strings.Contains(joined, "-n payments") {
			found = true
		}
	}
	if !found {
		t.Fatalf("seen %v", seen)
	}
}

func TestShellExit(t *testing.T) {
	withFakeEnv(t)
	var out, errBuf bytes.Buffer
	code := run([]string{}, &out, &errBuf, runOptions{
		stdin:     strings.NewReader("exit\n"),
		probeLive: func() bool { return true },
	})
	if code != 0 {
		t.Fatalf("code %d err %q", code, errBuf.String())
	}
}

func TestOneShotNs(t *testing.T) {
	withFakeEnv(t)
	var out, errBuf bytes.Buffer
	code := run([]string{"ns", "payments"}, &out, &errBuf, runOptions{
		probeLive: func() bool { return true },
	})
	if code != 0 || !strings.Contains(out.String(), "namespace payments") {
		t.Fatalf("code %d out %q err %q", code, out.String(), errBuf.String())
	}
}

func TestSandboxUpDown(t *testing.T) {
	withFakeEnv(t)
	home := os.Getenv("HOME")
	userConfig := filepath.Join(home, ".kube", "config")
	_ = os.MkdirAll(filepath.Dir(userConfig), 0o755)
	_ = os.WriteFile(userConfig, []byte("original-kubeconfig"), 0o600)
	var calls []kube.SandboxCommand
	host := &kube.SandboxHost{
		Home: home,
		Has:  func(string) bool { return true },
		Run: func(command kube.SandboxCommand) kube.SandboxResult {
			calls = append(calls, command)
			if len(command.Args) > 0 && command.Args[0] == "create" {
				_ = os.MkdirAll(filepath.Dir(kube.SandboxKubeconfig(home)), 0o700)
				_ = os.WriteFile(kube.SandboxKubeconfig(home), []byte("kind-config"), 0o600)
			}
			return kube.SandboxResult{Code: 0}
		},
	}
	var out, errBuf bytes.Buffer
	code := run([]string{"sandbox", "up"}, &out, &errBuf, runOptions{sandboxHost: host})
	if code != 0 || !strings.Contains(out.String(), "kube-shell-sandbox is up") {
		t.Fatalf("code %d out %q err %q", code, out.String(), errBuf.String())
	}
	raw, _ := os.ReadFile(userConfig)
	if string(raw) != "original-kubeconfig" {
		t.Fatal(string(raw))
	}
	if len(calls) < 2 || calls[0].Args[0] != "create" {
		t.Fatal(calls)
	}
	out.Reset()
	errBuf.Reset()
	code = run([]string{"sandbox", "down"}, &out, &errBuf, runOptions{sandboxHost: host})
	if code != 0 || !strings.Contains(out.String(), "is down") {
		t.Fatalf("code %d out %q err %q", code, out.String(), errBuf.String())
	}
	if _, err := os.Stat(kube.SandboxKubeconfig(home)); !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func TestSandboxFlagRequiresUp(t *testing.T) {
	withFakeEnv(t)
	var out, errBuf bytes.Buffer
	code := run([]string{"--sandbox", "get", "pods"}, &out, &errBuf, runOptions{
		sandboxHost: &kube.SandboxHost{Home: os.Getenv("HOME"), Has: func(string) bool { return true }, Run: func(kube.SandboxCommand) kube.SandboxResult { return kube.SandboxResult{} }},
	})
	if code != 1 || !strings.Contains(errBuf.String(), "kube sandbox is not up") {
		t.Fatalf("code %d err %q", code, errBuf.String())
	}
}

func TestSandboxUsage(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run([]string{"sandbox"}, &out, &errBuf, runOptions{})
	if code != 1 || !strings.Contains(errBuf.String(), "usage: kube sandbox") {
		t.Fatalf("code %d err %q", code, errBuf.String())
	}
}

func TestMcpInitializeViaCLI(t *testing.T) {
	withFakeEnv(t)
	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`)
	frame := append([]byte("Content-Length: "+strconv.Itoa(len(body))+"\r\n\r\n"), body...)
	var out, errBuf bytes.Buffer
	code := run([]string{"mcp"}, &out, &errBuf, runOptions{
		stdin:     bytes.NewReader(frame),
		probeLive: func() bool { return true },
	})
	if code != 0 || !strings.Contains(out.String(), "2024-11-05") {
		t.Fatalf("code %d out %q err %q", code, out.String(), errBuf.String())
	}
}
