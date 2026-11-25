package kube

import "strings"

// Plan describes what a kubectl line will touch without running it.
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

type KObject struct {
	Kind       string            `json:"kind"`
	Name       string            `json:"name"`
	NS         string            `json:"ns"`
	Age        string            `json:"age"`
	Labels     map[string]string `json:"labels"`
	Status     string            `json:"status,omitempty"`
	SvcType    string            `json:"svcType,omitempty"`
	Containers []Container       `json:"containers,omitempty"`
}

type Container struct {
	Name string `json:"name"`
}

type Context struct {
	Name      string `json:"name"`
	Cluster   string `json:"cluster"`
	User      string `json:"user"`
	Namespace string `json:"namespace"`
}

type Namespace struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Age    string `json:"age"`
}

type Node struct {
	Name        string `json:"name"`
	Status      string `json:"status"`
	Roles       string `json:"roles"`
	Age         string `json:"age"`
	Version     string `json:"version"`
	IP          string `json:"ip"`
	Schedulable bool   `json:"schedulable"`
}

type PV struct {
	Name     string `json:"name"`
	Capacity string `json:"capacity"`
	Mode     string `json:"mode"`
	Status   string `json:"status"`
	Claim    string `json:"claim"`
	Age      string `json:"age"`
}

type Event struct {
	NS      string `json:"ns"`
	Last    string `json:"last"`
	Type    string `json:"type"`
	Reason  string `json:"reason"`
	Object  string `json:"object"`
	Message string `json:"message"`
}

type Cluster struct {
	Current    string      `json:"current"`
	Contexts   []Context   `json:"contexts"`
	Namespaces []Namespace `json:"namespaces"`
	Nodes      []Node      `json:"nodes"`
	PVs        []PV        `json:"pvs"`
	Events     []Event     `json:"events"`
	Objects    []KObject   `json:"objects"`
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
		if resource.Name == name || ContainsString(resource.Short, raw) || resource.Singular == raw {
			return resource, true
		}
	}
	return Resource{}, false
}

// ContainsString reports whether want is in items.
func ContainsString(items []string, want string) bool {
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
