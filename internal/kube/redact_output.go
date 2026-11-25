package kube

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
)

// SensitiveStdoutKind reports whether kubectl may print Secret/ConfigMap/kubeconfig payloads.
func SensitiveStdoutKind(tokens []string) string {
	return sensitiveStdoutKind(tokens)
}

// WantsRedactedStdout is true when KUBE_SHELL_REDACT_STDOUT=1 and the argv is sensitive.
func WantsRedactedStdout(args []string) bool {
	if os.Getenv("KUBE_SHELL_REDACT_STDOUT") != "1" {
		return false
	}
	return SensitiveStdoutKind(args) != ""
}

func redactJSONMap(block map[string]any) map[string]any {
	out := map[string]any{}
	for key := range block {
		out[key] = "***"
	}
	return out
}

func redactJSONValue(value any) any {
	switch typed := value.(type) {
	case []any:
		out := make([]any, len(typed))
		for i, child := range typed {
			out[i] = redactJSONValue(child)
		}
		return out
	case map[string]any:
		out := map[string]any{}
		changed := false
		for key, child := range typed {
			if key == "data" || key == "stringData" || key == "binaryData" {
				if nested, ok := child.(map[string]any); ok {
					out[key] = redactJSONMap(nested)
					changed = true
					continue
				}
			}
			if key == "token" || key == "client-certificate-data" || key == "client-key-data" || key == "password" {
				out[key] = "***"
				changed = true
				continue
			}
			next := redactJSONValue(child)
			out[key] = next
			if next != child {
				changed = true
			}
		}
		if changed {
			return out
		}
		return value
	default:
		return value
	}
}

func redactJSON(stdout string) string {
	var parsed any
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		return stdout
	}
	redacted := redactJSONValue(parsed)
	raw, err := json.MarshalIndent(redacted, "", "  ")
	if err != nil {
		return stdout
	}
	return string(raw) + "\n"
}

var (
	sensitiveYAMLKeys = regexp.MustCompile(`^(data|stringData|token|client-certificate-data|client-key-data|password|client-secret)$`)
	yamlKeyLine       = regexp.MustCompile(`^([a-zA-Z0-9_.-]+):\s*(.*)$`)
)

func redactYAML(stdout string) string {
	lines := strings.Split(stdout, "\n")
	out := make([]string, 0, len(lines))
	inData := false
	dataIndent := 0
	for _, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		indent := len(line) - len(trimmed)
		keyMatch := yamlKeyLine.FindStringSubmatch(trimmed)
		if len(keyMatch) == 3 && sensitiveYAMLKeys.MatchString(keyMatch[1]) && keyMatch[2] == "" {
			inData = keyMatch[1] == "data" || keyMatch[1] == "stringData"
			dataIndent = indent
			out = append(out, line)
			continue
		}
		if inData && trimmed != "" && indent <= dataIndent && !strings.HasPrefix(trimmed, "data:") && !strings.HasPrefix(trimmed, "stringData:") {
			inData = false
		}
		if len(keyMatch) == 3 && sensitiveYAMLKeys.MatchString(keyMatch[1]) && keyMatch[2] != "" {
			colon := strings.Index(line, ":")
			out = append(out, line[:colon+1]+` "***"`)
			continue
		}
		if inData && !strings.HasSuffix(trimmed, ":") && strings.Contains(trimmed, ":") {
			colon := strings.Index(line, ":")
			if colon >= 0 {
				out = append(out, line[:colon+1]+` "***"`)
				continue
			}
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// RedactKubectlStdout masks Secret/ConfigMap/kubeconfig payloads in kubectl stdout.
func RedactKubectlStdout(args []string, stdout string) string {
	if SensitiveStdoutKind(args) == "" {
		return stdout
	}
	trimmed := strings.TrimSpace(stdout)
	if strings.HasPrefix(trimmed, "{") {
		return redactJSON(stdout)
	}
	if strings.Contains(trimmed, "apiVersion:") || strings.Contains(trimmed, "data:") || strings.Contains(trimmed, "clusters:") {
		return redactYAML(stdout)
	}
	return stdout
}
