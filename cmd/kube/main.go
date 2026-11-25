package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/NadafMM/kubectl-shell/internal/kube"
)

// version is set at link time via -X main.version=… (see Makefile / scripts/packaging).
// The fallback matches ./VERSION for plain `go build` / `go test`.
var version = "0.6.0"

// runOptions lets tests supply confirm answers and stdio without a real TTY.
type runOptions struct {
	confirm     func(prompt string) bool
	stdinTTY    bool
	stdin       io.Reader
	runLive     func(args []string, options kube.RunLiveOptions) int
	probeLive   func() bool
	agent       *bool
	forcePlain  bool
	sandboxHost *kube.SandboxHost
}

type session struct {
	yes            bool
	agent          bool
	planID         string
	confirmContext string
	policy         *kube.Policy
	stdinTTY       bool
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
	rest, code, done := prepareSandbox(flags.Sandbox, flags.Rest, stdout, stderr, opts)
	if done {
		return code
	}
	stdinTTY := opts.stdinTTY
	if opts.stdin != nil {
		stdinTTY = false
	} else if opts.confirm == nil && opts.agent == nil {
		fi, err := os.Stdin.Stat()
		stdinTTY = err == nil && fi.Mode()&os.ModeCharDevice != 0
	}
	agent := os.Getenv("KUBE_SHELL_AGENT") == "1"
	if opts.agent != nil {
		agent = *opts.agent
	}
	if flags.Yes && !kube.AllowYes(flags.Yes, len(rest), stdinTTY) {
		fmt.Fprintln(stderr, "`--yes` applies to one command, not the shell.")
		return 1
	}
	yes := flags.Yes
	envPolicy := ""
	if !agent {
		envPolicy = os.Getenv("KUBE_SHELL_POLICY")
	}
	policy, errMsg := kube.ReadPolicy(kube.PolicyFile(agent, flags.Policy, envPolicy))
	if errMsg != "" {
		fmt.Fprintf(stderr, "kube: %s\n", errMsg)
		return 1
	}
	host := session{
		yes: yes, agent: agent, planID: flags.PlanID, confirmContext: flags.ConfirmContext,
		policy: policy, stdinTTY: stdinTTY,
	}
	if flags.Record != "" {
		cluster := clusterFor("kubectl")
		ns := kube.NamespaceOf(cluster)
		if ns == "" {
			ns = "default"
		}
		ctx := cluster.Current
		if ctx == "" {
			ctx = "none"
		}
		kube.StartRecord(flags.Record, ctx, ns, time.Now().UTC().Format(time.RFC3339Nano))
	}

	if len(rest) == 0 {
		if !stdinTTY {
			stdin := opts.stdin
			if stdin == nil {
				stdin = os.Stdin
			}
			return runScript(stdin, host, stdout, stderr, opts)
		}
		return startRepl(host, stdout, stderr, opts)
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
			fmt.Fprintln(stderr, "usage: kube completion bash|zsh|fish|powershell")
			return 1
		}
		fmt.Fprint(stdout, script)
		return 0
	case "audit":
		return auditCommand(rest[1:], stdout, stderr)
	case "undo":
		return undoCommand(rest[1:], host, stdout, stderr, opts)
	case "mcp":
		return mcpCommand(host, stdout, stderr, opts)
	}

	meta := kube.MetaOf(strings.Join(rest, " "))
	switch meta {
	case "view", "ui":
		return viewCommand(rest[1:], host, stdout, stderr, opts)
	case "ns", "ctx", "live", "reset", "clear", "exit", "quit":
		// One-shot meta commands use the same line runner as the shell.
		cluster, ok := bootCluster(opts, stderr)
		if !ok {
			return 1
		}
		scope := &kube.ShellScope{}
		result := runUserLine(strings.Join(rest, " "), cluster, makeLineHost(host, stdout, stderr, opts, scope))
		if result.output != "\x1bc" {
			writeLineResult(result, stdout, stderr)
		} else {
			fmt.Fprint(stdout, "\x1b[H\x1b[2J")
		}
		if result.ok {
			return 0
		}
		return 1
	}

	return runKubectl(rest, host, stdout, stderr, opts)
}

