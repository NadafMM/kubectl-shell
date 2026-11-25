package kube

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"golang.org/x/term"
)

// ViewOptions configures openView.
type ViewOptions struct {
	Board        Board
	All          bool
	Yes          bool
	TypedConfirm bool
	Reload       func(all bool, ns string) (Cluster, bool)
	Stdout       io.Writer
	Stdin        *os.File
	StdoutFile   *os.File
	ForcePlain   bool
	Policy       *Policy
}

func lookupText(verb string, row Row) string {
	var args []string
	if verb == "logs" {
		args = []string{"logs", row.Name, "--tail=80"}
	} else {
		args = []string{"describe", row.Kind, row.Name}
	}
	if row.NS != "" {
		args = append(args, "-n", row.NS)
	}
	_, stdout, stderr := CaptureLive(args)
	text := stdout
	if strings.TrimSpace(text) == "" {
		text = stderr
	}
	if strings.TrimSpace(text) == "" {
		text = "(no output)"
	}
	return strings.TrimRight(text, " \t\r\n")
}

func runViewDelete(plan Plan, yes bool, policy *Policy, actor string) string {
	started := time.Now()
	argv := KubectlArgs(plan.Command)
	code, stdout, stderr := CaptureLive(argv)
	if AuditEnabled(policy) {
		exit := code
		approver := "prompt-y"
		if yes {
			approver = "flag-yes"
		}
		AppendAudit(AuditEntry{
			ID:         NewAuditId(),
			Timestamp:  time.Now().UTC().Format(time.RFC3339),
			Actor:      actor,
			Context:    plan.Context,
			Namespace:  plan.Namespace,
			Argv:       RedactArgv(argv),
			Risk:       plan.Risk,
			Decision:   "executed",
			Approver:   approver,
			ExitCode:   &exit,
			DurationMs: int(time.Since(started).Milliseconds()),
		}, 0)
	}
	msg := stderr
	if strings.TrimSpace(msg) == "" {
		msg = stdout
	}
	if strings.TrimSpace(msg) == "" {
		msg = "deleted"
	}
	line := strings.Split(msg, "\n")[0]
	return strings.TrimSpace(line)
}

func readViewKeys(in *os.File) []string {
	buf := make([]byte, 64)
	n, err := in.Read(buf)
	if err != nil || n <= 0 {
		return []string{"q"}
	}
	raw := string(buf[:n])
	var keys []string
	for i := 0; i < len(raw); i++ {
		if raw[i] != 0x1b {
			keys = append(keys, string(raw[i]))
			continue
		}
		rest := raw[i:]
		switch {
		case strings.HasPrefix(rest, "\x1b[A"):
			keys = append(keys, "up")
			i += 2
		case strings.HasPrefix(rest, "\x1b[B"):
			keys = append(keys, "down")
			i += 2
		case strings.HasPrefix(rest, "\x1b[5~"):
			keys = append(keys, "pgup")
			i += 3
		case strings.HasPrefix(rest, "\x1b[6~"):
			keys = append(keys, "pgdn")
			i += 3
		case strings.HasPrefix(rest, "\x1b[3~"):
			keys = append(keys, "del")
			i += 3
		default:
			keys = append(keys, "esc")
		}
	}
	out := keys[:0]
	for _, key := range keys {
		if key != "" {
			out = append(out, key)
		}
	}
	return out
}

func openTTY(stdin *os.File) (*os.File, func()) {
	if stdin == nil {
		stdin = os.Stdin
	}
	f, err := os.OpenFile("/dev/tty", os.O_RDONLY, 0)
	if err != nil {
		return stdin, func() {}
	}
	return f, func() { _ = f.Close() }
}

