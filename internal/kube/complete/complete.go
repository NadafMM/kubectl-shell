package complete

import (
	"github.com/NadafMM/kubectl-shell/internal/kube"
	"github.com/NadafMM/kubectl-shell/internal/kube/parse"
	"math"
	"regexp"
	"sort"
	"strings"
)

type Completion struct {
	Start      int
	End        int
	Candidates []kube.Candidate
}

const candidateLimit = 24

func rank(prefix string, items []kube.Candidate) []kube.Candidate {
	p := strings.ToLower(prefix)
	type scored struct {
		item  kube.Candidate
		score float64
	}
	var rows []scored
	for _, item := range items {
		value := strings.ToLower(item.Value)
		var aliases []string
		for _, alias := range item.Aliases {
			aliases = append(aliases, strings.ToLower(alias))
		}
		var score float64
		switch {
		case p == "":
			score = float64(item.Weight)
			if item.Weight == 0 {
				score = 1
			}
		case value == p:
			score = 1200 + float64(item.Weight)/1000
		case kube.ContainsString(aliases, p):
			score = 1100
		case strings.HasPrefix(value, p):
			score = 800 - math.Min(float64(len(value)), 180) + float64(item.Weight)/100
		case hasPrefixAny(aliases, p):
			score = 760 + float64(item.Weight)/100
		case len(p) >= 2 && (strings.Contains(value, p) || containsFold(aliases, p)):
			score = 420
		default:
			continue
		}
		rows = append(rows, scored{item: item, score: score})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].score != rows[j].score {
			return rows[i].score > rows[j].score
		}
		return rows[i].item.Value < rows[j].item.Value
	})
	var unique []kube.Candidate
	seen := map[string]bool{}
	for _, row := range rows {
		if seen[row.item.Value] {
			continue
		}
		seen[row.item.Value] = true
		unique = append(unique, row.item)
		if len(unique) >= candidateLimit {
			break
		}
	}
	if len(unique) == 0 && len(p) >= 3 {
		type near struct {
			item kube.Candidate
			dist int
		}
		var fuzzy []near
		for _, item := range items {
			dist := editDistance(p, strings.ToLower(item.Value))
			if dist <= 2 {
				fuzzy = append(fuzzy, near{item: item, dist: dist})
			}
		}
		sort.SliceStable(fuzzy, func(i, j int) bool {
			if fuzzy[i].dist != fuzzy[j].dist {
				return fuzzy[i].dist < fuzzy[j].dist
			}
			return fuzzy[i].item.Value < fuzzy[j].item.Value
		})
		if len(fuzzy) > 6 {
			fuzzy = fuzzy[:6]
		}
		out := make([]kube.Candidate, 0, len(fuzzy))
		for _, row := range fuzzy {
			out = append(out, row.item)
		}
		return out
	}
	return unique
}

func hasPrefixAny(items []string, prefix string) bool {
	for _, item := range items {
		if strings.HasPrefix(item, prefix) {
			return true
		}
	}
	return false
}

func containsFold(items []string, part string) bool {
	for _, item := range items {
		if strings.Contains(item, part) {
			return true
		}
	}
	return false
}

const maxEditDistanceLen = 256

