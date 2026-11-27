package parse

import (
	"github.com/NadafMM/kubectl-shell/internal/kube"
	"strings"
)

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

func ResolveResource(token string) (kube.Resource, bool) {
	return ResolveResourceInCluster(token, kube.Cluster{})
}

func ResolveResourceInCluster(token string, cluster kube.Cluster) (kube.Resource, bool) {
	raw := strings.ToLower(strings.Split(token, "/")[0])
	name := raw
	if mapped, ok := singular[raw]; ok {
		name = mapped
	}
	for _, resource := range Resources {
		if resource.Name == name || kube.ContainsString(resource.Short, raw) || resource.Singular == raw {
			return resource, true
		}
	}
	for _, resource := range cluster.DynamicResources {
		if strings.ToLower(resource.Name) == raw || strings.ToLower(resource.Singular) == raw || kube.ContainsString(resource.Short, raw) {
			return resource, true
		}
	}
	return kube.Resource{}, false
}

func FlagCatalog(verb string) []kube.FlagDef {
	seen := map[string]bool{}
	var out []kube.FlagDef
	add := func(flags []kube.FlagDef) {
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
