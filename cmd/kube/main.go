package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/NadafMM/kubectl-shell/internal/kube"
)

const version = "0.5.1"

// runOptions lets tests supply confirm answers and stdio without a real TTY.
type runOptions struct {
	confirm   func(prompt string) bool
	stdinTTY  bool
	runLive   func(args []string, options kube.RunLiveOptions) int
	probeLive func() bool
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, runOptions{}))
}

func run(args []string, stdout, stderr io.Writer, opts runOptions) int {
	if len(args) > 0 && args[0] == "__complete" {
		return complete(args[1:], stdout)
	}
	flags := kube.ParseLeadingFlags(args)
	if flags.Error != "" {
		fmt.Fprintln(stderr, flags.Error)
		return 1
	}
	if flags.Sample {
		fmt.Fprintln(stderr, "sample mode was removed. kube only runs against a reachable cluster.")
		return 1
	}
	if flags.Sandbox {
		fmt.Fprintln(stderr, "the Go port has not implemented sandbox yet. Use the TypeScript build.")
		return 2
	}
	if flags.Policy != "" || flags.PlanID != "" || flags.ConfirmContext != "" {
		fmt.Fprintln(stderr, "the Go port has not implemented policy and plan-id yet. Use the TypeScript build.")
		return 2
	}
	rest := flags.Rest
	stdinTTY := opts.stdinTTY
	if opts.confirm == nil {
		fi, err := os.Stdin.Stat()
		stdinTTY = err == nil && fi.Mode()&os.ModeCharDevice != 0
	}
	yes := kube.AllowYes(flags.Yes, len(rest), stdinTTY)

	if len(rest) == 0 {
		fmt.Fprintln(stderr, "the Go port does not open the shell yet. Try `kube --help`.")
		return 2
	}

	switch rest[0] {
	case "--version", "-v":
		fmt.Fprintln(stdout, version)
		return 0
	case "--help", "-h", "help":
		fmt.Fprintln(stdout, helpText())
		return 0
	case "plan":
		return planCommand(rest[1:], stdout, stderr)
	case "ask":
		return askCommand(rest[1:], stdout, stderr)
	case "completion":
		shell := ""
		if len(rest) > 1 {
			shell = rest[1]
		}
		script := kube.CompletionScript(shell)
		if script == "" {
			fmt.Fprintln(stderr, "usage: kube completion bash|zsh|fish")
			return 1
		}
		fmt.Fprint(stdout, script)
		return 0
	}

	meta := kube.MetaOf(strings.Join(rest, " "))
	switch meta {
	case "view", "ui", "ns", "ctx", "undo", "audit", "live", "reset", "clear", "exit", "quit", "mcp":
		fmt.Fprintf(stderr, "the Go port has not implemented %q yet. Use the TypeScript build.\n", rest[0])
		return 2
	}

	return runKubectl(rest, yes, stdout, stderr, opts)
}