func helpText() string {
	return `kube ` + version + ` (go)

This binary runs kubectl through a plan when risk is high, completes words, and answers ask.
Policy, plan-id, audit, undo, view, sandbox, and mcp apply. With no args it opens the interactive shell
(or runs a script from stdin when there is no TTY).

  kube                         shell. Tab fills the next word
  kube --version
  kube --help
  kube get pods
  kube --yes delete pod <name>
  kube view [pods|deploy|svc|nodes|ns] [-A]
  kube sandbox up|down|shell
  kube --sandbox get pods
  kube --record session.md
  kube mcp
  kube plan [--json] <kubectl args>
  kube ask [--json] <question>
  kube audit tail [-n 20]
  kube undo
  kube undo list
  kube ns <name> | kube ctx <name>
  kube completion bash|zsh|fish|powershell
  kube __complete --line <line> --cursor <n> [--json]

__complete fills resource names from kubectl when it answers, and keeps that
snapshot for 12 seconds. Delete, drain, and scale-to-zero plans include a
blast-radius report. High-risk and guarded commands ask before they run unless --yes is set.
Reads keep the completion cache; writes clear it. Mutating named edits can be undone.
Without a TTY, view prints a plain table. Inside the shell, ns and ctx change scope only.
record on|off (or --record) writes a redacted Markdown runbook of commands that ran.
sandbox uses a separate kubeconfig under ~/.kube-shell/sandbox and does not rewrite ~/.kube/config.`
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
	cluster := clusterFor("kubectl")
	advice := kube.AskAdvice(question, cluster, nil)
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

func auditCommand(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] != "tail" {
		fmt.Fprintln(stderr, "usage: kube audit tail [-n 20]")
		return 1
	}
	count := 20
	for i := 0; i < len(args); i++ {
		if args[i] == "-n" && i+1 < len(args) {
			n, err := strconv.Atoi(args[i+1])
			if err == nil {
				count = n
			}
		}
	}
	lines := kube.TailAudit(count)
	fmt.Fprintln(stdout, strings.Join(lines, "\n"))
	return 0
}

func containsVerb(verbs []string, verb string) bool {
	for _, item := range verbs {
		if item == verb {
			return true
		}
	}
	return false
}

