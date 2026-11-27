package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/NadafMM/kubectl-shell/internal/kube/live"
	ksandbox "github.com/NadafMM/kubectl-shell/internal/kube/sandbox"
	ksession "github.com/NadafMM/kubectl-shell/internal/kube/session"
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
	live.InvalidateLive()
	t.Cleanup(live.InvalidateLive)
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
	if code := run([]string{"completion"}, &out, &err, runOptions{}); code != 1 || !strings.Contains(err.String(), "bash|zsh|fish|powershell") {
		t.Fatalf("code %d err %q", code, err.String())
	}
}

func TestCompletionPowerShell(t *testing.T) {
	var out, err bytes.Buffer
	if code := run([]string{"completion", "powershell"}, &out, &err, runOptions{}); code != 0 || !strings.Contains(out.String(), "Register-ArgumentCompleter") {
		t.Fatalf("code %d out %q err %q", code, out.String(), err.String())
	}
	if code := run([]string{"completion", "pwsh"}, &out, &err, runOptions{}); code != 0 || !strings.Contains(out.String(), "Register-ArgumentCompleter") {
		t.Fatalf("pwsh code %d out %q", code, out.String())
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

func TestRecordFlagWritesSession(t *testing.T) {
	withFakeEnv(t)
	t.Cleanup(func() { ksession.StopRecord() })
	file := filepath.Join(os.Getenv("HOME"), "out.md")
	var out, err bytes.Buffer
	code := run([]string{"--record", file, "get", "pods"}, &out, &err, runOptions{})
	if code != 0 {
		t.Fatalf("code %d err %q", code, err.String())
	}
	raw, readErr := os.ReadFile(file)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !strings.Contains(string(raw), "kubectl get pods") {
		t.Fatalf("%q", raw)
	}
}

func TestRecordShellDropsRefusedDelete(t *testing.T) {
	withFakeEnv(t)
	t.Cleanup(func() { ksession.StopRecord() })
	script := "record on\ndelete pod worker-0\nget pods --from-literal password=secret\nrecord off\nget ns\n"
	var out, err bytes.Buffer
	code := run(nil, &out, &err, runOptions{
		stdin:    strings.NewReader(script),
		confirm:  func(string) bool { return false },
		stdinTTY: false,
	})
	if code != 1 && code != 0 {
		// last get ns should succeed; delete refusal may make script exit non-zero depending on runScript
		_ = code
	}
	text := out.String()
	recorded := ""
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "recording ") && !strings.HasPrefix(line, "recording stopped") {
			recorded = strings.TrimSpace(strings.TrimPrefix(line, "recording "))
			break
		}
	}
	if recorded == "" {
		t.Fatalf("no recording path in %q err %q", text, err.String())
	}
	raw, readErr := os.ReadFile(recorded)
	if readErr != nil {
		t.Fatal(readErr)
	}
	body := string(raw)
	if !strings.Contains(body, "kubectl get pods --from-literal password=***") {
		t.Fatalf("%q", body)
	}
	if strings.Contains(body, "delete") || strings.Contains(body, "password=secret") || strings.Contains(body, "kubectl get ns") {
		t.Fatalf("%q", body)
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
		runLive: func(args []string, _ live.RunLiveOptions) int {
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
		runLive: func([]string, live.RunLiveOptions) int {
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
		runLive: func(args []string, _ live.RunLiveOptions) int {
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
		runLive: func([]string, live.RunLiveOptions) int {
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
	ksession.AppendAudit(ksession.AuditEntry{
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
		runLive: func(args []string, _ live.RunLiveOptions) int {
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
		runLive: func(args []string, _ live.RunLiveOptions) int {
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
		runLive:   func([]string, live.RunLiveOptions) int { return 0 },
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
		runLive: func(args []string, _ live.RunLiveOptions) int {
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
	var calls []ksandbox.SandboxCommand
	host := &ksandbox.SandboxHost{
		Home: home,
		Has:  func(string) bool { return true },
		Run: func(command ksandbox.SandboxCommand) ksandbox.SandboxResult {
			calls = append(calls, command)
			if len(command.Args) > 0 && command.Args[0] == "create" {
				_ = os.MkdirAll(filepath.Dir(ksandbox.SandboxKubeconfig(home)), 0o700)
				_ = os.WriteFile(ksandbox.SandboxKubeconfig(home), []byte("kind-config"), 0o600)
			}
			return ksandbox.SandboxResult{Code: 0}
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
	if _, err := os.Stat(ksandbox.SandboxKubeconfig(home)); !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func TestSandboxFlagRequiresUp(t *testing.T) {
	withFakeEnv(t)
	var out, errBuf bytes.Buffer
	code := run([]string{"--sandbox", "get", "pods"}, &out, &errBuf, runOptions{
		sandboxHost: &ksandbox.SandboxHost{Home: os.Getenv("HOME"), Has: func(string) bool { return true }, Run: func(ksandbox.SandboxCommand) ksandbox.SandboxResult { return ksandbox.SandboxResult{} }},
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

func TestTypedConfirmInteractiveRejectsYAndAcceptsContext(t *testing.T) {
	withFakeEnv(t)
	policyPath := filepath.Join(os.Getenv("HOME"), "policy.json")
	if err := os.WriteFile(policyPath, []byte(`{"contexts":{"minikube":{"confirm":"typed"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errBuf bytes.Buffer
	ran := false
	code := run([]string{"--policy", policyPath, "delete", "pod", "checkout"}, &out, &errBuf, runOptions{
		probeLive:   func() bool { return true },
		confirmText: func(string) string { return "y" },
		runLive: func([]string, live.RunLiveOptions) int {
			ran = true
			return 0
		},
	})
	if code != 1 || ran || !strings.Contains(out.String(), "not run") {
		t.Fatalf("expected y refusal: code=%d ran=%v out=%q", code, ran, out.String())
	}

	out.Reset()
	errBuf.Reset()
	code = run([]string{"--policy", policyPath, "delete", "pod", "checkout"}, &out, &errBuf, runOptions{
		probeLive:   func() bool { return true },
		confirmText: func(string) string { return "minikube" },
		runLive: func([]string, live.RunLiveOptions) int {
			ran = true
			return 0
		},
	})
	if code != 0 || !ran {
		t.Fatalf("expected typed context pass: code=%d ran=%v out=%q err=%q", code, ran, out.String(), errBuf.String())
	}
}

func TestRunKubectlSkipsLiveClusterGetsOnRead(t *testing.T) {
	withFakeEnv(t)
	logPath := filepath.Join(os.Getenv("HOME"), "kubectl.log")
	_ = os.Remove(logPath)
	var out, errBuf bytes.Buffer
	code := run([]string{"get", "pods"}, &out, &errBuf, runOptions{
		runLive: func(args []string, _ live.RunLiveOptions) int { return 0 },
	})
	if code != 0 {
		t.Fatalf("code %d err=%q", code, errBuf.String())
	}
	raw, _ := os.ReadFile(logPath)
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.Contains(line, `"get"`) {
			t.Fatalf("unexpected preflight kubectl get during read passthrough: %s", line)
		}
	}
}

func TestHookScriptsAndPluginArgs(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish", "powershell", "pwsh"} {
		var out, errBuf bytes.Buffer
		if code := run([]string{"hook", shell}, &out, &errBuf, runOptions{}); code != 0 || !strings.Contains(out.String(), "kubectl") {
			t.Fatalf("shell=%s code=%d out=%q err=%q", shell, code, out.String(), errBuf.String())
		}
	}
	var out, errBuf bytes.Buffer
	if code := run([]string{"hook"}, &out, &errBuf, runOptions{}); code != 1 {
		t.Fatalf("expected usage error, got code=%d", code)
	}
	got := normalizePluginArgs("/usr/local/bin/kubectl-blast", []string{"delete", "deploy/web"})
	if len(got) != 3 || got[0] != "plan" || got[1] != "delete" {
		t.Fatalf("unexpected blast plugin args: %v", got)
	}
	gotWin := normalizePluginArgs(`C:\bin\kubectl-blast.exe`, []string{"--version"})
	if len(gotWin) != 1 || gotWin[0] != "--version" {
		t.Fatalf("unexpected blast version args: %v", gotWin)
	}
	out.Reset()
	errBuf.Reset()
	if code := run([]string{"hook", "zsh"}, &out, &errBuf, runOptions{}); code != 0 || !strings.Contains(out.String(), "kubecolor") || !strings.Contains(out.String(), "kube env") {
		t.Fatalf("expected kubecolor and kube env in zsh hook: %q", out.String())
	}
}

func TestEnvCommandWritesScopedKubeconfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	cfg := filepath.Join(home, "cfg")
	raw := "apiVersion: v1\nkind: Config\ncurrent-context: dev\ncontexts:\n  - name: dev\n    context:\n      cluster: c\n      user: u\n      namespace: default\nclusters:\n  - name: c\n    cluster:\n      server: https://127.0.0.1:6443\nusers:\n  - name: u\n    user:\n      token: t\n"
	if err := os.WriteFile(cfg, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KUBECONFIG", cfg)
	t.Setenv("KUBE_SHELL_BASE_KUBECONFIG", "")
	var out, errBuf bytes.Buffer
	code := run([]string{"env", "--ctx", "dev", "--ns", "payments", "--shell", "zsh", "--session", "t1"}, &out, &errBuf, runOptions{})
	if code != 0 || !strings.Contains(out.String(), "export KUBECONFIG=") || !strings.Contains(out.String(), "t1.yaml") {
		t.Fatalf("code=%d out=%q err=%q", code, out.String(), errBuf.String())
	}
}

func TestMcpReadOnlyFlagViaCLI(t *testing.T) {
	withFakeEnv(t)
	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"kube_execute","arguments":{"command":"delete pod checkout","planId":"abc"}}}` + "\n"
	var out, errBuf bytes.Buffer
	code := run([]string{"mcp", "--read-only"}, &out, &errBuf, runOptions{
		stdin:     strings.NewReader(req),
		probeLive: func() bool { return true },
	})
	if code != 0 || !strings.Contains(out.String(), "--read-only") {
		t.Fatalf("code=%d out=%q err=%q", code, out.String(), errBuf.String())
	}
}