func editDistance(a, b string) int {
	if len(a) > maxEditDistanceLen || len(b) > maxEditDistanceLen {
		return 9
	}
	if abs(len(a)-len(b)) > 2 {
		return 9
	}
	var prev, curr [maxEditDistanceLen + 1]int
	for j := 0; j <= len(b); j++ {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(prev[j]+1, curr[j-1]+1, prev[j-1]+cost)
		}
		prev = curr
	}
	return prev[len(b)]
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func valueItems(kind string, args []string, cluster kube.Cluster, scope parse.Scope, prefix string) []kube.Candidate {
	switch kind {
	case "namespace":
		out := make([]kube.Candidate, 0, len(cluster.Namespaces))
		for _, item := range cluster.Namespaces {
			out = append(out, kube.Candidate{Value: item.Name, Detail: "Namespace", Kind: "value", Weight: 40})
		}
		return out
	case "context":
		out := make([]kube.Candidate, 0, len(cluster.Contexts))
		for _, item := range cluster.Contexts {
			out = append(out, kube.Candidate{Value: item.Name, Detail: item.Cluster, Kind: "context", Weight: 40})
		}
		return out
	case "output":
		return numbered([]string{"wide", "yaml", "json", "name", "jsonpath", "custom-columns"}, "Output format", 80)
	case "container":
		names := map[string]bool{}
		for _, item := range referencedPods(args, scope, cluster) {
			for _, container := range item.Containers {
				names[container.Name] = true
			}
		}
		if len(names) == 0 {
			for _, item := range cluster.Objects {
				if item.Kind == "pod" && parse.InScope(item.NS, scope) {
					for _, container := range item.Containers {
						names[container.Name] = true
					}
				}
			}
		}
		out := make([]kube.Candidate, 0, len(names))
		for name := range names {
			out = append(out, kube.Candidate{Value: name, Detail: "Container", Kind: "value", Weight: 40})
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Value < out[j].Value })
		return out
	case "file", "dir":
		return parse.ManifestCandidates(prefix, "")
	case "selector":
		return selectorItems(scope, cluster)
	case "replicas":
		return plainValues([]string{"1", "2", "3", "4", "5"}, "Replica count", 40)
	case "tail":
		return numbered([]string{"20", "50", "100", "200", "500"}, "Log lines", 50)
	case "dryrun":
		return []kube.Candidate{
			{Value: "client", Detail: "Validate locally", Kind: "value", Weight: 60},
			{Value: "server", Detail: "Validate on the API server", Kind: "value", Weight: 50},
			{Value: "none", Detail: "Persist the change", Kind: "value", Weight: 20},
		}
	case "image":
		return nil
	case "port":
		return plainValues([]string{"80", "443", "8080", "3000", "9090"}, "Port", 30)
	case "for":
		return numbered([]string{"condition=ready", "condition=available", "condition=complete", "delete"}, "Wait condition", 40)
	case "sort":
		return plainValues([]string{".metadata.name", ".metadata.creationTimestamp", ".status.phase"}, "JSON path", 20)
	case "since":
		return plainValues([]string{"5s", "15m", "1h", "24h"}, "Duration", 20)
	case "address":
		return plainValues([]string{"127.0.0.1", "localhost", "0.0.0.0"}, "Bind address", 20)
	case "revision":
		return plainValues([]string{"1", "2"}, "Revision", 20)
	case "svctype":
		return plainValues([]string{"ClusterIP", "NodePort", "LoadBalancer"}, "Service type", 30)
	case "restart":
		return plainValues([]string{"Always", "OnFailure", "Never"}, "Restart policy", 30)
	case "literal":
		return []kube.Candidate{
			{Value: "LOG_LEVEL=info", Detail: "Example pair", Kind: "value", Weight: 20},
			{Value: "color=blue", Detail: "Example pair", Kind: "value", Weight: 16},
		}
	case "timeout":
		return plainValues([]string{"30s", "60s", "5m"}, "Duration", 20)
	case "grace":
		return plainValues([]string{"0", "30"}, "Seconds", 20)
	case "level":
		return plainValues([]string{"0", "2", "4", "6", "8"}, "Verbosity", 10)
	case "field":
		return []kube.Candidate{
			{Value: "status.phase=Running", Detail: "Running pods", Kind: "value", Weight: 30},
			{Value: "status.phase=Pending", Detail: "Pending pods", Kind: "value", Weight: 20},
		}
	case "user":
		return plainValues([]string{"system:admin", "alice"}, "User", 10)
	case "group":
		return plainValues([]string{"apps", "batch"}, "API group", 10)
	case "patchtype":
		return plainValues([]string{"strategic", "merge", "json"}, "Patch type", 20)
	default:
		return nil
	}
}

func plainValues(values []string, detail string, weight int) []kube.Candidate {
	out := make([]kube.Candidate, 0, len(values))
	for _, value := range values {
		out = append(out, kube.Candidate{Value: value, Detail: detail, Kind: "value", Weight: weight})
	}
	return out
}

func numbered(values []string, detail string, start int) []kube.Candidate {
	out := make([]kube.Candidate, 0, len(values))
	for i, value := range values {
		out = append(out, kube.Candidate{Value: value, Detail: detail, Kind: "value", Weight: start - i})
	}
	return out
}

func selectorItems(scope parse.Scope, cluster kube.Cluster) []kube.Candidate {
	seen := map[string]bool{}
	var items []kube.Candidate
	for _, object := range cluster.Objects {
		if !parse.InScope(object.NS, scope) {
			continue
		}
		keys := make([]string, 0, len(object.Labels))
		for key := range object.Labels {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			pair := key + "=" + object.Labels[key]
			if seen[pair] {
				continue
			}
			seen[pair] = true
			items = append(items, kube.Candidate{Value: pair, Detail: "Label selector", Kind: "value", Weight: 30})
		}
	}
	return items
}

func referencedPods(args []string, scope parse.Scope, cluster kube.Cluster) []kube.KObject {
	names := map[string]bool{}
	for _, word := range parse.PositionalWords(args).Words {
		names[word] = true
	}
	var out []kube.KObject
	for _, object := range cluster.Objects {
		if object.Kind == "pod" && parse.InScope(object.NS, scope) && names[object.Name] {
			out = append(out, object)
		}
	}
	return out
}