func runKubectl(argv []string, host session, stdout, stderr io.Writer, opts runOptions) int {
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
	started := time.Now()
	auditID := kube.NewAuditId()
	cluster := clusterFor(line)
	plan := kube.PlanCommand(line, cluster)
	rule := kube.RuleFor(host.policy, cluster.Current)
	actor := "human-cli"
	if host.agent {
		actor = "agent-cli"
	} else if host.stdinTTY {
		actor = "human-tty"
	}
	writeAudit := func(decision, approver string, exitCode *int) {
		if !kube.AuditEnabled(host.policy) {
			return
		}
		kube.AppendAudit(kube.AuditEntry{
			ID: auditID, Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
			Actor: actor, Context: plan.Context, Namespace: plan.Namespace,
			Argv: kube.RedactArgv(argv), Risk: plan.Risk, Decision: decision,
			Approver: approver, ExitCode: exitCode, DurationMs: int(time.Since(started).Milliseconds()),
		}, 0)
	}
	if containsVerb(rule.Deny, plan.Verb) {
		writeAudit("denied-policy", "none", nil)
		fmt.Fprintf(stdout, "refused: %s is denied by policy\n", plan.Verb)
		return 1
	}
	if host.agent && host.policy != nil && host.policy.Agents != nil && plan.Risk == "read" && !containsVerb(host.policy.Agents.Allow, plan.Verb) {
		writeAudit("denied-policy", "none", nil)
		fmt.Fprintf(stdout, "refused: %s is not allowed for agents\n", plan.Verb)
		return 1
	}
	needsPlan := host.agent && host.policy != nil && host.policy.Agents != nil && host.policy.Agents.WriteRequires == "approved-plan" && plan.Mutating
	blastText := ""
	if needsPlan {
		if host.planID == "" {
			writeAudit("denied-policy", "none", nil)
			fmt.Fprintln(stdout, "refused: agent writes need --plan-id from plan --json")
			return 1
		}
		ok, reason := kube.ConsumePlan(host.planID, kube.PlanExpect{Context: plan.Context, Namespace: plan.Namespace, Argv: argv}, time.Now())
		if !ok {
			writeAudit("denied-policy", "none", nil)
			fmt.Fprintf(stdout, "refused: %s\n", reason)
			return 1
		}
	} else if rule.Confirm == "typed" && host.yes && host.confirmContext != cluster.Current {
		writeAudit("refused-confirm", "none", nil)
		fmt.Fprintf(stdout, "refused: pass --confirm-context %s\n", cluster.Current)
		return 1
	} else if (plan.Risk == "high" || (rule.Mode == "guarded" && plan.Risk == "low")) && !host.yes {
		text := kube.FormatPlan(plan)
		if wantsBlast(plan) {
			if extra := kube.FormatBlast(liveBlast(plan, argv)); extra != "" {
				blastText = extra
				text += "\n" + extra
			}
		}
		fmt.Fprintln(stderr, text)
		confirm := opts.confirm
		if confirm == nil {
			confirm = defaultConfirm(stderr)
		}
		if !confirm("Run it? [y/N] ") {
			writeAudit("refused-confirm", "none", nil)
			fmt.Fprintln(stdout, "not run")
			return 1
		}
	}
	approver := "prompt-y"
	if needsPlan {
		approver = "plan-id:" + host.planID
	} else if rule.Confirm == "typed" && host.yes {
		approver = "typed-context"
	} else if host.yes {
		approver = "flag-yes"
	}
	dryRun := false
	for _, token := range argv {
		if token == "--dry-run" || strings.HasPrefix(token, "--dry-run=") {
			dryRun = true
			break
		}
	}
	if !dryRun {
		kube.RememberUndo(argv, cluster, auditID, time.Now())
	}
	runner := opts.runLive
	if runner == nil {
		runner = kube.RunLive
	}
	code := runner(argv, kube.RunLiveOptions{InheritStdin: true})
	decision := "executed"
	if dryRun {
		decision = "dry-run"
	}
	writeAudit(decision, approver, &code)
	entry := kube.RunbookEntry{
		Decision: decision, Verb: plan.Verb,
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Context:   plan.Context, Namespace: plan.Namespace,
		Command: kube.RedactCommand(argv), Risk: plan.Risk, ExitCode: &code,
		Blast: blastText,
	}
	if plan.Mutating {
		entry.Summary = plan.Summary
	}
	kube.NoteRan(entry)
	return code
}

func mcpCommand(host session, stdout, stderr io.Writer, opts runOptions) int {
	probe := opts.probeLive
	if probe == nil {
		probe = kube.ProbeLive
	}
	if !probe() {
		fmt.Fprintln(stderr, "kubectl could not reach a cluster. kube only runs against a reachable cluster.")
		return 1
	}
	cluster, ok := kube.LiveCluster("kubectl")
	if !ok {
		fmt.Fprintln(stderr, "kubectl could not reach a cluster. kube only runs against a reachable cluster.")
		return 1
	}
	stdin := opts.stdin
	if stdin == nil {
		stdin = os.Stdin
	}
	if err := kube.ServeMcp(cluster, version, stdin, stdout); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}
	_ = host
	return 0
}

// prepareSandbox handles kube sandbox up|down|shell and --sandbox KUBECONFIG.
// When done is true, the caller should return code immediately.
func prepareSandbox(sandbox bool, rest []string, stdout, stderr io.Writer, opts runOptions) (next []string, code int, done bool) {
	host := opts.sandboxHost
	if host == nil {
		h := kube.RealSandbox("")
		host = &h
	}
	if len(rest) > 0 && rest[0] == "sandbox" {
		action := ""
		if len(rest) > 1 {
			action = rest[1]
		}
		if len(rest) != 2 || (action != "up" && action != "down" && action != "shell") {
			fmt.Fprintln(stderr, "usage: kube sandbox up|down|shell")
			return rest, 1, true
		}
		if action == "up" || action == "down" {
			var ok bool
			var message string
			if action == "up" {
				ok, message = kube.SandboxUp(*host)
			} else {
				ok, message = kube.SandboxDown(*host)
			}
			text := message
			if !strings.HasSuffix(text, "\n") {
				text += "\n"
			}
			if ok {
				fmt.Fprint(stdout, text)
				return rest, 0, true
			}
			fmt.Fprint(stderr, text)
			return rest, 1, true
		}
		rest = nil
		sandbox = true
	}
	if sandbox {
		config := kube.SandboxKubeconfig(host.Home)
		if _, err := os.Stat(config); err != nil {
			fmt.Fprintln(stderr, "kube sandbox is not up. Run kube sandbox up.")
			return rest, 1, true
		}
		_ = os.Setenv("KUBECONFIG", config)
	}
	return rest, 0, false
}

