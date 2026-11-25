package kube

import (
	"os"
	"path/filepath"
	"strings"
)

const historyLimit = 200

func historyPath() string {
	return filepath.Join(StateDir(), "history")
}

// LoadHistory returns recent redacted history lines, newest first (readline order).
func LoadHistory() []string {
	raw, err := os.ReadFile(historyPath())
	if err != nil {
		return nil
	}
	var lines []string
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	if len(lines) > historyLimit {
		lines = lines[len(lines)-historyLimit:]
	}
	// newest first for liner/readline style APIs
	out := make([]string, len(lines))
	for i := range lines {
		out[i] = lines[len(lines)-1-i]
	}
	return out
}

func trimHistoryFile() {
	raw, err := os.ReadFile(historyPath())
	if err != nil {
		return
	}
	var lines []string
	for _, line := range strings.Split(string(raw), "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) <= historyLimit {
		return
	}
	_ = writePrivate(historyPath(), strings.Join(lines[len(lines)-historyLimit:], "\n")+"\n")
}

// RememberHistory appends a redacted line to ~/.kube-shell/history.
func RememberHistory(line string) {
	trimmed := RedactLine(strings.TrimSpace(line))
	if trimmed == "" {
		return
	}
	path := historyPath()
	if _, err := os.Stat(path); err != nil {
		_ = writePrivate(path, "")
	} else {
		_ = os.Chmod(path, 0o600)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	_, _ = f.WriteString(trimmed + "\n")
	_ = f.Close()
	trimHistoryFile()
}
