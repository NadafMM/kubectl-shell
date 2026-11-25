package kube

import (
	"os"
	"path/filepath"
	"strings"
)

func resourceItems(filter func(Resource) bool) []Candidate {
	var out []Candidate
	for _, resource := range Resources {
		if filter != nil && !filter(resource) {
			continue
		}
		out = append(out, Candidate{
			Value:   resource.Name,
			Detail:  resource.Detail,
			Kind:    "resource",
			Weight:  resource.Weight,
			Aliases: append([]string{resource.Singular}, resource.Short...),
		})
	}
	return out
}

func InScope(ns string, scope Scope) bool {
	return scope.All || ns == scope.NS
}

func namesOf(resource Resource, scope Scope, cluster Cluster) []Candidate {
	switch resource.Kind {
	case "namespace":
		out := make([]Candidate, 0, len(cluster.Namespaces))
		for _, item := range cluster.Namespaces {
			out = append(out, Candidate{Value: item.Name, Detail: item.Status, Kind: "namespace", Weight: 50})
		}
		return out
	case "node":
		out := make([]Candidate, 0, len(cluster.Nodes))
		for _, item := range cluster.Nodes {
			detail := item.Roles
			if !item.Schedulable {
				detail = "unschedulable"
			}
			out = append(out, Candidate{Value: item.Name, Detail: detail, Kind: "node", Weight: 50})
		}
		return out
	case "pv":
		if resource.Name != "persistentvolumes" {
			return nil
		}
		out := make([]Candidate, 0, len(cluster.PVs))
		for _, item := range cluster.PVs {
			out = append(out, Candidate{Value: item.Name, Detail: item.Capacity + " " + item.Status, Kind: "volume", Weight: 40})
		}
		return out
	case "event":
		if resource.Name != "events" {
			return nil
		}
		var out []Candidate
		for _, item := range cluster.Events {
			if !InScope(item.NS, scope) {
				continue
			}
			out = append(out, Candidate{
				Value:  strings.ToLower(item.Reason) + "." + strings.ReplaceAll(item.Object, "/", "-"),
				Detail: item.Message,
				Kind:   "event",
				Weight: 20,
			})
		}
		return out
	default:
		var out []Candidate
		for _, item := range cluster.Objects {
			if item.Kind != resource.Kind || (resource.Namespaced && !InScope(item.NS, scope)) {
				continue
			}
			detail := item.Status
			if detail == "" {
				detail = item.SvcType
			}
			if detail == "" {
				detail = item.Kind
			}
			if scope.All {
				status := item.Status
				if status == "" {
					status = item.Kind
				}
				detail = item.NS + " · " + status
			}
			weight := 50
			if item.Status == "CrashLoopBackOff" || item.Status == "Pending" {
				weight = 70
			}
			out = append(out, Candidate{Value: item.Name, Detail: detail, Kind: item.Kind, Weight: weight})
		}
		return out
	}
}

func SlashNames(typeToken, namePrefix string, scope Scope, cluster Cluster) []Candidate {
	resource, ok := ResolveResource(typeToken)
	if !ok {
		return nil
	}
	var out []Candidate
	for _, item := range namesOf(resource, scope, cluster) {
		if strings.HasPrefix(strings.ToLower(item.Value), strings.ToLower(namePrefix)) {
			item.Value = typeToken + "/" + item.Value
			out = append(out, item)
		}
	}
	return out
}

func workloadSlash(kinds []string, scope Scope, cluster Cluster) []Candidate {
	var out []Candidate
	for _, kind := range kinds {
		var resource Resource
		found := false
		for _, item := range Resources {
			if item.Kind == kind && strings.HasSuffix(item.Name, "s") {
				resource, found = item, true
				break
			}
		}
		if !found {
			for _, item := range Resources {
				if item.Kind == kind {
					resource, found = item, true
					break
				}
			}
		}
		if !found {
			continue
		}
		for _, item := range namesOf(resource, scope, cluster) {
			weight := item.Weight
			if weight == 0 {
				weight = 40
			}
			out = append(out, Candidate{
				Value:  resource.Singular + "/" + item.Value,
				Detail: item.Detail,
				Kind:   resource.Singular,
				Weight: weight - 4,
			})
		}
	}
	return out
}

