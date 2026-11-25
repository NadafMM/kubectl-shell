package kube

import (
	"regexp"
	"sort"
	"strings"
)

type Plan struct {
	Command       string   `json:"command"`
	Verb          string   `json:"verb"`
	Context       string   `json:"context"`
	Namespace     string   `json:"namespace"`
	AllNamespaces bool     `json:"allNamespaces"`
	Targets       []string `json:"targets"`
	Mutating      bool     `json:"mutating"`
	Risk          string   `json:"risk"`
	Summary       string   `json:"summary"`
}

type Advice struct {
	Why      string    `json:"why"`
	Commands []string  `json:"commands"`
	Findings []Finding `json:"findings,omitempty"`
}

func argsOf(engineLine string) []string {
	var tokens []string
	for _, token := range Tokenize(engineLine) {
		tokens = append(tokens, Unquote(token.Text))
	}
	if len(tokens) > 0 && (tokens[0] == "kubectl" || tokens[0] == "k") {
		return tokens[1:]
	}
	return tokens
}

func hasFlag(tokens []string, name string) bool {
	for _, token := range tokens {
		if token == name || strings.HasPrefix(token, name+"=") {
			return true
		}
	}
	return false
}

func dryRun(tokens []string) bool {
	for _, token := range tokens {
		if token == "--dry-run" || strings.HasPrefix(token, "--dry-run=") {
			return true
		}
	}
	return false
}

func riskOf(verb, sub string, tokens []string) string {
	switch verb {
	case "delete", "drain", "cordon", "taint", "cp", "attach", "exec", "debug", "port-forward", "proxy":
		return "high"
	}
	if verb == "certificate" && (sub == "approve" || sub == "deny") {
		return "high"
	}
	if verb == "config" && sub == "set-credentials" {
		return "high"
	}
	if !dryRun(tokens) && (verb == "run" || verb == "expose") {
		return "high"
	}
	if verb == "scale" {
		if value, ok := FlagValue(tokens, []string{"--replicas"}); ok && value == "0" {
			return "high"
		}
	}
	if verb == "rollout" && (sub == "undo" || sub == "restart") {
		return "high"
	}
	if verb == "replace" && hasFlag(tokens, "--force") {
		return "high"
	}
	if verb == "config" && (sub == "delete-context" || sub == "delete-cluster" || sub == "delete-user") {
		return "high"
	}
	if !dryRun(tokens) && (verb == "apply" || verb == "create" || verb == "replace" || verb == "patch") {
		return "high"
	}
	low := map[string]bool{
		"scale": true, "edit": true, "label": true, "annotate": true, "set": true,
		"rollout": true, "run": true, "expose": true, "autoscale": true, "uncordon": true,
		"apply": true, "create": true, "replace": true, "patch": true,
	}
	if low[verb] {
		return "low"
	}
	if verb == "config" && (sub == "use-context" || sub == "set-context" || sub == "set-cluster" || sub == "set") {
		return "low"
	}
	return "read"
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
	if verb == "config" && len(words) > 1 && words[1] == "view" && hasFlag(tokens, "--raw") {
		return "kubeconfig"
	}
	return ""
}

func summaryFor(plan Plan, sub string, tokens []string) string {
	where := plan.Namespace
	if plan.AllNamespaces {
		where = "every namespace"
	}
	what := plan.Verb
	if len(plan.Targets) > 0 {
		what = strings.Join(plan.Targets, ", ")
	}
	switch sensitiveStdoutKind(tokens) {
	case "secret":
		return "Reads " + what + " in " + where + ". kubectl may print Secret data on stdout; kube does not redact it unless KUBE_SHELL_REDACT_STDOUT=1 and stdout is not a TTY."
	case "configmap":
		return "Reads " + what + " in " + where + ". kubectl may print ConfigMap data on stdout; kube does not redact it unless KUBE_SHELL_REDACT_STDOUT=1 and stdout is not a TTY."
	case "kubeconfig":
		return "Prints the kubeconfig including client credentials. kube does not redact it unless KUBE_SHELL_REDACT_STDOUT=1 and stdout is not a TTY."
	}
	switch {
	case plan.Verb == "port-forward":
		return "Opens a local port to " + what + " in " + where + "."
	case plan.Verb == "proxy":
		return "Starts a local API proxy using your kubeconfig credentials."
	case plan.Verb == "delete":
		return "Deletes " + what + " in " + where + ". A controller may recreate a pod."
	case plan.Verb == "drain":
		return "Drains " + what + ". Pods are evicted off that node."
	case plan.Verb == "scale" && plan.Risk == "high":
		return "Scales " + what + " to zero replicas in " + where + "."
	case plan.Verb == "rollout" && sub == "undo":
		return "Rolls " + what + " back to the previous revision."
	case plan.Verb == "rollout" && sub == "restart":
		return "Restarts " + what + " in " + where + "."
	case plan.Verb == "cp":
		return "Copies files to or from " + what + "."
	case plan.Verb == "attach":
		return "Attaches to a process in " + what + "."
	case plan.Verb == "exec":
		return "Runs a process inside " + what + ". The container filesystem can change."
	case plan.Verb == "debug":
		return "Starts a debug container for " + what + "."
	case plan.Risk == "read":
		return "Reads " + what + " in " + where + ". Nothing in the cluster changes."
	case plan.Verb == "patch":
		return "Patches " + what + " in " + where + "."
	case plan.Verb == "apply" || plan.Verb == "create" || plan.Verb == "replace":
		return "Writes " + what + " in " + where + "."
	default:
		return "Changes " + what + " in " + where + "."
	}
}

