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
	"github.com/NadafMM/kubectl-shell/internal/kube/live"
	kplan "github.com/NadafMM/kubectl-shell/internal/kube/plan"
	"github.com/NadafMM/kubectl-shell/internal/kube/undo"
	"github.com/NadafMM/kubectl-shell/internal/kube/view"
)

type lineResult struct {
	output  string
	ok      bool
	cluster kube.Cluster
	exit    bool
}

type lineHost struct {
	session
	help    string
	version string
	confirm func(string) bool
	stderr  io.Writer
	stdout  io.Writer
	opts    runOptions
	scope   *kube.ShellScope
}

func writeLineResult(result lineResult, stdout, stderr io.Writer) {
	if result.output == "" {
		return
	}
	text := result.output
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	if result.ok {
		fmt.Fprint(stdout, text)
	} else {
		fmt.Fprint(stderr, text)
	}
}

func runUserLine(line string, cluster kube.Cluster, host lineHost) lineResult {
	trimmed := strings.TrimSpace(line)
	meta := kube.MetaOf(trimmed)
	scope := host.scope
	if scope == nil {
		scope = &kube.ShellScope{}
		host.scope = scope
	}

	switch meta {
	case "exit", "quit":
		return lineResult{ok: true, cluster: cluster, exit: true}
	case "reset":
		live.InvalidateLive()
		next, ok := live.LiveCluster("kubectl")
		if !ok {
			next = cluster
		}
		return lineResult{output: "refreshed names from " + next.Current, ok: true, cluster: next}
	case "sample":
		return lineResult{
			output:  "sample mode was removed. kube only runs against a reachable cluster.",
			ok:      false,
			cluster: cluster,
		}
	case "live":
		live.InvalidateLive()
		probe := host.opts.probeLive
		if probe == nil {
			probe = live.ProbeLive
		}
		if !probe() {
			return lineResult{
				output:  "No live cluster. Check kubectl and the current context.",
				ok:      false,
				cluster: cluster,
			}
		}
		next, ok := live.LiveCluster("kubectl")
		if !ok {
			next = cluster
		}
		return lineResult{output: "live cluster " + next.Current, ok: true, cluster: next}
	case "clear":
		return lineResult{output: "\x1bc", ok: true, cluster: cluster}
	case "help", "--help", "-h":
		return lineResult{output: host.help, ok: true, cluster: cluster}
	case "--version", "-v":
		return lineResult{output: host.version, ok: true, cluster: cluster}
	case "completion":
		shell := ""
		if parts := strings.Fields(trimmed); len(parts) > 1 {
			shell = parts[1]
		}
		script := kube.CompletionScript(shell)
		if script == "" {
			return lineResult{output: "usage: kube completion bash|zsh|fish|powershell", ok: false, cluster: cluster}
		}
		return lineResult{output: script, ok: true, cluster: cluster}
	case "undo":
		return undoLine(trimmed, cluster, host)
	case "audit":
		return auditLine(trimmed, cluster)
	case "record":
		return recordLine(trimmed, cluster)
	case "ns", "ctx":
		name := strings.TrimSpace(strings.TrimPrefix(trimmed, meta))
		name = strings.TrimSpace(name)
		if name == "" {
			return lineResult{output: "usage: " + meta + " <name>", ok: false, cluster: cluster}
		}
		if cluster.Current == "" {
			return lineResult{output: "no current context", ok: false, cluster: cluster}
		}
		if meta == "ns" {
			return lineResult{output: "namespace " + name, ok: true, cluster: kube.UseNamespace(cluster, scope, name)}
		}
		next, ok := kube.UseContext(cluster, scope, name)
		if !ok {
			return lineResult{output: "no context " + name, ok: false, cluster: cluster}
		}
		return lineResult{output: "context " + name, ok: true, cluster: next}
	case "view", "ui":
		parts := strings.Fields(trimmed)[1:]
		all := false
		word := ""
		for _, part := range parts {
			if part == "-A" || part == "--all-namespaces" || part == "all" {
				all = true
				continue
			}
			if !strings.HasPrefix(part, "-") && word == "" {
				word = part
			}
		}
		rule := kube.RuleFor(host.policy, cluster.Current)
		forcePlain := host.opts.forcePlain
		stdoutFile, _ := host.stdout.(*os.File)
		if stdoutFile == nil {
			forcePlain = true
		}
		next := view.OpenView(cluster, view.ViewOptions{
			Board:        view.BoardFrom(word),
			All:          all,
			Yes:          host.yes,
			TypedConfirm: rule.Confirm == "typed",
			ForcePlain:   forcePlain,
			Stdout:       host.stdout,
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
		return lineResult{ok: true, cluster: next}
	case "plan":
		return planLine(trimmed, cluster, scope)
	case "ask":
		asJSON := strings.Contains(" "+trimmed+" ", " --json ")
		question := trimmed
		question = strings.TrimPrefix(question, "?")
		question = strings.TrimSpace(question)
		if strings.HasPrefix(strings.ToLower(question), "ask ") {
			question = strings.TrimSpace(question[4:])
		}
		question = strings.ReplaceAll(question, "--json", "")
		question = strings.Trim(strings.TrimSpace(question), `"'`)
		advice := kplan.AskAdvice(question, cluster, kube.ContextArgs(scope))
		if asJSON {
			encoded, err := json.Marshal(advice)
			if err != nil {
				return lineResult{output: err.Error(), ok: false, cluster: cluster}
			}
			return lineResult{output: string(encoded), ok: true, cluster: cluster}
		}
		if !host.stdinTTY {
			return lineResult{output: kplan.FormatAdvice(advice), ok: true, cluster: cluster}
		}
		fmt.Fprintln(host.stderr, kplan.FormatAdvice(advice))
		confirm := host.confirm
		if confirm == nil {
			confirm = defaultConfirm(host.stderr)
		}
		if !confirm("Run the first command? [y/N] ") {
			return lineResult{ok: true, cluster: cluster}
		}
		if len(advice.Commands) == 0 {
			return lineResult{ok: true, cluster: cluster}
		}
		trimmed = advice.Commands[0]
	}

	view := kube.ToEngine(trimmed, len(trimmed), "repl")
	return executeEngine(view.Line, cluster, scope, host)
}

func undoLine(trimmed string, cluster kube.Cluster, host lineHost) lineResult {
	parts := strings.Fields(trimmed)
	if len(parts) > 1 && parts[1] == "list" {
		lines := make([]string, 0)
		for _, record := range undo.ListUndo(time.Now()) {
			state := "restorable"
			if record.Refused != "" {
				state = "not restorable"
			}
			lines = append(lines, fmt.Sprintf("%s  %s/%s  %s  %s", record.ID, record.Kind, record.Name, record.Namespace, state))
		}
		if len(lines) == 0 {
			return lineResult{output: "nothing to undo", ok: true, cluster: cluster}
		}
		return lineResult{output: strings.Join(lines, "\n"), ok: true, cluster: cluster}
	}
	record, ok := undo.LatestUndo(time.Now())
	if !ok {
		return lineResult{output: "nothing to undo", ok: false, cluster: cluster}
	}
	if refusal := undo.UndoRefusal(record); refusal != "" {
		return lineResult{output: refusal, ok: false, cluster: cluster}
	}
	preview := "apply saved " + record.Kind + "/" + record.Name
	if record.Revision != "" {
		preview = "rollout undo " + record.Kind + "/" + record.Name + " --to-revision=" + record.Revision
	}
	if !host.yes {
		confirm := host.confirm
		if confirm == nil {
			confirm = defaultConfirm(host.stderr)
		}
		if !confirm("Undo " + preview + "? [y/N] ") {
			return lineResult{output: "not run", ok: false, cluster: cluster}
		}
	}
	runner := host.opts.runLive
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
		return lineResult{output: fmt.Sprintf("restored %s/%s", record.Kind, record.Name), ok: true, cluster: cluster}
	}
	return lineResult{output: "undo failed", ok: false, cluster: cluster}
}

func recordLine(trimmed string, cluster kube.Cluster) lineResult {
	parts := strings.Fields(trimmed)
	action := "on"
	if len(parts) > 1 {
		action = parts[1]
	}
	if action == "off" {
		file := kube.StopRecord()
		if file == "" {
			return lineResult{output: "not recording", ok: true, cluster: cluster}
		}
		return lineResult{output: "recording stopped " + file, ok: true, cluster: cluster}
	}
	if action != "on" {
		return lineResult{output: "usage: record on|off", ok: false, cluster: cluster}
	}
	file := ""
	if len(parts) > 2 {
		file = parts[2]
	}
	if file == "" {
		ctx := cluster.Current
		if ctx == "" {
			ctx = "none"
		}
		file = kube.SessionPath(ctx, time.Now())
	}
	ns := kube.NamespaceOf(cluster)
	if ns == "" {
		ns = "default"
	}
	ctx := cluster.Current
	if ctx == "" {
		ctx = "none"
	}
	started := kube.StartRecord(file, ctx, ns, time.Now().UTC().Format(time.RFC3339Nano))
	return lineResult{output: "recording " + started, ok: true, cluster: cluster}
}

func auditLine(trimmed string, cluster kube.Cluster) lineResult {
	parts := strings.Fields(trimmed)
	if len(parts) > 1 && parts[1] != "tail" {
		return lineResult{output: "usage: kube audit tail [-n 20]", ok: false, cluster: cluster}
	}
	count := 20
	for i := 0; i < len(parts); i++ {
		if parts[i] == "-n" && i+1 < len(parts) {
			if n, err := strconv.Atoi(parts[i+1]); err == nil {
				count = n
			}
		}
	}
	return lineResult{output: strings.Join(kube.TailAudit(count), "\n"), ok: true, cluster: cluster}
}

func planLine(trimmed string, cluster kube.Cluster, scope *kube.ShellScope) lineResult {
	asJSON := strings.Contains(" "+trimmed+" ", " --json ")
	body := trimmed
	if strings.HasPrefix(body, "plan") {
		body = strings.TrimSpace(body[4:])
	}
	body = strings.ReplaceAll(body, "--json", "")
	body = strings.TrimSpace(body)
	if body == "" {
		return lineResult{output: "usage: kube plan [--json] delete pod <name>", ok: false, cluster: cluster}
	}
	view := kube.ToEngine(body, len(body), "repl")
	planned := kplan.PlanCommand(view.Line, cluster)
	argv := kube.CommandArgs(view.Line, scope)
	var blast *kplan.Blast
	if wantsBlast(planned) {
		report := liveBlast(planned, argv)
		blast = &report
	}
	if !asJSON {
		text := kplan.FormatPlan(planned)
		if blast != nil {
			if extra := kplan.FormatBlast(*blast); extra != "" {
				text += "\n" + extra
			}
		}
		return lineResult{output: text, ok: true, cluster: cluster}
	}
	type payload struct {
		kube.Plan
		Blast  *kplan.Blast `json:"blast,omitempty"`
		PlanID string       `json:"planId,omitempty"`
	}
	out := payload{Plan: planned, Blast: blast}
	id, err := kube.MintPlan(kube.PlanExpect{Context: planned.Context, Namespace: planned.Namespace, Argv: argv}, time.Now())
	if err == nil {
		out.PlanID = id
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		return lineResult{output: err.Error(), ok: false, cluster: cluster}
	}
	return lineResult{output: string(encoded), ok: true, cluster: cluster}
}

func executeEngine(engineLine string, cluster kube.Cluster, scope *kube.ShellScope, host lineHost) lineResult {
	planned := kplan.PlanCommand(engineLine, cluster)
	started := time.Now()
	auditID := kube.NewAuditId()
	argv := kube.CommandArgs(engineLine, scope)
	scopeCtx := ""
	if scope != nil {
		scopeCtx = scope.Context
	}
	policyCtx := kube.EffectiveContext(cluster.Current, scopeCtx)
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
			Actor: actor, Context: planned.Context, Namespace: planned.Namespace,
			Argv: kube.RedactArgv(argv), Risk: planned.Risk, Decision: decision,
			Approver: approver, ExitCode: exitCode, DurationMs: int(time.Since(started).Milliseconds()),
		}, 0)
	}
	auth := kube.Authorize(kube.AuthRequest{
		Plan: planned, Argv: argv, Policy: host.policy, Agent: host.agent,
		Yes: host.yes, ConfirmContext: host.confirmContext, PlanID: host.planID,
		PolicyContext: policyCtx,
	}, func(planID string, expect kube.PlanExpect) (bool, string) {
		return kube.ConsumePlan(planID, expect, time.Now())
	})
	blastText := ""
	if !auth.OK {
		writeAudit(auth.AuditDecision, "none", nil)
		return lineResult{output: auth.Refuse, ok: false, cluster: cluster}
	}
	if auth.RequireConfirm {
		text := kplan.FormatPlan(planned)
		if wantsBlast(planned) {
			if extra := kplan.FormatBlast(liveBlast(planned, argv)); extra != "" {
				blastText = extra
				text += "\n" + extra
			}
		}
		fmt.Fprintln(host.stderr, text)
		confirm := host.confirm
		if confirm == nil {
			confirm = defaultConfirm(host.stderr)
		}
		if !confirm("Run it? [y/N] ") {
			writeAudit("refused-confirm", "none", nil)
			return lineResult{output: "not run", ok: false, cluster: cluster}
		}
	} else if warn := kube.SensitiveReadWarning(argv); warn != "" {
		fmt.Fprintln(host.stderr, warn)
	}
	approver := auth.Approver
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
	runner := host.opts.runLive
	if runner == nil {
		runner = live.RunLive
	}
	code := runner(argv, live.RunLiveOptions{InheritStdin: !host.stdinTTY})
	decision := "executed"
	if dryRun {
		decision = "dry-run"
	}
	writeAudit(decision, approver, &code)
	entry := kube.RunbookEntry{
		Decision: decision, Verb: planned.Verb,
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Context:   planned.Context, Namespace: planned.Namespace,
		Command: kube.RedactCommand(argv), Risk: planned.Risk, ExitCode: &code,
		Blast: blastText,
	}
	if planned.Mutating {
		entry.Summary = planned.Summary
	}
	kube.NoteRan(entry)
	return lineResult{ok: code == 0, cluster: live.ApplyLiveContext(cluster)}
}
