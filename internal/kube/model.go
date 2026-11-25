package kube

import "strings"

type KObject struct {
	Kind       string
	Name       string
	NS         string
	Age        string
	Labels     map[string]string
	Status     string
	SvcType    string
	Containers []Container
}

type Container struct {
	Name string
}

type Context struct {
	Name      string
	Cluster   string
	User      string
	Namespace string
}

type Namespace struct {
	Name   string
	Status string
	Age    string
}

type Node struct {
	Name        string
	Status      string
	Roles       string
	Age         string
	Version     string
	IP          string
	Schedulable bool
}

type PV struct {
	Name     string
	Capacity string
	Mode     string
	Status   string
	Claim    string
	Age      string
}

type Event struct {
	NS      string
	Last    string
	Type    string
	Reason  string
	Object  string
	Message string
}

type Cluster struct {
	Current    string
	Contexts   []Context
	Namespaces []Namespace
	Nodes      []Node
	PVs        []PV
	Events     []Event
	Objects    []KObject
}

type Resource struct {
	Name       string
	Short      []string
	Singular   string
	Kind       string
	Detail     string
	Weight     int
	Namespaced bool
	API        string
	APIKind    string
}

type FlagDef struct {
	Name   string
	Detail string
	Takes  string
	Weight int
}

type Named struct {
	Value  string
	Detail string
	Weight int
}

type Explain struct {
	Detail string
	Doc    string
}

type Candidate struct {
	Value   string
	Detail  string
	Kind    string
	Weight  int
	Aliases []string
}

func CurrentContext(cluster Cluster) (Context, bool) {
	for _, item := range cluster.Contexts {
		if item.Name == cluster.Current {
			return item, true
		}
	}
	if len(cluster.Contexts) > 0 {
		return cluster.Contexts[0], true
	}
	return Context{}, false
}

func NamespaceOf(cluster Cluster) string {
	ctx, ok := CurrentContext(cluster)
	if !ok || ctx.Namespace == "" {
		return "default"
	}
	return ctx.Namespace
}

func PromptOf(cluster Cluster) string {
	ctx, ok := CurrentContext(cluster)
	if !ok {
		return "none:default"
	}
	return ctx.Name + ":" + ctx.Namespace
}

var singular map[string]string

func init() {
	singular = map[string]string{}
	for _, resource := range Resources {
		singular[resource.Singular] = resource.Name
		for _, short := range resource.Short {
			singular[short] = resource.Name
		}
	}
}

func ResolveResource(token string) (Resource, bool) {
	raw := strings.ToLower(strings.Split(token, "/")[0])
	name := raw
	if mapped, ok := singular[raw]; ok {
		name = mapped
	}
	for _, resource := range Resources {
		if resource.Name == name || containsString(resource.Short, raw) || resource.Singular == raw {
			return resource, true
		}
	}
	return Resource{}, false
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func FlagCatalog(verb string) []FlagDef {
	seen := map[string]bool{}
	var out []FlagDef
	add := func(flags []FlagDef) {
		for _, flag := range flags {
			if seen[flag.Name] {
				continue
			}
			seen[flag.Name] = true
			out = append(out, flag)
		}
	}
	if verb != "" {
		add(VerbFlags[verb])
	}
	add(GlobalFlags)
	return out
}

func TakesOf(flag, verb string) string {
	name := flag
	if i := strings.Index(flag, "="); i >= 0 {
		name = flag[:i]
	}
	for _, flagDef := range FlagCatalog(verb) {
		if flagDef.Name == name {
			return flagDef.Takes
		}
	}
	return ""
}
