package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/NadafMM/kubectl-shell/internal/kube"
	"golang.org/x/term"
)

func bootCluster(opts runOptions, stderr io.Writer) (kube.Cluster, bool) {
	probe := opts.probeLive
	if probe == nil {
		probe = kube.ProbeLive
	}
	if !probe() {
		fmt.Fprintln(stderr, "kubectl could not reach a cluster. kube only runs against a reachable cluster.")
		return kube.Cluster{}, false
	}
	cluster, ok := kube.LiveCluster("kubectl")
	if !ok {
		fmt.Fprintln(stderr, "kubectl could not reach a cluster. kube only runs against a reachable cluster.")
		return kube.Cluster{}, false
	}
	return cluster, true
}

func makeLineHost(host session, stdout, stderr io.Writer, opts runOptions, scope *kube.ShellScope) lineHost {
	confirm := opts.confirm
	if confirm == nil {
		confirm = defaultConfirm(stderr)
	}
	return lineHost{
		session: host,
		help:    helpText(),
		version: version,
		confirm: confirm,
		stderr:  stderr,
		stdout:  stdout,
		opts:    opts,
		scope:   scope,
	}
}

func runScript(stdin io.Reader, host session, stdout, stderr io.Writer, opts runOptions) int {
	cluster, ok := bootCluster(opts, stderr)
	if !ok {
		return 1
	}
	scope := &kube.ShellScope{}
	lh := makeLineHost(host, stdout, stderr, opts, scope)
	code := 0
	scanner := bufio.NewScanner(stdin)
	// allow long pasted lines
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		kube.RememberHistory(line)
		result := runUserLine(line, cluster, lh)
		cluster = result.cluster
		if result.output != "\x1bc" {
			writeLineResult(result, stdout, stderr)
		}
		if !result.ok {
			code = 1
		}
		if result.exit {
			break
		}
	}
	return code
}

func promptString(cluster kube.Cluster, policy *kube.Policy) string {
	guarded := kube.RuleFor(policy, cluster.Current).Mode == "guarded"
	body := kube.PromptOf(cluster) + "$ "
	label := kube.PromptLabel(guarded, body)
	if guarded && strings.HasPrefix(label, "[GUARDED] ") {
		return "\x1b[31m[GUARDED]\x1b[0m" + label[len("[GUARDED]"):]
	}
	return label
}

func startRepl(host session, stdout, stderr io.Writer, opts runOptions) int {
	cluster, ok := bootCluster(opts, stderr)
	if !ok {
		return 1
	}
	scope := &kube.ShellScope{}
	lh := makeLineHost(host, stdout, stderr, opts, scope)

	fmt.Fprintf(stdout, "kube  live cluster %s. Tab completes real names. `help` lists commands.\n", kube.PromptOf(cluster))

	// Ignore SIGINT in the editor so Ctrl+C is a key (0x03). RunLive/CaptureLive
	// temporarily restore the default so child kubectl (logs -f, exec, …) can stop.
	kube.SetREPLInterruptIgnore(true)
	defer kube.SetREPLInterruptIgnore(false)

	history := kube.LoadHistory()
	state, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		// Fall back to line-at-a-time when raw mode is unavailable.
		return runScript(os.Stdin, host, stdout, stderr, opts)
	}
	defer func() { _ = term.Restore(int(os.Stdin.Fd()), state) }()

	editor := newLineEditor(os.Stdin, stdout, history)
	for {
		prompt := promptString(cluster, host.policy)
		line, status := editor.readLine(prompt)
		if status == lineEOF {
			fmt.Fprintln(stdout)
			return 0
		}
		if status == lineInterrupt {
			if editor.hasPartial() {
				editor.clearPartial()
				fmt.Fprintln(stdout)
				continue
			}
			fmt.Fprintln(stdout)
			return 0
		}
		kube.RememberHistory(line)
		// Leave raw mode while kubectl / confirms run so children see a normal TTY.
		_ = term.Restore(int(os.Stdin.Fd()), state)
		result := runUserLine(line, cluster, lh)
		kube.SetREPLInterruptIgnore(true)
		_, _ = term.MakeRaw(int(os.Stdin.Fd()))
		cluster = result.cluster
		if result.output == "\x1bc" {
			fmt.Fprint(stdout, "\x1b[H\x1b[2J")
		} else {
			writeLineResult(result, stdout, stderr)
		}
		if result.exit {
			fmt.Fprintln(stdout)
			return 0
		}
	}
}

const (
	lineOK = iota
	lineEOF
	lineInterrupt
)

type lineEditor struct {
	in      *os.File
	out     io.Writer
	history []string
	histIdx int
	buf     []rune
	cursor  int
}

func newLineEditor(in *os.File, out io.Writer, history []string) *lineEditor {
	return &lineEditor{in: in, out: out, history: append([]string{}, history...), histIdx: -1}
}