// OpenView shows the resource table. Without a TTY it prints TableText and returns.
func OpenView(clusterIn Cluster, options ViewOptions) Cluster {
	stdout := options.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stdoutFile := options.StdoutFile
	if stdoutFile == nil {
		stdoutFile = os.Stdout
	}
	stdin := options.Stdin
	if stdin == nil {
		stdin = os.Stdin
	}

	plain := options.ForcePlain || !term.IsTerminal(int(stdin.Fd())) || !term.IsTerminal(int(stdoutFile.Fd()))
	if plain {
		fmt.Fprintln(stdout, TableText(clusterIn, options.Board, options.All))
		return clusterIn
	}

	cluster := clusterIn
	var text []string
	var pendingDelete *Plan
	nsCursor := 0
	currentNS := NamespaceOf(cluster)
	for i, item := range cluster.Namespaces {
		if item.Name == currentNS {
			nsCursor = i
			break
		}
	}
	state := NewViewState(options.Board, options.All, currentNS, options.Yes)
	state.NSCursor = nsCursor

	oldState, err := term.MakeRaw(int(stdin.Fd()))
	if err != nil {
		fmt.Fprintln(stdout, TableText(clusterIn, options.Board, options.All))
		return clusterIn
	}
	defer func() { _ = term.Restore(int(stdin.Fd()), oldState) }()

	ttyIn, closeTTY := openTTY(stdin)
	defer closeTTY()

	fmt.Fprint(stdoutFile, "\x1b[?1049h\x1b[?25l")
	defer fmt.Fprint(stdoutFile, "\x1b[?25h\x1b[?1049l")

	rows := func() []Row {
		return RowsOf(cluster, state.Board, state.NS, state.All, state.Filter)
	}
	namespaces := func() []string {
		names := make([]string, 0, len(cluster.Namespaces))
		for _, item := range cluster.Namespaces {
			names = append(names, item.Name)
		}
		return names
	}
	reload := func() {
		if options.Reload == nil {
			state.Message = "refresh failed"
			return
		}
		next, ok := options.Reload(state.All, state.NS)
		if ok {
			cluster = next
			state.Message = "refreshed"
			return
		}
		state.Message = "refresh failed"
	}
	draw := func() {
		width, height, err := term.GetSize(int(stdoutFile.Fd()))
		if err != nil || width < 40 {
			width = 80
		}
		if height < 12 {
			height = 24
		}
		list := rows()
		cursor := state.Cursor
		if cursor >= len(list) {
			cursor = max(0, len(list)-1)
		}
		nsCur := state.NSCursor
		var lines []string
		label := boardLabelOf(state.Board)
		scope := state.NS
		if state.All {
			scope = "all namespaces"
		}
		lines = append(lines, paint(fit(fmt.Sprintf(" kube  %s  %s  %s  %d", cluster.Current, scope, label, len(list)), width), "head"))
		switch state.Mode {
		case ViewModeHelp:
			for _, line := range []string{
				"j/k or arrows   move",
				"enter           describe",
				"l               logs, pods only",
				"d               delete, then y",
				"/               filter by name",
				"1-5             pods, deploy, svc, nodes, ns",
				"a               all namespaces",
				"n               pick a namespace",
				"r               refresh",
				":pods :deploy   same as the number keys",
				"q               back to the shell",
			} {
				lines = append(lines, fit(line, width))
			}
		case ViewModeText:
			end := min(len(text), state.Scroll+height-3)
			for _, line := range text[state.Scroll:end] {
				lines = append(lines, fit(line, width))
			}
		case ViewModeNS:
			names := namespaces()
			if nsCur >= len(names) {
				nsCur = max(0, len(names)-1)
			}
			room := height - 3
			start := max(0, min(nsCur-2, max(0, len(names)-room)))
			for i := start; i < min(len(names), start+room); i++ {
				name := names[i]
				suffix := ""
				if name == state.NS {
					suffix = "  current"
				}
				tone := "plain"
				if i == nsCur {
					tone = "selected"
				}
				lines = append(lines, paint(fit("  "+name+suffix, width), tone))
			}
		default:
			showNS := state.All && state.Board != BoardNode && state.Board != BoardNamespace
			nameW := max(16, min(36, width-48))
			if !showNS {
				nameW = max(16, min(36, width-32))
			}
			header := ""
			if showNS {
				header += fit("NAMESPACE", 16)
			}
			header += fit("NAME", nameW) + "  " + fit("INFO", 16) + "  STATUS"
			lines = append(lines, paint(fit(header, width), "head"))
			room := height - 4
			start := max(0, min(cursor-room/2, max(0, len(list)-room)))
			for i := start; i < min(len(list), start+room); i++ {
				row := list[i]
				body := ""
				if showNS {
					body += fit(row.NS, 16)
				}
				body += fit(row.Name, nameW) + "  " + fit(row.Info, 16) + "  " + row.Status + "  " + row.Age
				tone := toneOf(row.Status)
				if i == cursor {
					tone = "selected"
				}
				lines = append(lines, paint(fit(body, width), tone))
			}
		}
		for len(lines) < height-1 {
			lines = append(lines, fit("", width))
		}
		hint := ""
		switch {
		case state.Commanding:
			hint = ":" + state.Command
		case state.Filtering:
			hint = "/" + state.Filter
		case state.Mode == ViewModeConfirm:
			hint = state.Message
		case state.Message != "":
			hint = state.Message
		default:
			hint = "1 pods  2 deploy  3 svc  4 nodes  5 ns    enter describe   l logs   / filter   d delete   ? help   q quit"
		}
		lines = append(lines, paint(fit(hint, width), "head"))
		state.Cursor = cursor
		state.NSCursor = nsCur
		if len(lines) > height {
			lines = lines[:height]
		}
		fmt.Fprintf(stdoutFile, "\x1b[H\x1b[2J%s", strings.Join(lines, "\n"))
	}

	actor := "human-tty"
	draw()
	for {
		for _, key := range readViewKeys(ttyIn) {
			list := rows()
			names := namespaces()
			next, effect := Reduce(state, key, list, names)
			state = next
			switch effect.Type {
			case "quit":
				return cluster
			case "reload":
				reload()
			case "describe", "logs":
				if effect.Index >= 0 && effect.Index < len(list) {
					row := list[effect.Index]
					verb := "describe"
					if effect.Type == "logs" {
						verb = "logs"
					}
					text = strings.Split(lookupText(verb, row), "\n")
					state.TextLength = len(text)
					state.Scroll = 0
					state.Mode = ViewModeText
				}
			case "arm-delete", "delete":
				if options.TypedConfirm {
					state.Message = "typed context: run kube delete from the shell"
					continue
				}
				if effect.Type == "arm-delete" {
					if effect.Index >= 0 && effect.Index < len(list) {
						planned := PlanCommand(DeleteEngineLine(list[effect.Index]), cluster)
						pendingDelete = &planned
						state.Message = planned.Risk + " risk: " + planned.Summary + "  [y/N]"
					}
				}
				if effect.Type == "delete" {
					var planned *Plan
					if pendingDelete != nil {
						planned = pendingDelete
					} else if effect.Index >= 0 && effect.Index < len(list) {
						p := PlanCommand(DeleteEngineLine(list[effect.Index]), cluster)
						planned = &p
					}
					pendingDelete = nil
					if planned != nil {
						state.Message = runViewDelete(*planned, options.Yes, options.Policy, actor)
						reload()
					}
				}
			}
		}
		draw()
	}
}
