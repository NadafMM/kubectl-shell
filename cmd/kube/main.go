package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/NadafMM/kubectl-shell/internal/kube/parse"
	kpolicy "github.com/NadafMM/kubectl-shell/internal/kube/policy"
	"github.com/NadafMM/kubectl-shell/internal/kube/redact"
	ksession "github.com/NadafMM/kubectl-shell/internal/kube/session"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/NadafMM/kubectl-shell/internal/kube"
	kcomplete "github.com/NadafMM/kubectl-shell/internal/kube/complete"
	"github.com/NadafMM/kubectl-shell/internal/kube/live"
	"github.com/NadafMM/kubectl-shell/internal/kube/mcp"
	kplan "github.com/NadafMM/kubectl-shell/internal/kube/plan"
	ksandbox "github.com/NadafMM/kubectl-shell/internal/kube/sandbox"
	"github.com/NadafMM/kubectl-shell/internal/kube/undo"
	"github.com/NadafMM/kubectl-shell/internal/kube/view"
)

// version is set at link time via -X main.version=… (see Makefile / scripts/packaging).
// The fallback matches ./VERSION for plain `go build` / `go test`.
var version = "1.0.0"

// runOptions lets tests supply confirm answers and stdio without a real TTY.
type runOptions struct {
	confirm     func(prompt string) bool
	confirmText func(prompt string) string
	stdinTTY    bool
	stdin       io.Reader
	runLive     func(args []string, options live.RunLiveOptions) int
	probeLive   func() bool
	agent       *bool
	forcePlain  bool
	sandboxHost *ksandbox.SandboxHost
}

type session struct {
	yes            bool
	agent          bool
	planID         string
	confirmContext string
	policy         *kpolicy.Policy
	stdinTTY       bool
}

func normalizePluginArgs(exe string, args []string) []string {
	base := strings.TrimSuffix(strings.ToLower(filepath.Base(exe)), ".exe")
	if base == "kubectl-blast" {
		if len(args) == 0 {
			return []string{"--help"}
		}
		switch args[0] {
		case "plan", "--help", "-h", "--version", "-v":
			return args
		default:
			return append([]string{"plan"}, args...)
		}
	}
	return args
}

func main() {
	os.Exit(run(normalizePluginArgs(os.Args[0], os.Args[1:]), os.Stdout, os.Stderr, runOptions{}))
}