func helpText() string {
	return `kube ` + version + ` (go)

This binary runs kubectl through a plan when risk is high, completes words, and answers ask.
The interactive shell and view still run from the TypeScript build.

  kube --version
  kube --help
  kube get pods
  kube --yes delete pod <name>
  kube plan [--json] <kubectl args>
  kube ask [--json] <question>
  kube completion bash|zsh|fish
  kube __complete --line <line> --cursor <n> [--json]

__complete fills resource names from kubectl when it answers, and keeps that
snapshot for 12 seconds. Delete, drain, and scale-to-zero plans include a
blast-radius report. High-risk commands ask before they run unless --yes is set.
Reads keep the completion cache; writes clear it.`
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
	cluster, ok := kube.LiveCluster(view.Line)
	if !ok {
		cluster = kube.Cluster{}
	}
	found := kube.Complete(view.Line, view.Cursor, cluster)
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

func stripJSON(args []string) (bool, []string) {
	asJSON := false
	var rest []string
	for _, arg := range args {
		if arg == "--json" {
			asJSON = true
			continue
		}
		rest = append(rest, arg)
	}
	return asJSON, rest
}

func clusterFor(line string) kube.Cluster {
	cluster, ok := kube.LiveCluster(line)
	if !ok {
		return kube.Cluster{}
	}
	return cluster
}

func engineLine(argv []string) (string, bool) {
	parts := make([]string, 0, len(argv)+1)
	parts = append(parts, "kubectl")
	for _, arg := range argv {
		quoted, ok := kube.QuoteArg(arg)
		if !ok {
			return "", false
		}
		parts = append(parts, quoted)
	}
	return strings.Join(parts, " "), true
}

func planCommand(args []string, stdout, stderr io.Writer) int {
	asJSON, rest := stripJSON(args)
	if len(rest) == 0 {
		fmt.Fprintln(stderr, "usage: kube plan [--json] delete pod <name>")
		return 1
	}
	line := strings.Join(rest, " ")
	if rest[0] != "kubectl" && rest[0] != "k" {
		line = "kubectl " + line
	}
	cluster := clusterFor(line)
	plan := kube.PlanCommand(line, cluster)
	argv := append([]string{}, rest...)
	if len(argv) > 0 && (argv[0] == "kubectl" || argv[0] == "k") {
		argv = argv[1:]
	}
	var blast *kube.Blast
	if wantsBlast(plan) {
		report := liveBlast(plan, argv)
		blast = &report
	}
	if !asJSON {
		text := kube.FormatPlan(plan)
		if blast != nil {
			if extra := kube.FormatBlast(*blast); extra != "" {
				text += "\n" + extra
			}
		}
		fmt.Fprintln(stdout, text)
		return 0
	}
	type payload struct {
		kube.Plan
		Blast  *kube.Blast `json:"blast,omitempty"`
		PlanID string      `json:"planId,omitempty"`
	}
	out := payload{Plan: plan, Blast: blast}
	id, err := kube.MintPlan(kube.PlanExpect{Context: plan.Context, Namespace: plan.Namespace, Argv: argv}, time.Now())
	if err == nil {
		out.PlanID = id
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}
	fmt.Fprintf(stdout, "%s\n", encoded)
	return 0
}

func wantsBlast(plan kube.Plan) bool {
	return plan.Verb == "delete" || plan.Verb == "drain" || (plan.Verb == "scale" && plan.Risk == "high")
}

func liveBlast(plan kube.Plan, argv []string) kube.Blast {
	started := time.Now().UnixMilli()
	getJSON := func(args []string) kube.KubectlJSONResult {
		left := kube.BlastBudgetMS - (time.Now().UnixMilli() - started)
		if left <= 0 {
			return kube.KubectlJSONResult{OK: false, Error: "timed out"}
		}
		return kube.RunJson(args, int(left))
	}
	return kube.BlastRadius(plan, argv, getJSON, func() int64 { return time.Now().UnixMilli() })
}

func askCommand(args []string, stdout, stderr io.Writer) int {
	asJSON, rest := stripJSON(args)
	question := strings.Trim(strings.Join(rest, " "), `"'`)
	advice := kube.Advise(question, clusterFor("kubectl"))
	if asJSON {
		encoded, err := json.Marshal(advice)
		if err != nil {
			fmt.Fprintln(stderr, err.Error())
			return 1
		}
		fmt.Fprintf(stdout, "%s\n", encoded)
		return 0
	}
	fmt.Fprintln(stdout, kube.FormatAdvice(advice))
	return 0
}

func runKubectl(argv []string, yes bool, stdout, stderr io.Writer, opts runOptions) int {
	probe := opts.probeLive
	if probe == nil {
		probe = kube.ProbeLive
	}
	if !probe() {
		fmt.Fprintln(stderr, "kubectl could not reach a cluster. kube only runs against a reachable cluster.")
		return 1
	}
	line, ok := engineLine(argv)
	if !ok {
		fmt.Fprintln(stderr, "cannot pass an argument that contains both quote styles")
		return 1
	}
	cluster := clusterFor(line)
	plan := kube.PlanCommand(line, cluster)
	if plan.Risk == "high" && !yes {
		text := kube.FormatPlan(plan)
		if wantsBlast(plan) {
			if extra := kube.FormatBlast(liveBlast(plan, argv)); extra != "" {
				text += "\n" + extra
			}
		}
		fmt.Fprintln(stderr, text)
		confirm := opts.confirm
		if confirm == nil {
			confirm = defaultConfirm(stderr)
		}
		if !confirm("Run it? [y/N] ") {
			fmt.Fprintln(stdout, "not run")
			return 1
		}
	}
	runner := opts.runLive
	if runner == nil {
		runner = kube.RunLive
	}
	code := runner(argv, kube.RunLiveOptions{InheritStdin: true})
	if code == 0 {
		return 0
	}
	return code
}

func defaultConfirm(stderr io.Writer) func(string) bool {
	return func(prompt string) bool {
		tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
		if err != nil {
			fmt.Fprintf(stderr, "\nkube: could not read the answer (%v); not running it.\n", err)
			return false
		}
		defer tty.Close()
		return kube.ConfirmLine(tty, tty, prompt)
	}
}
