package redact

import (
	"regexp"
	"strings"
)

var pairFlags = []string{"--from-literal", "--from-file"}
var valueFlagsRedact = []string{"--docker-password", "--password", "--token", "--client-secret"}
var sensitiveKey = regexp.MustCompile(`(?i)^(password|token|secret|apikey|api-key|client-secret|client_secret)$`)

func hidePair(value string) string {
	eq := strings.Index(value, "=")
	if eq < 0 {
		return "***"
	}
	return value[:eq] + "=***"
}

func matchFlag(token string, flags []string) string {
	for _, flag := range flags {
		if token == flag || strings.HasPrefix(token, flag+"=") {
			return flag
		}
	}
	return ""
}

func redactAssignment(token string) string {
	eq := strings.Index(token, "=")
	if eq < 0 {
		return token
	}
	key := token[:eq]
	if sensitiveKey.MatchString(key) {
		return key + "=***"
	}
	return token
}

// RedactLine hides secret-bearing flag values in a shell line.
func RedactLine(line string) string {
	tokens := strings.Split(line, " ")
	var out []string
	for i := 0; i < len(tokens); i++ {
		token := tokens[i]
		if pair := matchFlag(token, pairFlags); pair != "" {
			if token == pair {
				out = append(out, token)
				if i+1 < len(tokens) {
					out = append(out, hidePair(tokens[i+1]))
					i++
				}
			} else {
				out = append(out, pair+"="+hidePair(token[len(pair)+1:]))
			}
			continue
		}
		if valueFlag := matchFlag(token, valueFlagsRedact); valueFlag != "" {
			if token == valueFlag {
				out = append(out, token)
				if i+1 < len(tokens) {
					out = append(out, "***")
					i++
				}
			} else {
				out = append(out, valueFlag+"=***")
			}
			continue
		}
		out = append(out, redactAssignment(token))
	}
	return strings.Join(out, " ")
}

// RedactArgv hides secret-bearing values in argv for the audit log.
// Tokens are redacted in place so values that contain spaces stay one element.
func RedactArgv(argv []string) []string {
	out := make([]string, 0, len(argv))
	for i := 0; i < len(argv); i++ {
		token := argv[i]
		if pair := matchFlag(token, pairFlags); pair != "" {
			if token == pair {
				out = append(out, token)
				if i+1 < len(argv) {
					out = append(out, hidePair(argv[i+1]))
					i++
				}
			} else {
				out = append(out, pair+"="+hidePair(token[len(pair)+1:]))
			}
			continue
		}
		if valueFlag := matchFlag(token, valueFlagsRedact); valueFlag != "" {
			if token == valueFlag {
				out = append(out, token)
				if i+1 < len(argv) {
					out = append(out, "***")
					i++
				}
			} else {
				out = append(out, valueFlag+"=***")
			}
			continue
		}
		out = append(out, redactAssignment(token))
	}
	return out
}
