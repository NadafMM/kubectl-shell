package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/NadafMM/kubectl-shell/internal/kube"
)

const version = "0.5.1"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "__complete" {
		return complete(args[1:], stdout)
	}
	if len(args) == 0 {
		fmt.Fprintln(stderr, "the Go port does not open the shell yet. Try `kube --help`.")
		return 2
	}
	switch args[0] {
	case "--version", "-v":
		fmt.Fprintln(stdout, version)
		return 0
	case "--help", "-h", "help":
		fmt.Fprintln(stdout, helpText())
		return 0
	case "completion":
		shell := ""
		if len(args) > 1 {
			shell = args[1]
		}
		script := kube.CompletionScript(shell)
		if script == "" {
			fmt.Fprintln(stderr, "usage: kube completion bash|zsh|fish")
			return 1
		}
		fmt.Fprint(stdout, script)
		return 0
	default:
		fmt.Fprintf(stderr, "the Go port has not implemented %q yet. kubectl execution, plan, and the shell are still the TypeScript build.\n", args[0])
		return 2
	}
}

func helpText() string {
	return `kube ` + version + ` (go)

This binary is the start of the Go port. It completes words and prints shell scripts.
The interactive shell, plan, view, and kubectl execution still run from the TypeScript build.

  kube --version
  kube --help
  kube completion bash|zsh|fish
  kube __complete --line <line> --cursor <n> [--json]

Live object names are not loaded yet. Completion uses the static kubectl catalog.`
}

func complete(args []string, stdout io.Writer) int {
	cursor := 0
	cursorSet := false
	line := ""
	asJSON := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--cursor":
			if i+1 < len(args) {
				i++
				n, err := strconv.Atoi(args[i])
				if err == nil {
					cursor = n
					cursorSet = true
				}
			}
		case "--json":
			asJSON = true
		case "--line":
			if i+1 < len(args) {
				line = args[i+1]
			}
			i = len(args)
		}
	}
	if !cursorSet {
		cursor = len(line)
	}
	view := kube.ToEngine(line, cursor, "shell")
	if view.Skip {
		return 0
	}
	found := kube.Complete(view.Line, view.Cursor, kube.Cluster{})
	var values []kube.Candidate
	for _, item := range found.Candidates {
		value, ok := kube.CompletionValue(item.Value)
		if !ok {
			continue
		}
		item.Value = value
		item.Detail = kube.PlainText(item.Detail)
		values = append(values, item)
	}
	if asJSON {
		type row struct {
			Value  string `json:"value"`
			Detail string `json:"detail"`
			Kind   string `json:"kind"`
		}
		out := make([]row, 0, len(values))
		for _, item := range values {
			out = append(out, row{Value: item.Value, Detail: item.Detail, Kind: item.Kind})
		}
		encoded, err := json.Marshal(out)
		if err != nil {
			return 1
		}
		fmt.Fprintf(stdout, "%s\n", encoded)
		return 0
	}
	lines := make([]string, 0, len(values))
	for _, item := range values {
		lines = append(lines, item.Value)
	}
	if len(lines) > 0 {
		fmt.Fprintln(stdout, strings.Join(lines, "\n"))
	}
	return 0
}