func ManifestCandidates(prefix, root string) []Candidate {
	if root == "" {
		root, _ = os.Getwd()
	}
	normalized := strings.ReplaceAll(prefix, "\\", "/")
	slash := strings.LastIndex(normalized, "/")
	dirPart := ""
	if slash >= 0 {
		dirPart = normalized[:slash]
	}
	dir := filepath.Join(root, filepath.FromSlash(dirPart))
	relative, err := filepath.Rel(root, dir)
	if err != nil || strings.HasPrefix(relative, "..") || filepath.IsAbs(relative) {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []Candidate
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") || name == "node_modules" || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		rel := name
		if dirPart != "" {
			rel = dirPart + "/" + name
		}
		rel = strings.ReplaceAll(rel, "\\", "/")
		if !strings.HasPrefix(strings.ToLower(rel), strings.ToLower(normalized)) {
			continue
		}
		if entry.IsDir() {
			out = append(out, Candidate{Value: rel + "/", Detail: "Directory", Kind: "file", Weight: 35})
		} else if strings.HasSuffix(strings.ToLower(name), ".yaml") || strings.HasSuffix(strings.ToLower(name), ".yml") {
			out = append(out, Candidate{Value: rel, Detail: "Manifest", Kind: "file", Weight: 40})
		}
	}
	return out
}

func FlagItems(verb string) []Candidate {
	flags := FlagsForVerb(verb)
	out := make([]Candidate, 0, len(flags))
	for _, flag := range flags {
		out = append(out, Candidate{Value: flag.Name, Detail: flag.Detail, Kind: "flag", Weight: flag.Weight})
	}
	return out
}

func ports() []Candidate {
	return []Candidate{
		{Value: "8080:80", Detail: "Local 8080 to pod 80", Kind: "port", Weight: 80},
		{Value: "8443:443", Detail: "Local 8443 to pod 443", Kind: "port", Weight: 50},
		{Value: "3000:3000", Detail: "Grafana-style forward", Kind: "port", Weight: 40},
		{Value: "9090:9090", Detail: "Prometheus-style forward", Kind: "port", Weight: 36},
	}
}

func targets(scope Scope, cluster Cluster, kinds []string) []Candidate {
	kindSet := map[string]bool{}
	for _, kind := range kinds {
		kindSet[kind] = true
	}
	return append(resourceItems(func(resource Resource) bool { return kindSet[resource.Kind] }), workloadSlash(kinds, scope, cluster)...)
}

func afterResource(rest []string, scope Scope, cluster Cluster, verb string) []Candidate {
	if len(rest) == 0 {
		return resourceItems(nil)
	}
	if len(rest) == 1 {
		resource, ok := ResolveResource(rest[0])
		if !ok || strings.Contains(rest[0], "/") {
			return FlagItems(verb)
		}
		names := namesOf(resource, scope, cluster)
		if len(names) > 0 {
			return names
		}
		return FlagItems(verb)
	}
	return FlagItems(verb)
}

var nameVerbs = map[string]bool{
	"get": true, "describe": true, "delete": true, "edit": true,
	"label": true, "annotate": true, "patch": true, "wait": true,
}

var flagVerbs = map[string]bool{
	"completion": true, "version": true, "api-resources": true, "api-versions": true,
	"cluster-info": true, "proxy": true, "run": true,
}

type fillCtx struct {
	verb    string
	rest    []string
	scope   Scope
	cluster Cluster
	prefix  string
}

