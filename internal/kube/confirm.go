package kube

import (
	"bufio"
	"io"
	"strings"
)

// AllowYes is true when --yes may skip the confirm prompt.
// An empty interactive shell must not inherit a prior --yes.
func AllowYes(yes bool, argc int, tty bool) bool {
	if !yes {
		return false
	}
	if argc == 0 && tty {
		return false
	}
	return true
}

// ReadAnswer is true when the first non-space character of the answer line is y or Y.
// Remaining bytes after the newline stay in chunks for the next read.
func ReadAnswer(chunks *[]string) bool {
	answer := ""
	for len(*chunks) > 0 {
		next := (*chunks)[0]
		*chunks = (*chunks)[1:]
		newline := strings.IndexAny(next, "\r\n")
		if newline >= 0 {
			answer += next[:newline]
			rest := next[newline+1:]
			if rest != "" {
				*chunks = append([]string{rest}, *chunks...)
			}
			break
		}
		answer += next
	}
	trimmed := strings.TrimLeft(answer, " \t")
	if trimmed == "" {
		return false
	}
	first := trimmed[0]
	return first == 'y' || first == 'Y'
}

// ConfirmReader reads one answer line from r. A read error is a refusal.
func ConfirmReader(r io.Reader, warn func(string)) bool {
	chunks := []string{}
	buf := make([]byte, 1)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			text := string(buf[:n])
			chunks = append(chunks, text)
			if text == "\n" || text == "\r" {
				break
			}
		}
		if err != nil {
			if len(chunks) == 0 {
				if warn != nil {
					msg := err.Error()
					warn("\nkube: could not read the answer (" + msg + "); not running it.\n")
				}
				return false
			}
			break
		}
		if n == 0 {
			break
		}
	}
	return ReadAnswer(&chunks)
}

// ConfirmLine writes prompt to w and reads one y/N answer from r.
func ConfirmLine(w io.Writer, r io.Reader, prompt string) bool {
	_, _ = io.WriteString(w, prompt)
	return ConfirmReader(r, func(text string) {
		_, _ = io.WriteString(w, text)
	})
}

// ScanConfirm is a simpler line reader used when a full line is available.
func ScanConfirm(r io.Reader) bool {
	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		return false
	}
	line := strings.TrimLeft(scanner.Text(), " \t")
	return len(line) > 0 && (line[0] == 'y' || line[0] == 'Y')
}