// PlanCommand describes what a kubectl line will touch. It does not run it.
func PlanCommand(engineLine string, cluster Cluster) Plan {
	args := argsOf(engineLine)
	scope := ScopeOf(args, cluster)
	parsed := PositionalWords(args)
	verb, sub := "", ""
	if len(parsed.Words) > 0 {
		verb = parsed.Words[0]
	}
	if len(parsed.Words) > 1 {
		sub = parsed.Words[1]
	}
	resource, hasResource := ResolveResource(sub)
	var names []string
	if hasResource {
		names = parsed.Words[2:]
	} else if verb == "drain" || verb == "cordon" || verb == "uncordon" {
		names = parsed.Words[1:]
	}
	kind := ""
	if hasResource {
		kind = resource.Singular
	} else if verb == "drain" {
		kind = "node"
	}
	targets := []string{}
	if len(names) > 0 {
		for _, name := range names {
			if kind != "" {
				targets = append(targets, kind+"/"+name)
			} else {
				targets = append(targets, name)
			}
		}
	} else if sub != "" {
		if hasFlag(args, "--all") {
			targets = []string{"all " + sub}
		} else {
			targets = []string{sub}
		}
	}
	riskSub := sub
	if hasResource && len(parsed.Words) > 2 {
		riskSub = parsed.Words[2]
	} else if hasResource {
		riskSub = ""
	}
	risk := riskOf(verb, riskSub, args)
	ns := scope.NS
	if ns == "" {
		ns = NamespaceOf(cluster)
	}
	command := "kubectl"
	if len(args) > 0 {
		command += " " + strings.Join(args, " ")
	}
	plan := Plan{
		Command: command, Verb: verb, Context: scope.Context,
		Namespace: ns, AllNamespaces: scope.All, Targets: targets,
		Mutating: risk != "read", Risk: risk,
	}
	summarySub := sub
	if verb != "rollout" && hasResource {
		summarySub = sub
	}
	plan.Summary = summaryFor(plan, summarySub, args)
	return plan
}

func FormatPlan(plan Plan) string {
	where := "namespace " + plan.Namespace
	if plan.AllNamespaces {
		where = "all namespaces"
	}
	return plan.Command + "\ncontext " + plan.Context + " · " + where + " · risk " + plan.Risk + "\n" + plan.Summary
}

type hit struct {
	name, kind, ns, status string
}

func unhealthy(cluster Cluster) (hit, bool) {
	pattern := regexp.MustCompile(`(?i)CrashLoop|ImagePull|ErrImage|Error|Pending|BackOff|OOM`)
	for _, item := range cluster.Objects {
		if pattern.MatchString(item.Status) {
			status := item.Status
			if status == "" {
				status = item.Kind
			}
			return hit{name: item.Name, kind: item.Kind, ns: item.NS, status: status}, true
		}
	}
	return hit{}, false
}

func mentioned(question string, cluster Cluster) (hit, bool) {
	q := strings.ToLower(question)
	ranked := append([]KObject{}, cluster.Objects...)
	sort.SliceStable(ranked, func(i, j int) bool {
		return len(ranked[i].Name) > len(ranked[j].Name)
	})
	for _, item := range ranked {
		if len(item.Name) >= 3 && strings.Contains(q, strings.ToLower(item.Name)) {
			status := item.Status
			if status == "" {
				status = item.Kind
			}
			return hit{name: item.Name, kind: item.Kind, ns: item.NS, status: status}, true
		}
	}
	return hit{}, false
}

