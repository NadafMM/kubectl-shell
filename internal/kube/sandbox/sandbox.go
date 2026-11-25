package sandbox

import (
	"bytes"
	"github.com/NadafMM/kubectl-shell/internal/kube/live"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

const SandboxCluster = "kube-shell-sandbox"

// SandboxSeed is applied after kind create so ask/view have something to show.
const SandboxSeed = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
  namespace: default
spec:
  replicas: 2
  selector:
    matchLabels:
      app: demo
  template:
    metadata:
      labels:
        app: demo
    spec:
      containers:
        - name: app
          image: nginx:1.27
          ports:
            - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: demo
  namespace: default
spec:
  selector:
    app: demo
  ports:
    - port: 80
      targetPort: 80
---
apiVersion: v1
kind: Pod
metadata:
  name: broken
  namespace: default
spec:
  containers:
    - name: app
      image: kube-shell/missing:none
`

// SandboxCommand is one kind/kubectl invocation for the sandbox helpers.
type SandboxCommand struct {
	Cmd   string
	Args  []string
	Env   []string
	Input string
}

// SandboxResult is the captured output of a sandbox host command.
type SandboxResult struct {
	Code   int
	Stdout string
	Stderr string
}

// SandboxHost abstracts PATH checks and process launches for tests.
type SandboxHost struct {
	Home string
	Run  func(SandboxCommand) SandboxResult
	Has  func(name string) bool
}

// SandboxKubeconfig is ~/.kube-shell/sandbox/kubeconfig under home.
func SandboxKubeconfig(home string) string {
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	return filepath.Join(home, ".kube-shell", "sandbox", "kubeconfig")
}

func kindCmd() string {
	if v := os.Getenv("KUBE_SHELL_KIND"); v != "" {
		return v
	}
	return "kind"
}

func sandboxKubectlCmd() string {
	return live.KubectlBin()
}

func childEnv(kubeconfig string) []string {
	env := os.Environ()
	out := make([]string, 0, len(env)+1)
	replaced := false
	for _, item := range env {
		if strings.HasPrefix(item, "KUBECONFIG=") {
			out = append(out, "KUBECONFIG="+kubeconfig)
			replaced = true
			continue
		}
		out = append(out, item)
	}
	if !replaced {
		out = append(out, "KUBECONFIG="+kubeconfig)
	}
	return out
}

func tighten(path string, mode os.FileMode) {
	_ = os.Chmod(path, mode)
}

// SandboxUp creates the kind cluster and seeds demo workloads.
func SandboxUp(host SandboxHost) (ok bool, message string) {
	if host.Has == nil || !host.Has("kind") || !host.Has("docker") {
		return false, "kube sandbox needs kind and Docker on PATH"
	}
	kubeconfig := SandboxKubeconfig(host.Home)
	dir := filepath.Dir(kubeconfig)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return false, err.Error()
	}
	tighten(dir, 0o700)
	tighten(filepath.Dir(dir), 0o700)
	created := host.Run(SandboxCommand{
		Cmd:  kindCmd(),
		Args: []string{"create", "cluster", "--name", SandboxCluster, "--kubeconfig", kubeconfig},
		Env:  childEnv(kubeconfig),
	})
	if created.Code != 0 {
		msg := strings.TrimSpace(created.Stderr)
		if msg == "" {
			msg = strings.TrimSpace(created.Stdout)
		}
		if msg == "" {
			msg = "kind create failed"
		}
		return false, msg
	}
	tighten(kubeconfig, 0o600)
	applied := host.Run(SandboxCommand{
		Cmd:   sandboxKubectlCmd(),
		Args:  []string{"apply", "-f", "-"},
		Env:   childEnv(kubeconfig),
		Input: SandboxSeed,
	})
	if applied.Code != 0 {
		detail := strings.TrimSpace(applied.Stderr)
		if detail == "" {
			detail = strings.TrimSpace(applied.Stdout)
		}
		if detail == "" {
			detail = "kubectl apply failed"
		}
		return false, "sandbox cluster was created, but the demo workload was not applied\n" + detail
	}
	return true, "sandbox " + SandboxCluster + " is up\n" + kubeconfig
}

var clusterGoneRe = regexp.MustCompile(`(?i)not found|no kind cluster|failed to find|unknown cluster`)

func clusterGone(result SandboxResult) bool {
	return clusterGoneRe.MatchString(result.Stderr + "\n" + result.Stdout)
}

// SandboxDown deletes the kind cluster and removes only the sandbox kubeconfig.
func SandboxDown(host SandboxHost) (ok bool, message string) {
	kubeconfig := SandboxKubeconfig(host.Home)
	deleted := host.Run(SandboxCommand{
		Cmd:  kindCmd(),
		Args: []string{"delete", "cluster", "--name", SandboxCluster},
		Env:  childEnv(kubeconfig),
	})
	if deleted.Code != 0 && !clusterGone(deleted) {
		msg := strings.TrimSpace(deleted.Stderr)
		if msg == "" {
			msg = strings.TrimSpace(deleted.Stdout)
		}
		if msg == "" {
			msg = "kind delete failed"
		}
		return false, msg
	}
	_ = os.Remove(kubeconfig)
	return true, "sandbox " + SandboxCluster + " is down"
}

// CommandOnPath reports whether name resolves on PATH (or via KUBE_SHELL_* overrides).
func CommandOnPath(name string) bool {
	if name == "kind" && os.Getenv("KUBE_SHELL_KIND") != "" {
		return true
	}
	if name == "docker" && os.Getenv("KUBE_SHELL_DOCKER") != "" {
		return true
	}
	_, err := exec.LookPath(name)
	return err == nil
}

func launchSandbox(command SandboxCommand) SandboxResult {
	cmdName := command.Cmd
	args := append([]string{}, command.Args...)
	scripted := strings.HasSuffix(cmdName, ".mjs") || strings.HasSuffix(cmdName, ".cjs")
	if scripted {
		args = append([]string{cmdName}, args...)
		if node := os.Getenv("KUBE_SHELL_NODE"); node != "" {
			cmdName = node
		} else if path, err := exec.LookPath("node"); err == nil {
			cmdName = path
		} else {
			cmdName = "node"
		}
	}
	if nodeScript := os.Getenv("KUBE_SHELL_NODE_SCRIPT"); nodeScript != "" && len(args) > 0 && args[0] == "apply" && !scripted {
		args = append([]string{nodeScript}, args...)
	}
	cmd := exec.Command(cmdName, args...)
	cmd.Env = command.Env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if command.Input != "" {
		cmd.Stdin = strings.NewReader(command.Input)
	}
	err := cmd.Run()
	if err == nil {
		return SandboxResult{Code: 0, Stdout: stdout.String(), Stderr: stderr.String()}
	}
	code := 1
	if exit, ok := err.(*exec.ExitError); ok {
		code = exit.ExitCode()
	}
	return SandboxResult{Code: code, Stdout: stdout.String(), Stderr: stderr.String()}
}

// RealSandbox builds a host that shells out to kind and kubectl.
func RealSandbox(home string) SandboxHost {
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	return SandboxHost{Home: home, Run: launchSandbox, Has: CommandOnPath}
}