func CandidatesFor(verb string, rest, _ []string, scope Scope, cluster Cluster, prefix string) []Candidate {
	if subs, ok := Subs[verb]; ok && len(rest) == 0 && !strings.Contains(prefix, "/") {
		out := make([]Candidate, 0, len(subs))
		for _, sub := range subs {
			out = append(out, Candidate{Value: sub.Value, Detail: sub.Detail, Kind: "command", Weight: sub.Weight})
		}
		return out
	}
	ctx := fillCtx{verb: verb, rest: rest, scope: scope, cluster: cluster, prefix: prefix}
	switch verb {
	case "config":
		return fillConfig(ctx)
	case "rollout":
		return fillRollout(ctx)
	case "auth":
		return fillAuth(ctx)
	case "create":
		if len(ctx.rest) == 0 {
			return nil
		}
		return FlagItems(ctx.verb)
	case "set":
		return fillSet(ctx)
	case "top":
		return FlagItems("get")
	case "logs", "exec", "debug", "cp":
		return fillPodish(ctx)
	case "port-forward":
		return fillPortForward(ctx)
	case "scale":
		return fillScale(ctx)
	case "expose":
		return fillExpose(ctx)
	case "explain":
		return fillExplain(ctx)
	case "cordon", "uncordon", "drain", "taint":
		return fillNode(ctx)
	case "apply", "diff", "replace":
		return fillManifest(ctx)
	case "help":
		return VerbCandidates()
	case "kustomize":
		var dirs []Candidate
		for _, item := range ManifestCandidates(ctx.prefix, "") {
			if strings.HasSuffix(item.Value, "/") {
				dirs = append(dirs, item)
			}
		}
		return dirs
	}
	if flagVerbs[verb] {
		return FlagItems(verb)
	}
	if nameVerbs[verb] {
		return afterResource(rest, scope, cluster, verb)
	}
	return FlagItems(verb)
}

func fillConfig(ctx fillCtx) []Candidate {
	sub := ""
	if len(ctx.rest) > 0 {
		sub = ctx.rest[0]
	}
	if (sub == "use-context" || sub == "delete-context") && len(ctx.rest) == 1 {
		out := make([]Candidate, 0, len(ctx.cluster.Contexts))
		for _, item := range ctx.cluster.Contexts {
			out = append(out, Candidate{Value: item.Name, Detail: item.Cluster, Kind: "context", Weight: 50})
		}
		return out
	}
	if sub == "set-context" && len(ctx.rest) == 1 {
		out := make([]Candidate, 0, len(ctx.cluster.Contexts)+2)
		for _, item := range ctx.cluster.Contexts {
			out = append(out, Candidate{Value: item.Name, Detail: item.Cluster, Kind: "context", Weight: 40})
		}
		out = append(out,
			Candidate{Value: "--current", Detail: "Modify the active context", Kind: "flag", Weight: 80},
			Candidate{Value: "--namespace", Detail: "Set the default namespace", Kind: "flag", Weight: 70},
		)
		return out
	}
	return FlagItems(ctx.verb)
}

func fillRollout(ctx fillCtx) []Candidate {
	sub := ""
	if len(ctx.rest) > 0 {
		sub = ctx.rest[0]
	}
	after := []string{}
	if len(ctx.rest) > 0 {
		after = ctx.rest[1:]
	}
	if sub == "" {
		return nil
	}
	if len(after) == 0 {
		return targets(ctx.scope, ctx.cluster, []string{"deployment", "statefulset", "daemonset"})
	}
	if len(after) == 1 && !strings.Contains(after[0], "/") {
		if resource, ok := ResolveResource(after[0]); ok {
			return namesOf(resource, ctx.scope, ctx.cluster)
		}
	}
	return FlagItems(ctx.verb)
}

func fillAuth(ctx fillCtx) []Candidate {
	if len(ctx.rest) == 1 {
		verbs := []string{"get", "list", "watch", "create", "update", "patch", "delete", "*"}
		out := make([]Candidate, 0, len(verbs))
		for i, value := range verbs {
			out = append(out, Candidate{Value: value, Detail: "Verb to check", Kind: "verb", Weight: 80 - i})
		}
		return out
	}
	if len(ctx.rest) == 2 {
		return resourceItems(nil)
	}
	return FlagItems(ctx.verb)
}

func fillSet(ctx fillCtx) []Candidate {
	sub := ""
	if len(ctx.rest) > 0 {
		sub = ctx.rest[0]
	}
	after := []string{}
	if len(ctx.rest) > 0 {
		after = ctx.rest[1:]
	}
	if sub == "" {
		return nil
	}
	if len(after) == 0 {
		return targets(ctx.scope, ctx.cluster, []string{"deployment", "statefulset", "daemonset"})
	}
	if sub == "image" && len(after) >= 1 {
		return nil
	}
	return FlagItems(ctx.verb)
}

