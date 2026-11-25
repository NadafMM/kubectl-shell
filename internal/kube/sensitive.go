package kube

import "strings"

func hasSensitiveFlag(tokens []string, name string) bool {
	for _, token := range tokens {
		if token == name || strings.HasPrefix(token, name+"=") {
			return true
		}
	}
	return false
}

func outputFormatToken(token string) (string, bool) {
	switch {
	case token == "-o" || token == "--output":
		return "", false
	case strings.HasPrefix(token, "-o="):
		return token[3:], true
	case strings.HasPrefix(token, "--output="):
		return token[len("--output="):], true
	default:
		return "", false
	}
}

func isRevealingFormat(format string) bool {
	switch format {
	case "yaml", "json", "wide", "template":
		return true
	}
	return strings.HasPrefix(format, "jsonpath") || strings.HasPrefix(format, "go-template") || strings.HasPrefix(format, "custom-columns")
}

func revealingOutputFormat(tokens []string) bool {
	if output, ok := FlagValue(tokens, []string{"-o", "--output"}); ok && isRevealingFormat(output) {
		return true
	}
	for _, token := range tokens {
		if inline, ok := outputFormatToken(token); ok && isRevealingFormat(inline) {
			return true
		}
		if token == "-o=wide" || token == "--output=wide" {
			return true
		}
	}
	return false
}

func resourceTokenKind(token string) string {
	resource, ok := ResolveResource(token)
	if (ok && resource.Kind == "secret") || token == "secret" || token == "secrets" {
		return "secret"
	}
	if (ok && resource.Kind == "configmap") || token == "configmap" || token == "configmaps" {
		return "configmap"
	}
	return ""
}

func sensitiveStdoutKind(tokens []string) string {
	words := PositionalWords(tokens).Words
	verb := ""
	if len(words) > 0 {
		verb = words[0]
	}
	if verb == "get" && revealingOutputFormat(tokens) {
		for i := 1; i < len(words); i++ {
			if kind := resourceTokenKind(words[i]); kind != "" {
				return kind
			}
		}
		return ""
	}
	if verb == "describe" {
		second := ""
		if len(words) > 1 {
			second = words[1]
		}
		return resourceTokenKind(second)
	}
	if verb == "config" && len(words) > 1 && words[1] == "view" && hasSensitiveFlag(tokens, "--raw") {
		return "kubeconfig"
	}
	return ""
}
