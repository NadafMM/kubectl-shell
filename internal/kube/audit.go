package kube

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const auditMaxBytes = 5 * 1024 * 1024

type AuditEntry struct {
	ID         string   `json:"id"`
	Timestamp  string   `json:"timestamp"`
	Actor      string   `json:"actor"`
	Context    string   `json:"context"`
	Namespace  string   `json:"namespace"`
	Argv       []string `json:"argv"`
	Risk       string   `json:"risk"`
	Decision   string   `json:"decision"`
	Approver   string   `json:"approver"`
	ExitCode   *int     `json:"exitCode"`
	DurationMs int      `json:"durationMs"`
}

func auditPath() string {
	return filepath.Join(StateDir(), "audit.jsonl")
}

// AuditEnabled is true when policy requires it or KUBE_SHELL_AUDIT is not "0".
func AuditEnabled(policy *Policy) bool {
	if policy != nil && policy.Audit == "required" {
		return true
	}
	return os.Getenv("KUBE_SHELL_AUDIT") != "0"
}

// RedactArgv hides secret-bearing values in argv for the audit log.
func RedactArgv(argv []string) []string {
	return strings.Split(RedactLine(strings.Join(argv, " ")), " ")
}

// AppendAudit writes one JSONL audit row, rotating when the file grows too large.
func AppendAudit(entry AuditEntry, maxBytes int) {
	if maxBytes <= 0 {
		maxBytes = auditMaxBytes
	}
	file := auditPath()
	raw, err := json.Marshal(entry)
	if err != nil {
		return
	}
	line := string(raw) + "\n"
	if info, err := os.Stat(file); err == nil && info.Size() >= int64(maxBytes) {
		previous := file + ".1"
		_ = os.Remove(previous)
		_ = os.Rename(file, previous)
	}
	if _, err := os.Stat(file); err != nil {
		_ = writePrivate(file, line)
	} else {
		f, err := os.OpenFile(file, os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			return
		}
		_, _ = f.WriteString(line)
		_ = f.Close()
		_ = os.Chmod(file, 0o600)
	}
	if os.Getenv("KUBE_SHELL_AUDIT_STDERR") == "1" {
		_, _ = os.Stderr.WriteString(line)
	}
}

// TailAudit returns the last count lines of the audit log.
func TailAudit(count int) []string {
	raw, err := os.ReadFile(auditPath())
	if err != nil {
		return nil
	}
	var lines []string
	for _, line := range strings.Split(string(raw), "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	if count <= 0 {
		count = 20
	}
	if len(lines) > count {
		return lines[len(lines)-count:]
	}
	return lines
}

// NewAuditId returns a 16-character hex id.
func NewAuditId() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return strconv.FormatInt(int64(os.Getpid()), 16)
	}
	return hex.EncodeToString(buf)
}