func run(args []string, stdout, stderr io.Writer, opts runOptions) int {
	if len(args) > 0 && args[0] == "__complete" {
		return complete(args[1:], stdout)
	}
	flags := parse.ParseLeadingFlags(args)
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
	if flags.Yes && !kpolicy.AllowYes(flags.Yes, len(rest), stdinTTY) {
		fmt.Fprintln(stderr, "`--yes` applies to one command, not the shell.")
		return 1
	}
	yes := flags.Yes
	envPolicy := ""
	if !agent {
		envPolicy = os.Getenv("KUBE_SHELL_POLICY")
	}
	policy, errMsg := kpolicy.ReadPolicy(kpolicy.PolicyFile(agent, flags.Policy, envPolicy))
	if errMsg != "" {
		fmt.Fprintf(stderr, "kube: %s\n", errMsg)
		return 1
	}
	// match_non_tty: non-interactive clients follow agent allow / write_requires.
	if !agent && kpolicy.MatchNonTtyAgent(policy, stdinTTY) {
		agent = true
		policy, errMsg = kpolicy.ReadPolicy(kpolicy.PolicyFile(true, "", ""))
		if errMsg != "" {
			fmt.Fprintf(stderr, "kube: %s\n", errMsg)
			return 1
		}
	}
	host := session{
		yes: yes, agent: agent, planID: flags.PlanID, confirmContext: flags.ConfirmContext,
		policy: policy, stdinTTY: stdinTTY,
	}
	if flags.Record != "" {
		cluster, _ := contextFor("kubectl")
		ns := kube.NamespaceOf(cluster)
		if ns == "" {
			ns = "default"
		}
		ctx := cluster.Current
		if ctx == "" {
			ctx = "none"
		}
		ksession.StartRecord(flags.Record, ctx, ns, time.Now().UTC().Format(time.RFC3339Nano))
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
		script := parse.CompletionScript(shell)
		if script == "" {
			fmt.Fprintln(stderr, "usage: kube completion bash|zsh|fish|powershell")
			return 1
		}
		fmt.Fprint(stdout, script)
		return 0
	case "hook":
		shell := ""
		if len(rest) > 1 {
			shell = rest[1]
		}
		script := parse.HookScript(shell)
		if script == "" {
			fmt.Fprintln(stderr, "usage: kube hook bash|zsh|fish|powershell")
			return 1
		}
		fmt.Fprint(stdout, script)
		return 0
	case "env":
		return envCommand(rest[1:], stdout, stderr)
	case "audit":
		return auditCommand(rest[1:], stdout, stderr)
	case "undo":
		return undoCommand(rest[1:], host, stdout, stderr, opts)
	case "mcp":
		return mcpCommand(rest[1:], host, stdout, stderr, opts)
	}

	meta := parse.MetaOf(strings.Join(rest, " "))
	switch meta {
	case "view", "ui":
		return viewCommand(rest[1:], host, stdout, stderr, opts)
	case "ns", "ctx", "live", "reset", "clear", "exit", "quit":
		// One-shot meta commands use the same line runner as the shell.
		cluster, ok := bootCluster(opts, stderr)
		if !ok {
			return 1
		}
		scope := &ksession.ShellScope{}
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
  kube hook bash|zsh|fish|powershell
  kube env [--ctx <name>] [--ns <name>] [--shell bash|zsh|fish|powershell]
  kube mcp [--read-only] [--context <ctx>] [--namespace <ns>]
  kube __complete --line <line> --cursor <n> [--json]

__complete fills resource names from kubectl when it answers, and keeps that
snapshot for 12 seconds. Delete, drain, and scale-to-zero plans include a
blast-radius report. High-risk writes and guarded commands ask before they run unless --yes is set.
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
	view := parse.ToEngine(line, cursor, "shell")
	if view.Skip {
		return 0
	}
	cluster, ok := live.LiveCluster(view.Line)
	if !ok {
		cluster = kube.Cluster{}
	}
	found := kcomplete.Complete(view.Line, view.Cursor, cluster)
	var values []kube.Candidate
	for _, item := range found.Candidates {
		value, ok := parse.CompletionValue(item.Value)
		if !ok {
			continue
		}
		item.Value = value
		item.Detail = parse.PlainText(item.Detail)
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
	cluster, ok := live.LiveCluster(line)
	if !ok {
		return kube.Cluster{}
	}
	return cluster
}

func contextFor(line string) (kube.Cluster, bool) {
	return live.ContextSnapshot(line)
}

func engineLine(argv []string) (string, bool) {
	parts := make([]string, 0, len(argv)+1)
	parts = append(parts, "kubectl")
	for _, arg := range argv {
		quoted, ok := parse.QuoteArg(arg)
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
	cluster, ok := contextFor(line)
	if !ok {
		cluster = kube.Cluster{}
	}
	plan := kplan.PlanCommand(line, cluster)
	argv := append([]string{}, rest...)
	if len(argv) > 0 && (argv[0] == "kubectl" || argv[0] == "k") {
		argv = argv[1:]
	}
	var blast *kplan.Blast
	if wantsBlast(plan) {
		report := liveBlast(plan, argv)
		blast = &report
	}
	if !asJSON {
		text := kplan.FormatPlan(plan)
		if blast != nil {
			if extra := kplan.FormatBlast(*blast); extra != "" {
				text += "\n" + extra
			}
		}
		fmt.Fprintln(stdout, text)
		return 0
	}
	type payload struct {
		kube.Plan
		Blast  *kplan.Blast `json:"blast,omitempty"`
		PlanID string       `json:"planId,omitempty"`
	}
	out := payload{Plan: plan, Blast: blast}
	id, err := kpolicy.MintPlan(kpolicy.PlanExpect{Context: plan.Context, Namespace: plan.Namespace, Argv: argv}, time.Now())
	if err == nil {
		out.PlanID = id
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}
	agentPlan := os.Getenv("KUBE_SHELL_AGENT") == "1"
	if policy, _ := kpolicy.ReadPolicy(kpolicy.PolicyFile(agentPlan, "", "")); ksession.AuditEnabled(policy) {
		actor := "human-cli"
		if agentPlan {
			actor = "agent-cli"
		}
		zero := 0
		ksession.AppendAudit(ksession.AuditEntry{
			ID: ksession.NewAuditId(), Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
			Actor: actor, Context: plan.Context, Namespace: plan.Namespace,
			Argv: redact.RedactArgv(argv), Risk: plan.Risk, Decision: "planned",
			Approver: "none", ExitCode: &zero, DurationMs: 0,
		}, 0)
	}
	fmt.Fprintf(stdout, "%s\n", encoded)
	return 0
}

func wantsBlast(plan kube.Plan) bool {
	if plan.Verb == "delete" || plan.Verb == "drain" || (plan.Verb == "scale" && plan.Risk == "high") {
		return true
	}
	return (plan.Verb == "apply" || plan.Verb == "replace" || plan.Verb == "patch") && plan.Mutating && plan.Risk == "high"
}

func liveBlast(plan kube.Plan, argv []string) kplan.Blast {
	started := time.Now().UnixMilli()
	getJSON := func(args []string) live.KubectlJSONResult {
		left := kplan.BlastBudgetMS - (time.Now().UnixMilli() - started)
		if left <= 0 {
			return live.KubectlJSONResult{OK: false, Error: "timed out"}
		}
		return live.RunJson(args, int(left))
	}
	runDiff := func(args []string) (int, string, string) {
		left := kplan.BlastBudgetMS - (time.Now().UnixMilli() - started)
		if left <= 0 {
			return 2, "", "timed out"
		}
		return live.RunDiff(args, int(left))
	}
	return kplan.BlastRadiusWithOptions(plan, argv, getJSON, func() int64 { return time.Now().UnixMilli() }, kplan.BlastOptions{RunDiff: runDiff})
}

func askCommand(args []string, stdout, stderr io.Writer) int {
	asJSON, rest := stripJSON(args)
	question := strings.Trim(strings.Join(rest, " "), `"'`)
	cluster := clusterFor("kubectl")
	advice := kplan.AskAdvice(question, cluster, nil)
	if asJSON {
		encoded, err := json.Marshal(advice)
		if err != nil {
			fmt.Fprintln(stderr, err.Error())
			return 1
		}
		fmt.Fprintf(stdout, "%s\n", encoded)
		return 0
	}
	fmt.Fprintln(stdout, kplan.FormatAdvice(advice))
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
	lines := ksession.TailAudit(count)
	fmt.Fprintln(stdout, strings.Join(lines, "\n"))
	return 0
}

func execGuardedKubectl(
	plan kube.Plan,
	argv []string,
	policyCtx string,
	cluster kube.Cluster,
	host session,
	stderr io.Writer,
	opts runOptions,
	inheritStdin bool,
) (code int, refusal string, ran bool) {
	started := time.Now()
	auditID := ksession.NewAuditId()
	actor := "human-cli"
	if host.agent {
		actor = "agent-cli"
	} else if host.stdinTTY {
		actor = "human-tty"
	}
	writeAudit := func(decision, approver string, exitCode *int) {
		if !ksession.AuditEnabled(host.policy) {
			return
		}
		ksession.AppendAudit(ksession.AuditEntry{
			ID: auditID, Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
			Actor: actor, Context: plan.Context, Namespace: plan.Namespace,
			Argv: redact.RedactArgv(argv), Risk: plan.Risk, Decision: decision,
			Approver: approver, ExitCode: exitCode, DurationMs: int(time.Since(started).Milliseconds()),
		}, 0)
	}
	auth := kpolicy.Authorize(kpolicy.AuthRequest{
		Plan: plan, Argv: argv, Policy: host.policy, Agent: host.agent,
		Yes: host.yes, ConfirmContext: host.confirmContext, PlanID: host.planID,
		PolicyContext: policyCtx,
	}, func(planID string, expect kpolicy.PlanExpect) (bool, string) {
		return kpolicy.ConsumePlan(planID, expect, time.Now())
	})
	blastText := ""
	if !auth.OK {
		writeAudit(auth.AuditDecision, "none", nil)
		return 1, auth.Refuse, false
	}
	approver := auth.Approver
	if auth.RequireConfirm {
		text := kplan.FormatPlan(plan)
		if wantsBlast(plan) {
			if extra := kplan.FormatBlast(liveBlast(plan, argv)); extra != "" {
				blastText = extra
				text += "\n" + extra
			}
		}
		fmt.Fprintln(stderr, text)
		if auth.Rule.Confirm == "typed" {
			ctxName := kpolicy.EffectiveContext(plan.Context, policyCtx)
			var ans string
			switch {
			case opts.confirmText != nil:
				ans = opts.confirmText("Type context name (" + ctxName + ") to run: ")
			case opts.confirm != nil:
				ans = ""
			default:
				ans = defaultConfirmText(stderr)("Type context name (" + ctxName + ") to run: ")
			}
			if strings.TrimSpace(ans) != ctxName || ctxName == "" {
				writeAudit("refused-confirm", "none", nil)
				return 1, "not run", false
			}
			approver = "typed-context"
		} else {
			confirm := opts.confirm
			if confirm == nil {
				confirm = defaultConfirm(stderr)
			}
			if !confirm("Run it? [y/N] ") {
				writeAudit("refused-confirm", "none", nil)
				return 1, "not run", false
			}
		}
	} else if warn := redact.SensitiveReadWarning(argv); warn != "" {
		fmt.Fprintln(stderr, warn)
	}
	dryRun := false
	for _, token := range argv {
		if token == "--dry-run" || strings.HasPrefix(token, "--dry-run=") {
			dryRun = true
			break
		}
	}
	if !dryRun {
		undo.RememberUndo(argv, cluster, auditID, time.Now())
	}
	runner := opts.runLive
	if runner == nil {
		runner = live.RunLive
	}
	code = runner(argv, live.RunLiveOptions{InheritStdin: inheritStdin})
	decision := "executed"
	if dryRun {
		decision = "dry-run"
	}
	writeAudit(decision, approver, &code)
	entry := ksession.RunbookEntry{
		Decision: decision, Verb: plan.Verb,
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Context:   plan.Context, Namespace: plan.Namespace,
		Command: ksession.RedactCommand(argv), Risk: plan.Risk, ExitCode: &code,
		Blast: blastText,
	}
	if plan.Mutating {
		entry.Summary = plan.Summary
	}
	ksession.NoteRan(entry)
	return code, "", true
}

func runKubectl(argv []string, host session, stdout, stderr io.Writer, opts runOptions) int {
	if opts.probeLive != nil && !opts.probeLive() {
		fmt.Fprintln(stderr, "kubectl could not reach a cluster. kube only runs against a reachable cluster.")
		return 1
	}
	line, ok := engineLine(argv)
	if !ok {
		fmt.Fprintln(stderr, "cannot pass an argument that contains both quote styles")
		return 1
	}
	cluster, ok := contextFor(line)
	if !ok {
		if opts.probeLive == nil {
			fmt.Fprintln(stderr, "kubectl could not reach a cluster. kube only runs against a reachable cluster.")
			return 1
		}
		cluster = kube.Cluster{}
	}
	plan := kplan.PlanCommand(line, cluster)
	code, refusal, ran := execGuardedKubectl(plan, argv, cluster.Current, cluster, host, stderr, opts, true)
	if !ran && refusal != "" {
		fmt.Fprintln(stdout, refusal)
	}
	return code
}

func envCommand(args []string, stdout, stderr io.Writer) int {
	var ctxName, nsName, shell, sessionID string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--ctx" || arg == "--context":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "kube env: "+arg+" needs a value")
				return 1
			}
			i++
			ctxName = args[i]
		case strings.HasPrefix(arg, "--ctx="):
			ctxName = arg[len("--ctx="):]
		case strings.HasPrefix(arg, "--context="):
			ctxName = arg[len("--context="):]
		case arg == "--ns" || arg == "--namespace":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "kube env: "+arg+" needs a value")
				return 1
			}
			i++
			nsName = args[i]
		case strings.HasPrefix(arg, "--ns="):
			nsName = arg[len("--ns="):]
		case strings.HasPrefix(arg, "--namespace="):
			nsName = arg[len("--namespace="):]
		case arg == "--shell":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "kube env: --shell needs a value")
				return 1
			}
			i++
			shell = args[i]
		case strings.HasPrefix(arg, "--shell="):
			shell = arg[len("--shell="):]
		case arg == "--session":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "kube env: --session needs a value")
				return 1
			}
			i++
			sessionID = args[i]
		case strings.HasPrefix(arg, "--session="):
			sessionID = arg[len("--session="):]
		default:
			fmt.Fprintln(stderr, "usage: kube env [--ctx <name>] [--ns <name>] [--shell bash|zsh|fish|powershell] [--session <id>]")
			return 1
		}
	}
	basePath := os.Getenv("KUBE_SHELL_BASE_KUBECONFIG")
	if basePath == "" {
		basePath = os.Getenv("KUBECONFIG")
	}
	scopedPath, _, _, err := ksession.WriteScopedKubeconfig(ctxName, nsName, sessionID, time.Now())
	if err != nil {
		fmt.Fprintln(stderr, "kube env:", err)
		return 1
	}
	if shell == "" {
		shell = "bash"
	}
	snippet := ksession.FormatScopeEnv(shell, scopedPath, basePath)
	if snippet == "" {
		fmt.Fprintln(stderr, "usage: kube env --shell bash|zsh|fish|powershell")
		return 1
	}
	fmt.Fprint(stdout, snippet)
	return 0
}