func viewCommand(args []string, host session, stdout, stderr io.Writer, opts runOptions) int {
	probe := opts.probeLive
	if probe == nil {
		probe = kube.ProbeLive
	}
	if !probe() {
		fmt.Fprintln(stderr, "kubectl cannot reach the cluster")
		return 1
	}
	all := false
	word := ""
	for _, arg := range args {
		switch arg {
		case "-A", "--all-namespaces", "all":
			all = true
		default:
			if !strings.HasPrefix(arg, "-") && word == "" {
				word = arg
			}
		}
	}
	board := kube.BoardFrom(word)
	line := "kubectl get pods"
	if all {
		line = "kubectl get pods -A"
	}
	cluster, ok := kube.LiveCluster(line)
	if !ok {
		fmt.Fprintln(stderr, "kubectl cannot describe the current context")
		return 1
	}
	forcePlain := opts.forcePlain
	stdoutFile, _ := stdout.(*os.File)
	if stdoutFile == nil {
		forcePlain = true
	}
	rule := kube.RuleFor(host.policy, cluster.Current)
	kube.OpenView(cluster, kube.ViewOptions{
		Board:        board,
		All:          all,
		Yes:          host.yes,
		TypedConfirm: rule.Confirm == "typed",
		ForcePlain:   forcePlain,
		Stdout:       stdout,
		StdoutFile:   stdoutFile,
		Policy:       host.policy,
		Reload: func(wantAll bool, namespace string) (kube.Cluster, bool) {
			kube.InvalidateLive()
			reloadLine := "kubectl get pods -n " + namespace
			if wantAll {
				reloadLine = "kubectl get pods -A"
			}
			return kube.LiveCluster(reloadLine)
		},
	})
	return 0
}

func undoCommand(args []string, host session, stdout, stderr io.Writer, opts runOptions) int {
	if len(args) > 0 && args[0] == "list" {
		lines := make([]string, 0)
		for _, record := range kube.ListUndo(time.Now()) {
			state := "restorable"
			if record.Refused != "" {
				state = "not restorable"
			}
			lines = append(lines, fmt.Sprintf("%s  %s/%s  %s  %s", record.ID, record.Kind, record.Name, record.Namespace, state))
		}
		if len(lines) == 0 {
			fmt.Fprintln(stdout, "nothing to undo")
			return 0
		}
		fmt.Fprintln(stdout, strings.Join(lines, "\n"))
		return 0
	}
	if len(args) > 0 {
		fmt.Fprintln(stderr, "usage: kube undo | kube undo list")
		return 1
	}
	record, ok := kube.LatestUndo(time.Now())
	if !ok {
		fmt.Fprintln(stdout, "nothing to undo")
		return 1
	}
	if refusal := kube.UndoRefusal(record); refusal != "" {
		fmt.Fprintln(stdout, refusal)
		return 1
	}
	preview := "apply saved " + record.Kind + "/" + record.Name
	if record.Revision != "" {
		preview = "rollout undo " + record.Kind + "/" + record.Name + " --to-revision=" + record.Revision
	}
	if !host.yes {
		confirm := opts.confirm
		if confirm == nil {
			confirm = defaultConfirm(stderr)
		}
		if !confirm("Undo " + preview + "? [y/N] ") {
			fmt.Fprintln(stdout, "not run")
			return 1
		}
	}
	runner := opts.runLive
	if runner == nil {
		runner = kube.RunLive
	}
	restore := kube.RestoreArgs(record)
	options := kube.RunLiveOptions{InheritStdin: true}
	if len(restore) >= 2 && restore[0] == "apply" && record.Snapshot != nil {
		options.StdinReader = bytes.NewReader(record.Snapshot)
		options.InheritStdin = false
	}
	code := runner(restore, options)
	if code == 0 {
		fmt.Fprintf(stdout, "restored %s/%s\n", record.Kind, record.Name)
		return 0
	}
	fmt.Fprintln(stdout, "undo failed")
	return 1
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