func (e *lineEditor) hasPartial() bool { return len(e.buf) > 0 }
func (e *lineEditor) clearPartial()    { e.buf = nil; e.cursor = 0; e.histIdx = -1 }

func (e *lineEditor) readLine(prompt string) (string, int) {
	e.buf = nil
	e.cursor = 0
	e.histIdx = -1
	fmt.Fprint(e.out, prompt)
	for {
		var b [8]byte
		n, err := e.in.Read(b[:])
		if err != nil || n <= 0 {
			return "", lineEOF
		}
		raw := b[:n]
		i := 0
		for i < len(raw) {
			ch := raw[i]
			switch {
			case ch == 0x03: // Ctrl+C
				return "", lineInterrupt
			case ch == 0x04: // Ctrl+D
				if len(e.buf) == 0 {
					return "", lineEOF
				}
				i++
			case ch == 0x7f || ch == 0x08: // backspace
				if e.cursor > 0 {
					e.buf = append(e.buf[:e.cursor-1], e.buf[e.cursor:]...)
					e.cursor--
					e.redraw(prompt)
				}
				i++
			case ch == '\t':
				e.complete(prompt)
				i++
			case ch == '\r' || ch == '\n':
				fmt.Fprint(e.out, "\r\n")
				return string(e.buf), lineOK
			case ch == 0x1b && i+2 < len(raw) && raw[i+1] == '[':
				switch raw[i+2] {
				case 'A': // up
					e.historyUp(prompt)
				case 'B': // down
					e.historyDown(prompt)
				case 'C': // right
					if e.cursor < len(e.buf) {
						e.cursor++
						e.redraw(prompt)
					}
				case 'D': // left
					if e.cursor > 0 {
						e.cursor--
						e.redraw(prompt)
					}
				}
				i += 3
			case ch >= 0x20 && ch != 0x7f:
				r := rune(ch)
				e.buf = append(e.buf[:e.cursor], append([]rune{r}, e.buf[e.cursor:]...)...)
				e.cursor++
				e.redraw(prompt)
				i++
			default:
				i++
			}
		}
	}
}

func (e *lineEditor) redraw(prompt string) {
	fmt.Fprintf(e.out, "\r\x1b[2K%s%s", prompt, string(e.buf))
	if e.cursor < len(e.buf) {
		fmt.Fprintf(e.out, "\x1b[%dD", len(e.buf)-e.cursor)
	}
}

func (e *lineEditor) historyUp(prompt string) {
	if len(e.history) == 0 {
		return
	}
	if e.histIdx < 0 {
		e.histIdx = 0
	} else if e.histIdx+1 < len(e.history) {
		e.histIdx++
	}
	e.buf = []rune(e.history[e.histIdx])
	e.cursor = len(e.buf)
	e.redraw(prompt)
}

func (e *lineEditor) historyDown(prompt string) {
	if e.histIdx < 0 {
		return
	}
	if e.histIdx == 0 {
		e.histIdx = -1
		e.buf = nil
		e.cursor = 0
		e.redraw(prompt)
		return
	}
	e.histIdx--
	e.buf = []rune(e.history[e.histIdx])
	e.cursor = len(e.buf)
	e.redraw(prompt)
}

func (e *lineEditor) complete(prompt string) {
	line := string(e.buf)
	cursor := len([]byte(string(e.buf[:e.cursor])))
	view := kube.ToEngine(line, cursor, "repl")
	cluster, ok := kube.LiveCluster(view.Line)
	if !ok {
		cluster = kube.Cluster{}
	}
	found := kube.Complete(view.Line, view.Cursor, cluster)
	start := found.Start - view.Delta
	end := found.End - view.Delta
	if start < 0 {
		start = 0
	}
	if end < 0 {
		end = 0
	}
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
	if len(values) == 0 {
		fmt.Fprint(e.out, "\x07")
		return
	}
	top := values[0]
	nextLine, nextCursor := kube.SpliceToken(line, start, end, top.Value, true)
	if nextLine == line && nextCursor == cursor {
		if len(values) > 1 {
			width := 0
			for _, item := range values {
				if len(item.Value) > width {
					width = len(item.Value)
				}
			}
			fmt.Fprint(e.out, "\r\n")
			for _, item := range values {
				fmt.Fprintf(e.out, "  %-*s  %s\r\n", width, item.Value, item.Detail)
			}
			e.redraw(prompt)
			return
		}
		fmt.Fprint(e.out, "\x07")
		return
	}
	e.buf = []rune(nextLine)
	// map byte cursor to rune cursor approximately by rebuilding
	e.cursor = len([]rune(nextLine[:min(nextCursor, len(nextLine))]))
	if e.cursor > len(e.buf) {
		e.cursor = len(e.buf)
	}
	e.redraw(prompt)
}