func fillPortForward(ctx fillCtx) []Candidate {
	if len(ctx.rest) == 0 {
		resource, _ := ResolveResource("pods")
		pods := namesOf(resource, ctx.scope, ctx.cluster)
		return append(pods, workloadSlash([]string{"service", "deployment"}, ctx.scope, ctx.cluster)...)
	}
	if len(ctx.rest) == 1 {
		return ports()
	}
	return FlagItems(ctx.verb)
}

func fillScale(ctx fillCtx) []Candidate {
	if len(ctx.rest) == 0 {
		return targets(ctx.scope, ctx.cluster, []string{"deployment", "statefulset", "replicaset"})
	}
	if len(ctx.rest) == 1 && !strings.Contains(ctx.rest[0], "/") {
		if resource, ok := ResolveResource(ctx.rest[0]); ok {
			return namesOf(resource, ctx.scope, ctx.cluster)
		}
	}
	return FlagItems(ctx.verb)
}

func fillExpose(ctx fillCtx) []Candidate {
	if len(ctx.rest) == 0 {
		kinds := map[string]bool{"deployment": true, "pod": true, "service": true, "replicaset": true}
		return resourceItems(func(resource Resource) bool { return kinds[resource.Kind] })
	}
	if len(ctx.rest) == 1 {
		if resource, ok := ResolveResource(ctx.rest[0]); ok {
			return namesOf(resource, ctx.scope, ctx.cluster)
		}
	}
	return FlagItems(ctx.verb)
}

func fillExplain(ctx fillCtx) []Candidate {
	if len(ctx.rest) != 0 {
		return nil
	}
	out := make([]Candidate, 0, len(ExplainDocs))
	index := 0
	// Map iteration order differs from the TypeScript object order. Rank by the
	// catalog order of keys so an empty prefix stays stable.
	keys := []string{
		"pods", "pods.metadata", "pods.spec", "pods.spec.containers", "pods.spec.containers.image",
		"pods.status", "deployments", "deployments.spec", "services", "services.spec",
	}
	for _, value := range keys {
		info, ok := ExplainDocs[value]
		if !ok {
			continue
		}
		out = append(out, Candidate{Value: value, Detail: info.Detail, Kind: "field", Weight: 100 - index})
		index++
	}
	return out
}

func fillNode(ctx fillCtx) []Candidate {
	if len(ctx.rest) == 0 {
		resource, _ := ResolveResource("nodes")
		return namesOf(resource, ctx.scope, ctx.cluster)
	}
	return FlagItems(ctx.verb)
}

func fillManifest(ctx fillCtx) []Candidate {
	if len(ctx.rest) == 0 && !strings.HasPrefix(ctx.prefix, "-") {
		return append([]Candidate{
			{Value: "-f", Detail: "Apply or diff a file", Kind: "flag", Weight: 100},
			{Value: "-k", Detail: "Kustomize directory", Kind: "flag", Weight: 70},
		}, ManifestCandidates(ctx.prefix, "")...)
	}
	return FlagItems(ctx.verb)
}

func fillPodish(ctx fillCtx) []Candidate {
	if len(ctx.rest) != 0 {
		return FlagItems(ctx.verb)
	}
	resource, _ := ResolveResource("pods")
	pods := namesOf(resource, ctx.scope, ctx.cluster)
	for i := range pods {
		pods[i].Weight = 80
	}
	if ctx.verb == "cp" {
		for i := range pods {
			pods[i].Value = pods[i].Value + ":/app"
			pods[i].Detail = "pod path"
			pods[i].Kind = "path"
		}
		return pods
	}
	return append(pods, workloadSlash([]string{"deployment", "statefulset", "job"}, ctx.scope, ctx.cluster)...)
}

// VerbCandidates completes kubectl verbs from the built-in grammar.
func VerbCandidates() []Candidate {
	out := make([]Candidate, 0, len(Verbs))
	for _, item := range Verbs {
		out = append(out, Candidate{Value: item.Value, Detail: item.Detail, Kind: "verb", Weight: item.Weight})
	}
	return out
}