func shellWords() []kube.Candidate {
	return []kube.Candidate{
		{Value: "bash", Detail: "Interactive shell", Kind: "command", Weight: 100},
		{Value: "sh", Detail: "POSIX shell", Kind: "command", Weight: 80},
		{Value: "ls", Detail: "List files", Kind: "command", Weight: 70},
		{Value: "env", Detail: "Print environment", Kind: "command", Weight: 64},
		{Value: "printenv", Detail: "Print environment", Kind: "command", Weight: 40},
		{Value: "pwd", Detail: "Working directory", Kind: "command", Weight: 50},
		{Value: "whoami", Detail: "Current user", Kind: "command", Weight: 46},
		{Value: "ps", Detail: "Process list", Kind: "command", Weight: 30},
		{Value: "cat", Detail: "Print a file", Kind: "command", Weight: 28},
	}
}

var equalsFlag = regexp.MustCompile(`^(--?[A-Za-z0-9-]+)=(.*)$`)

func Complete(line string, cursor int, cluster kube.Cluster) Completion {
	loc := parse.Locate(line, cursor)
	prior := make([]string, 0, loc.Index)
	for _, token := range loc.Tokens[:loc.Index] {
		prior = append(prior, token.Text)
	}
	prefix := ""
	if loc.Start <= cursor && cursor <= len(line) {
		prefix = line[loc.Start:cursor]
	}
	base := Completion{Start: loc.Start, End: loc.End}

	if len(prior) == 0 {
		items := []kube.Candidate{{
			Value: "kubectl", Detail: "Kubernetes CLI", Kind: "command", Weight: 100, Aliases: []string{"k"},
		}}
		items = append(items, parse.ShellCommandCandidates(prefix)...)
		items = append(items, kube.Candidate{Value: "help", Detail: "How Tab completion works here", Kind: "command", Weight: 10})
		base.Candidates = rank(prefix, items)
		return base
	}

	bin := prior[0]
	if bin == "view" || bin == "ui" {
		if len(prior) == 1 {
			base.Candidates = rank(prefix, append([]kube.Candidate{}, parse.ViewArgs...))
			return base
		}
	}

	if bin != "kubectl" && bin != "k" && bin != "help" {
		shell := parse.ShellCommandCandidates(bin)
		if len(shell) > 0 {
			base.Candidates = rank(prefix, shell)
			return base
		}
		base.Candidates = rank(prefix, append(parse.ShellCommandCandidates(bin), kube.Candidate{
			Value: "kubectl", Detail: "Commands start with kubectl", Kind: "command", Weight: 60,
		}))
		return base
	}

	if bin == "help" {
		base.Candidates = rank(prefix, parse.VerbCandidates())
		return base
	}

	args := prior[1:]
	scope := parse.ScopeOf(append([]string{"kubectl"}, args...), cluster)

	if equals := equalsFlag.FindStringSubmatch(prefix); equals != nil {
		flag := equals[1]
		kind := parse.TakesOf(flag, parse.VerbOf(args))
		var values []kube.Candidate
		if kind != "" {
			values = valueItems(kind, args, cluster, scope, equals[2])
		}
		mapped := make([]kube.Candidate, 0, len(values))
		for _, item := range values {
			item.Value = flag + "=" + item.Value
			mapped = append(mapped, item)
		}
		base.Candidates = rank(prefix, mapped)
		return base
	}

	if strings.HasPrefix(prefix, "-") {
		base.Candidates = rank(prefix, parse.FlagItems(parse.VerbOf(args)))
		return base
	}

	var previous string
	if len(args) > 0 {
		previous = args[len(args)-1]
	}
	verb := parse.VerbOf(args)
	if previous != "" && parse.TakesOf(previous, verb) != "" && !strings.Contains(previous, "=") {
		kind := parse.TakesOf(previous, verb)
		base.Candidates = rank(prefix, valueItems(kind, args[:len(args)-1], cluster, scope, prefix))
		return base
	}

	parsed := parse.PositionalWords(args)
	if parsed.AfterDashDash {
		base.Candidates = rank(prefix, shellWords())
		return base
	}
	if len(parsed.Words) == 0 {
		base.Candidates = rank(prefix, parse.VerbCandidates())
		return base
	}
	rest := parsed.Words[1:]
	if strings.Contains(prefix, "/") && len(rest) == 0 {
		slash := strings.Index(prefix, "/")
		base.Candidates = rank(prefix, parse.SlashNames(prefix[:slash], prefix[slash+1:], scope, cluster))
		return base
	}
	base.Candidates = rank(prefix, parse.CandidatesFor(verb, rest, args, scope, cluster, prefix))
	return base
}
