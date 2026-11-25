package kube

import "strings"

type ShellCommand struct {
	Value   string
	Detail  string
	Weight  int
	Aliases []string
}

var ShellCommands = []ShellCommand{
	{Value: "view", Detail: "Full-screen resource list", Weight: 98, Aliases: []string{"ui"}},
	{Value: "ctx", Detail: "Switch context for this shell only", Weight: 97},
	{Value: "ns", Detail: "Switch namespace for this shell only", Weight: 96},
	{Value: "plan", Detail: "What a command will change", Weight: 92},
	{Value: "ask", Detail: "Turn a question into kubectl lines", Weight: 90},
	{Value: "live", Detail: "Reconnect to the cluster", Weight: 86},
	{Value: "reset", Detail: "Refresh names from the cluster", Weight: 84},
	{Value: "audit", Detail: "Show recent redacted command records", Weight: 70},
	{Value: "undo", Detail: "Restore the previous saved object", Weight: 68},
	{Value: "record", Detail: "Record commands that actually ran", Weight: 66},
	{Value: "help", Detail: "List shell commands", Weight: 50},
	{Value: "clear", Detail: "Clear the screen", Weight: 40},
	{Value: "exit", Detail: "Leave the shell", Weight: 30, Aliases: []string{"quit"}},
}

var ViewArgs = []Candidate{
	{Value: "pods", Detail: "Pods board", Kind: "command", Weight: 100},
	{Value: "deploy", Detail: "Deployments board", Kind: "command", Weight: 96},
	{Value: "svc", Detail: "Services board", Kind: "command", Weight: 92},
	{Value: "nodes", Detail: "Nodes board", Kind: "command", Weight: 88},
	{Value: "ns", Detail: "Namespaces board", Kind: "command", Weight: 84},
	{Value: "all", Detail: "Every namespace", Kind: "command", Weight: 80},
	{Value: "-A", Detail: "Every namespace", Kind: "command", Weight: 80},
	{Value: "--all-namespaces", Detail: "Every namespace", Kind: "command", Weight: 78},
}

func namesOfCommand(command ShellCommand) []string {
	return append([]string{command.Value}, command.Aliases...)
}

var cliWords = map[string]bool{
	"--help": true, "-h": true, "--version": true, "-v": true, "completion": true, "sample": true,
}

// MetaOf reports a shell command or CLI flag that must not be sent to kubectl.
func MetaOf(line string) string {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "?") {
		return "ask"
	}
	word := ""
	if fields := strings.Fields(trimmed); len(fields) > 0 {
		word = fields[0]
	}
	if word == "" {
		return ""
	}
	if cliWords[word] {
		return word
	}
	for _, command := range ShellCommands {
		for _, name := range namesOfCommand(command) {
			if name == word {
				return word
			}
		}
	}
	return ""
}

func shellMetaFirstWord(first string) bool {
	if first == "" {
		return false
	}
	lower := strings.ToLower(first)
	for _, command := range ShellCommands {
		for _, name := range namesOfCommand(command) {
			if lower == name {
				return true
			}
			if strings.HasPrefix(name, lower) && len(lower) >= 2 {
				return true
			}
		}
	}
	return false
}

func shellCommandCandidates(prefixToken string) []Candidate {
	p := strings.ToLower(prefixToken)
	var out []Candidate
	for _, command := range ShellCommands {
		item := Candidate{
			Value:   command.Value,
			Detail:  command.Detail,
			Kind:    "command",
			Weight:  command.Weight,
			Aliases: command.Aliases,
		}
		names := append([]string{item.Value}, item.Aliases...)
		keep := false
		for _, name := range names {
			if p == "" || name == p || strings.HasPrefix(name, p) || strings.HasPrefix(p, name) {
				keep = true
				break
			}
		}
		if keep {
			out = append(out, item)
		}
	}
	return out
}
