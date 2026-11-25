package kube

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// RunbookEntry is one command that ran (or dry-ran) for the session file.
type RunbookEntry struct {
	Decision  string
	Verb      string
	Timestamp string
	Context   string
	Namespace string
	Command   string
	Risk      string
	ExitCode  *int
	Summary   string
	Blast     string
}

var (
	recordMu sync.Mutex
	active   string
)

func verbNames() map[string]bool {
	out := make(map[string]bool, len(Verbs))
	for _, item := range Verbs {
		out[item.Value] = true
	}
	return out
}

// SessionPath is the default Markdown runbook path under ~/.kube-shell/sessions/.
func SessionPath(context string, now time.Time) string {
	iso := now.UTC().Format("2006-01-02T15:04:05.000") + "Z"
	stamp := strings.Map(func(r rune) rune {
		if r == ':' || r == '.' {
			return '-'
		}
		return r
	}, iso)
	safe := strings.Trim(sanitizeContext(context), "-")
	if safe == "" {
		safe = "none"
	}
	return filepath.Join(StateDir(), "sessions", stamp+"-"+safe+".md")
}

func sanitizeContext(context string) string {
	var b strings.Builder
	for _, r := range context {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return b.String()
}

// RedactCommand formats kubectl argv with secrets masked.
func RedactCommand(argv []string) string {
	return "kubectl " + strings.Join(RedactArgv(argv), " ")
}

// ShouldRecord is true for executed/dry-run kubectl verbs only.
func ShouldRecord(decision, verb string) bool {
	if decision != "executed" && decision != "dry-run" {
		return false
	}
	return verbNames()[verb]
}

// Recording returns the active session file, if any.
func Recording() string {
	recordMu.Lock()
	defer recordMu.Unlock()
	return active
}

// StartRecord begins appending to file (creates a header when missing).
func StartRecord(file string, context, namespace, started string) string {
	recordMu.Lock()
	defer recordMu.Unlock()
	if active != "" {
		return active
	}
	active = file
	if _, err := os.Stat(file); err != nil {
		_ = WritePrivate(file, "# kube session\n\n- context: "+context+"\n- namespace: "+namespace+"\n- started: "+started+"\n")
	}
	return file
}

// StopRecord clears the active session and returns the previous path.
func StopRecord() string {
	recordMu.Lock()
	defer recordMu.Unlock()
	file := active
	active = ""
	return file
}

// NoteRan appends an entry when recording and ShouldRecord.
func NoteRan(entry RunbookEntry) {
	recordMu.Lock()
	file := active
	recordMu.Unlock()
	if file == "" || !ShouldRecord(entry.Decision, entry.Verb) {
		return
	}
	exit := "null"
	if entry.ExitCode != nil {
		exit = strconv.Itoa(*entry.ExitCode)
	}
	lines := []string{
		"",
		"## " + entry.Command,
		"",
		"- at: " + entry.Timestamp,
		"- context: " + entry.Context,
		"- namespace: " + entry.Namespace,
		"- exit: " + exit,
		"",
	}
	if entry.Summary != "" {
		lines = append(lines, entry.Summary, "")
	}
	if entry.Blast != "" {
		lines = append(lines, entry.Blast, "")
	}
	previous, err := os.ReadFile(file)
	if err != nil {
		previous = nil
	}
	_ = WritePrivate(file, string(previous)+strings.Join(lines, "\n"))
}