func nsSuffix(found hit, ok bool, cluster Cluster) string {
	if !ok || found.ns == "" || found.ns == NamespaceOf(cluster) {
		return ""
	}
	return " -n " + found.ns
}

func podish(found hit, ok bool) string {
	if !ok || found.kind == "pod" {
		return "pod"
	}
	return found.kind
}

// Advise turns a question into kubectl lines. It does not call the network.
func Advise(question string, cluster Cluster) Advice {
	q := strings.ToLower(strings.TrimSpace(question))
	named, namedOK := mentioned(question, cluster)
	sick, sickOK := named, namedOK
	if !sickOK {
		sick, sickOK = unhealthy(cluster)
	}
	ns := nsSuffix(sick, sickOK, cluster)
	name := "<name>"
	if sickOK {
		name = sick.name
	}
	kind := podish(sick, sickOK)
	whyName := "No matching object in this context."
	if sickOK {
		whyName = sick.name + " is " + sick.status + " in " + sick.ns + "."
	}
	switch {
	case q == "":
		return Advice{Why: "Ask about a pod, a crash, a pending schedule, or the current context.", Commands: []string{"get pods", "get events"}}
	case regexp.MustCompile(`context|where am i|which cluster`).MatchString(q):
		return Advice{Why: "Current context is " + cluster.Current + ", namespace " + NamespaceOf(cluster) + ".", Commands: []string{"config current-context", "config get-contexts"}}
	case regexp.MustCompile(`namespace|which ns|what ns`).MatchString(q):
		return Advice{Why: "Namespace on " + cluster.Current + " is " + NamespaceOf(cluster) + ".", Commands: []string{"get ns", "config view --minify"}}
	case regexp.MustCompile(`crash|backoff|restart|oom|not starting|failing`).MatchString(q):
		return Advice{Why: whyName, Commands: []string{"describe " + kind + " " + name + ns, "logs " + name + " --tail=80" + ns, "get events" + ns}}
	case regexp.MustCompile(`pending|unschedul|cannot schedule|can't schedule|insufficient`).MatchString(q):
		return Advice{Why: whyName, Commands: []string{"describe " + kind + " " + name + ns, "get nodes", "get events" + ns}}
	case regexp.MustCompile(`imagepull|errimage|pull back|registry`).MatchString(q):
		return Advice{Why: whyName, Commands: []string{"describe " + kind + " " + name + ns, "get events" + ns}}
	case regexp.MustCompile(`not ready|unhealthy|readiness|liveness|probe`).MatchString(q):
		return Advice{Why: whyName, Commands: []string{"describe " + kind + " " + name + ns, "get pods" + ns}}
	case regexp.MustCompile(`\blogs?\b`).MatchString(q):
		return Advice{Why: whyName, Commands: []string{"logs " + name + " --tail=100" + ns}}
	case regexp.MustCompile(`event`).MatchString(q):
		return Advice{Why: "Recent events explain most scheduling and probe failures.", Commands: []string{"get events" + ns}}
	case regexp.MustCompile(`node`).MatchString(q):
		return Advice{Why: "Node pressure shows up as Pending pods and evictions.", Commands: []string{"get nodes", "describe nodes"}}
	case regexp.MustCompile(`\btop\b|cpu|memory hog|using memory`).MatchString(q):
		return Advice{Why: "Metrics need the metrics-server. If top fails, describe the pod instead.", Commands: []string{"top pods" + ns, "top nodes"}}
	case regexp.MustCompile(`endpoint|service .*down|svc .*down|can.?t connect`).MatchString(q):
		return Advice{Why: "A service with no endpoints has no ready pods behind it.", Commands: []string{"get endpoints" + ns, "get pods" + ns}}
	case regexp.MustCompile(`port-?forward|forward`).MatchString(q):
		return Advice{Why: "Forward a local port to " + name + ".", Commands: []string{"port-forward " + kind + "/" + name + " 8080:80" + ns}}
	case regexp.MustCompile(`what('?s| is) running|all pods|list pods|show pods`).MatchString(q):
		return Advice{Why: whyName, Commands: []string{"get pods" + ns, "get deploy,svc" + ns}}
	default:
		return Advice{Why: whyName, Commands: []string{"get pods" + ns, "describe " + kind + " " + name + ns, "get events" + ns}}
	}
}

func FormatAdvice(advice Advice) string {
	lines := []string{advice.Why, ""}
	for _, line := range advice.Commands {
		lines = append(lines, "kubectl "+line)
	}
	return strings.Join(lines, "\n")
}