func mcpCommand(args []string, host session, stdout, stderr io.Writer, opts runOptions) int {
	var mcpOpts mcp.McpOptions
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--read-only":
			mcpOpts.ReadOnly = true
		case arg == "--context":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "usage: kube mcp [--read-only] [--context <ctx>] [--namespace <ns>]")
				return 1
			}
			i++
			mcpOpts.Context = args[i]
		case strings.HasPrefix(arg, "--context="):
			mcpOpts.Context = arg[len("--context="):]
		case arg == "--namespace" || arg == "-n":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "usage: kube mcp [--read-only] [--context <ctx>] [--namespace <ns>]")
				return 1
			}
			i++
			mcpOpts.Namespace = args[i]
		case strings.HasPrefix(arg, "--namespace="):
			mcpOpts.Namespace = arg[len("--namespace="):]
		case strings.HasPrefix(arg, "-n="):
			mcpOpts.Namespace = arg[len("-n="):]
		default:
			fmt.Fprintln(stderr, "usage: kube mcp [--read-only] [--context <ctx>] [--namespace <ns>]")
			return 1
		}
	}
	if opts.probeLive != nil && !opts.probeLive() {
		fmt.Fprintln(stderr, "kubectl could not reach a cluster. kube only runs against a reachable cluster.")
		return 1
	}
	cluster, ok := contextFor("kubectl")
	if !ok && opts.probeLive == nil {
		fmt.Fprintln(stderr, "kubectl could not reach a cluster. kube only runs against a reachable cluster.")
		return 1
	}
	stdin := opts.stdin
	if stdin == nil {
		stdin = os.Stdin
	}
	clusterFn := func() kube.Cluster {
		if fresh, fok := contextFor("kubectl"); fok {
			return fresh
		}
		return cluster
	}
	if err := mcp.ServeMcpOptions(clusterFn, version, stdin, stdout, mcpOpts); err != nil {
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
		h := ksandbox.RealSandbox("")
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
				ok, message = ksandbox.SandboxUp(*host)
			} else {
				ok, message = ksandbox.SandboxDown(*host)
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
		config := ksandbox.SandboxKubeconfig(host.Home)
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
		probe = live.ProbeLive
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
	board := view.BoardFrom(word)
	line := "kubectl get pods"
	if all {
		line = "kubectl get pods -A"
	}
	cluster, ok := live.LiveCluster(line)
	if !ok {
		fmt.Fprintln(stderr, "kubectl cannot describe the current context")
		return 1
	}
	forcePlain := opts.forcePlain
	stdoutFile, _ := stdout.(*os.File)
	if stdoutFile == nil {
		forcePlain = true
	}
	rule := kpolicy.RuleFor(host.policy, cluster.Current)
	view.OpenView(cluster, view.ViewOptions{
		Board:        board,
		All:          all,
		Yes:          host.yes,
		TypedConfirm: rule.Confirm == "typed",
		ForcePlain:   forcePlain,
		Stdout:       stdout,
		StdoutFile:   stdoutFile,
		Policy:       host.policy,
		Reload: func(wantAll bool, namespace string) (kube.Cluster, bool) {
			live.InvalidateLive()
			reloadLine := "kubectl get pods -n " + namespace
			if wantAll {
				reloadLine = "kubectl get pods -A"
			}
			return live.LiveCluster(reloadLine)
		},
	})
	return 0
}

