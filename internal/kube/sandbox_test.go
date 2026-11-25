package kube

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sandboxFixture(t *testing.T) (home, userConfig string, calls *[]SandboxCommand, run func(SandboxCommand) SandboxResult) {
	t.Helper()
	home = t.TempDir()
	userConfig = filepath.Join(home, ".kube", "config")
	if err := os.MkdirAll(filepath.Dir(userConfig), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(userConfig, []byte("original-kubeconfig"), 0o600); err != nil {
		t.Fatal(err)
	}
	var recorded []SandboxCommand
	run = func(command SandboxCommand) SandboxResult {
		recorded = append(recorded, command)
		if len(command.Args) > 0 && command.Args[0] == "create" {
			_ = os.MkdirAll(filepath.Dir(SandboxKubeconfig(home)), 0o700)
			_ = os.WriteFile(SandboxKubeconfig(home), []byte("kind-config"), 0o600)
		}
		return SandboxResult{Code: 0}
	}
	return home, userConfig, &recorded, run
}

func TestSandboxUpCreatesKindAndSeeds(t *testing.T) {
	home, userConfig, calls, run := sandboxFixture(t)
	t.Setenv("KUBECONFIG", userConfig)
	ok, message := SandboxUp(SandboxHost{Home: home, Run: run, Has: func(string) bool { return true }})
	if !ok || !strings.Contains(message, "kube-shell-sandbox is up") || !strings.Contains(message, "sandbox") {
		t.Fatalf("%v %q", ok, message)
	}
	raw, _ := os.ReadFile(userConfig)
	if string(raw) != "original-kubeconfig" {
		t.Fatal(string(raw))
	}
	if len(*calls) < 2 {
		t.Fatal(*calls)
	}
	create := (*calls)[0]
	if !strings.Contains(create.Cmd, "kind") {
		t.Fatal(create.Cmd)
	}
	wantArgs := []string{"create", "cluster", "--name", SandboxCluster, "--kubeconfig", SandboxKubeconfig(home)}
	if strings.Join(create.Args, " ") != strings.Join(wantArgs, " ") {
		t.Fatal(create.Args)
	}
	env := strings.Join(create.Env, "\n")
	if !strings.Contains(env, "KUBECONFIG="+SandboxKubeconfig(home)) {
		t.Fatal(create.Env)
	}
	apply := (*calls)[1]
	if strings.Join(apply.Args[len(apply.Args)-3:], " ") != "apply -f -" {
		t.Fatal(apply.Args)
	}
	if apply.Input != SandboxSeed {
		t.Fatal("seed mismatch")
	}
	if !strings.Contains(SandboxSeed, "kind: Deployment") || !strings.Contains(SandboxSeed, "name: broken") {
		t.Fatal(SandboxSeed)
	}
	got, _ := os.ReadFile(SandboxKubeconfig(home))
	if string(got) != "kind-config" {
		t.Fatal(string(got))
	}
}

func TestSandboxUpNeedsKindAndDocker(t *testing.T) {
	home, userConfig, calls, run := sandboxFixture(t)
	ok, message := SandboxUp(SandboxHost{Home: home, Run: run, Has: func(name string) bool { return name == "kind" }})
	if ok || message != "kube sandbox needs kind and Docker on PATH" || len(*calls) != 0 {
		t.Fatalf("%v %q %v", ok, message, *calls)
	}
	if _, err := os.Stat(SandboxKubeconfig(home)); !os.IsNotExist(err) {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(userConfig)
	if string(raw) != "original-kubeconfig" {
		t.Fatal(string(raw))
	}
}

func TestSandboxUpSkipsApplyWhenKindFails(t *testing.T) {
	home, userConfig, calls, _ := sandboxFixture(t)
	ok, message := SandboxUp(SandboxHost{
		Home: home,
		Has:  func(string) bool { return true },
		Run: func(command SandboxCommand) SandboxResult {
			*calls = append(*calls, command)
			return SandboxResult{Code: 1, Stderr: "kind exploded"}
		},
	})
	if ok || !strings.Contains(message, "kind exploded") || len(*calls) != 1 {
		t.Fatalf("%v %q %v", ok, message, *calls)
	}
	raw, _ := os.ReadFile(userConfig)
	if string(raw) != "original-kubeconfig" {
		t.Fatal(string(raw))
	}
}

func TestSandboxDownDeletesCluster(t *testing.T) {
	home, userConfig, calls, run := sandboxFixture(t)
	config := SandboxKubeconfig(home)
	_ = os.MkdirAll(filepath.Dir(config), 0o700)
	_ = os.WriteFile(config, []byte("kind-config"), 0o600)
	audit := filepath.Join(home, ".kube-shell", "audit.jsonl")
	_ = os.WriteFile(audit, []byte("keep\n"), 0o600)
	ok, message := SandboxDown(SandboxHost{Home: home, Run: run, Has: func(string) bool { return true }})
	if !ok || !strings.Contains(message, "kube-shell-sandbox is down") {
		t.Fatalf("%v %q", ok, message)
	}
	if len(*calls) == 0 || strings.Join((*calls)[0].Args, " ") != "delete cluster --name kube-shell-sandbox" {
		t.Fatal(*calls)
	}
	if _, err := os.Stat(config); !os.IsNotExist(err) {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(audit)
	if string(raw) != "keep\n" {
		t.Fatal(string(raw))
	}
	user, _ := os.ReadFile(userConfig)
	if string(user) != "original-kubeconfig" {
		t.Fatal(string(user))
	}
}

func TestSandboxDownKeepsConfigOnHardFailure(t *testing.T) {
	home, userConfig, _, _ := sandboxFixture(t)
	config := SandboxKubeconfig(home)
	_ = os.MkdirAll(filepath.Dir(config), 0o700)
	_ = os.WriteFile(config, []byte("kind-config"), 0o600)
	ok, message := SandboxDown(SandboxHost{
		Home: home,
		Has:  func(string) bool { return true },
		Run:  func(SandboxCommand) SandboxResult { return SandboxResult{Code: 1, Stderr: "docker daemon is down"} },
	})
	if ok || !strings.Contains(message, "docker daemon is down") {
		t.Fatalf("%v %q", ok, message)
	}
	raw, _ := os.ReadFile(config)
	if string(raw) != "kind-config" {
		t.Fatal(string(raw))
	}
	user, _ := os.ReadFile(userConfig)
	if string(user) != "original-kubeconfig" {
		t.Fatal(string(user))
	}
}

func TestSandboxDownRemovesConfigWhenClusterGone(t *testing.T) {
	home, _, _, _ := sandboxFixture(t)
	config := SandboxKubeconfig(home)
	_ = os.MkdirAll(filepath.Dir(config), 0o700)
	_ = os.WriteFile(config, []byte("kind-config"), 0o600)
	ok, _ := SandboxDown(SandboxHost{
		Home: home,
		Has:  func(string) bool { return true },
		Run: func(SandboxCommand) SandboxResult {
			return SandboxResult{Code: 1, Stderr: "ERROR: failed to find cluster"}
		},
	})
	if !ok {
		t.Fatal("expected ok")
	}
	if _, err := os.Stat(config); !os.IsNotExist(err) {
		t.Fatal(err)
	}
}