func undoCommand(args []string, host session, stdout, stderr io.Writer, opts runOptions) int {
	if len(args) > 0 && args[0] == "list" {
		lines := make([]string, 0)
		for _, record := range undo.ListUndo(time.Now()) {
			state := "restorable"
			if record.Refused != "" {
				state = "not restorable"
			}
			if record.ManagedBy != "" {
				state += " [managed by " + record.ManagedBy + "]"
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
	record, ok := undo.LatestUndo(time.Now())
	if !ok {
		fmt.Fprintln(stdout, "nothing to undo")
		return 1
	}
	if refusal := undo.UndoRefusal(record); refusal != "" {
		fmt.Fprintln(stdout, refusal)
		return 1
	}
	if warn := undo.UndoGitOpsWarning(record); warn != "" {
		fmt.Fprintln(stderr, warn)
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
		runner = live.RunLive
	}
	restore := undo.RestoreArgs(record)
	options := live.RunLiveOptions{InheritStdin: true}
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
		in, out, closeTTY, err := kube.OpenTTYPrompt(stderr)
		if err != nil {
			fmt.Fprintf(stderr, "\nkube: could not read the answer (%v); not running it.\n", err)
			return false
		}
		defer closeTTY()
		return kpolicy.ConfirmLine(out, in, prompt)
	}
}

func defaultConfirmText(stderr io.Writer) func(string) string {
	return func(prompt string) string {
		in, out, closeTTY, err := kube.OpenTTYPrompt(stderr)
		if err != nil {
			fmt.Fprintf(stderr, "\nkube: could not read the answer (%v); not running it.\n", err)
			return ""
		}
		defer closeTTY()
		_, _ = io.WriteString(out, prompt)
		return kpolicy.ReadExactLine(in, func(text string) {
			_, _ = io.WriteString(stderr, text)
		})
	}
}
