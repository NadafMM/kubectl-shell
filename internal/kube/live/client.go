package live

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/NadafMM/kubectl-shell/internal/kube"
	"github.com/NadafMM/kubectl-shell/internal/kube/parse"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	discmemory "k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
)

type kubeClientBundle struct {
	config    *rest.Config
	clientset *kubernetes.Clientset
	dynamic   dynamic.Interface
	discovery discovery.CachedDiscoveryInterface
	mapper    *restmapper.DeferredDiscoveryRESTMapper
}

var (
	clientMu    sync.Mutex
	clientCache = map[string]*kubeClientBundle{}
)

// UsingExecHook reports whether test or override env vars force shelling out to kubectl.
func UsingExecHook() bool {
	return os.Getenv("KUBE_SHELL_KUBECTL") != "" || os.Getenv("KUBE_SHELL_NODE_SCRIPT") != ""
}

// ResetClientCache drops pooled client-go connections (e.g. on sandbox or kubeconfig changes).
func ResetClientCache() {
	clientMu.Lock()
	defer clientMu.Unlock()
	clientCache = map[string]*kubeClientBundle{}
}

func loadingRules() *clientcmd.ClientConfigLoadingRules {
	return clientcmd.NewDefaultClientConfigLoadingRules()
}

// LoadKubeconfigCluster reads kubeconfig directly via client-go without spawning kubectl.
func LoadKubeconfigCluster() (kube.Cluster, bool) {
	raw, err := loadingRules().Load()
	if err != nil || raw == nil || strings.TrimSpace(raw.CurrentContext) == "" {
		return kube.Cluster{}, false
	}
	contexts := make([]kube.Context, 0, len(raw.Contexts))
	for name, ctx := range raw.Contexts {
		if ctx == nil {
			continue
		}
		ns := ctx.Namespace
		if ns == "" {
			ns = "default"
		}
		contexts = append(contexts, kube.Context{
			Name:      name,
			Cluster:   ctx.Cluster,
			User:      ctx.AuthInfo,
			Namespace: ns,
		})
	}
	return kube.Cluster{
		Current:    raw.CurrentContext,
		Contexts:   contexts,
		Namespaces: []kube.Namespace{},
		Nodes:      []kube.Node{},
		PVs:        []kube.PV{},
		Events:     []kube.Event{},
		Objects:    []kube.KObject{},
	}, true
}

func clientForContext(contextName string) (*kubeClientBundle, error) {
	clientMu.Lock()
	if bundle, ok := clientCache[contextName]; ok {
		clientMu.Unlock()
		return bundle, nil
	}
	clientMu.Unlock()

	overrides := &clientcmd.ConfigOverrides{}
	if contextName != "" {
		overrides.CurrentContext = contextName
	}
	cc := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules(), overrides)
	cfg, err := cc.ClientConfig()
	if err != nil {
		return nil, err
	}
	cfg.QPS = 50
	cfg.Burst = 100
	cfg.Timeout = 6 * time.Second

	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	disc := discmemory.NewMemCacheClient(cs.Discovery())
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(disc)
	bundle := &kubeClientBundle{
		config:    cfg,
		clientset: cs,
		dynamic:   dyn,
		discovery: disc,
		mapper:    mapper,
	}
	clientMu.Lock()
	clientCache[contextName] = bundle
	clientMu.Unlock()
	return bundle, nil
}

// ProbeLiveDirect checks cluster API reachability using pooled client-go HTTP/2.
func ProbeLiveDirect() bool {
	if UsingExecHook() {
		return false
	}
	bundle, err := clientForContext("")
	if err != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	_, err = bundle.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{Limit: 1})
	return err == nil
}

var builtinGVRs = map[string]struct {
	gvr        schema.GroupVersionResource
	namespaced bool
}{
	"pods":                      {schema.GroupVersionResource{Version: "v1", Resource: "pods"}, true},
	"services":                  {schema.GroupVersionResource{Version: "v1", Resource: "services"}, true},
	"configmaps":                {schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}, true},
	"secrets":                   {schema.GroupVersionResource{Version: "v1", Resource: "secrets"}, true},
	"serviceaccounts":           {schema.GroupVersionResource{Version: "v1", Resource: "serviceaccounts"}, true},
	"persistentvolumeclaims":    {schema.GroupVersionResource{Version: "v1", Resource: "persistentvolumeclaims"}, true},
	"persistentvolumes":         {schema.GroupVersionResource{Version: "v1", Resource: "persistentvolumes"}, false},
	"namespaces":                {schema.GroupVersionResource{Version: "v1", Resource: "namespaces"}, false},
	"nodes":                     {schema.GroupVersionResource{Version: "v1", Resource: "nodes"}, false},
	"events":                    {schema.GroupVersionResource{Version: "v1", Resource: "events"}, true},
	"endpoints":                 {schema.GroupVersionResource{Version: "v1", Resource: "endpoints"}, true},
	"resourcequotas":            {schema.GroupVersionResource{Version: "v1", Resource: "resourcequotas"}, true},
	"limitranges":               {schema.GroupVersionResource{Version: "v1", Resource: "limitranges"}, true},
	"deployments":               {schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}, true},
	"replicasets":               {schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "replicasets"}, true},
	"statefulsets":              {schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"}, true},
	"daemonsets":                {schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "daemonsets"}, true},
	"jobs":                      {schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}, true},
	"cronjobs":                  {schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "cronjobs"}, true},
	"ingresses":                 {schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"}, true},
	"httproutes":                {schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "httproutes"}, true},
	"networkpolicies":           {schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies"}, true},
	"horizontalpodautoscalers":  {schema.GroupVersionResource{Group: "autoscaling", Version: "v2", Resource: "horizontalpodautoscalers"}, true},
	"poddisruptionbudgets":      {schema.GroupVersionResource{Group: "policy", Version: "v1", Resource: "poddisruptionbudgets"}, true},
	"storageclasses":            {schema.GroupVersionResource{Group: "storage.k8s.io", Version: "v1", Resource: "storageclasses"}, false},
	"priorityclasses":           {schema.GroupVersionResource{Group: "scheduling.k8s.io", Version: "v1", Resource: "priorityclasses"}, false},
	"customresourcedefinitions": {schema.GroupVersionResource{Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions"}, false},
	"endpointslices":            {schema.GroupVersionResource{Group: "discovery.k8s.io", Version: "v1", Resource: "endpointslices"}, true},
}

func resolveGVR(bundle *kubeClientBundle, token string) (schema.GroupVersionResource, bool, bool) {
	if res, ok := parse.ResolveResource(token); ok {
		if entry, hit := builtinGVRs[res.Name]; hit {
			return entry.gvr, entry.namespaced, true
		}
	}
	base := strings.ToLower(strings.Split(token, ".")[0])
	if entry, hit := builtinGVRs[base]; hit {
		return entry.gvr, entry.namespaced, true
	}
	if bundle == nil || bundle.mapper == nil {
		return schema.GroupVersionResource{}, false, false
	}
	gvr, err := bundle.mapper.ResourceFor(schema.GroupVersionResource{Resource: strings.ToLower(token)})
	if err != nil {
		return schema.GroupVersionResource{}, false, false
	}
	gvk, err := bundle.mapper.KindFor(gvr)
	if err != nil {
		return gvr, true, true
	}
	mapping, err := bundle.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return gvr, true, true
	}
	return mapping.Resource, mapping.Scope.Name() == "namespace", true
}

// RunJsonDirect executes simple `kubectl get ... -o json` requests directly via
// client-go dynamic client. The boolean return is false when the caller should
// fall back to spawning kubectl (e.g. test hooks, delete --dry-run, or multi-resource).
func RunJsonDirect(args []string, timeoutMs int) (KubectlJSONResult, bool) {
	if UsingExecHook() || len(args) < 2 || args[0] != "get" {
		return KubectlJSONResult{}, false
	}
	// Multi-resource comma lists (deployments.apps,statefulsets.apps) fall back to kubectl or split.
	if strings.Contains(args[1], ",") {
		return KubectlJSONResult{}, false
	}
	var ns, contextName, labelSel, fieldSel string
	allNS := false
	var names []string
	for i := 1; i < len(args); i++ {
		tok := args[i]
		switch {
		case tok == "-A" || tok == "--all-namespaces":
			allNS = true
		case tok == "-n" || tok == "--namespace":
			if i+1 < len(args) {
				i++
				ns = args[i]
			}
		case strings.HasPrefix(tok, "-n="):
			ns = tok[3:]
		case strings.HasPrefix(tok, "--namespace="):
			ns = tok[len("--namespace="):]
		case tok == "--context":
			if i+1 < len(args) {
				i++
				contextName = args[i]
			}
		case strings.HasPrefix(tok, "--context="):
			contextName = tok[len("--context="):]
		case tok == "-l" || tok == "--selector":
			if i+1 < len(args) {
				i++
				labelSel = args[i]
			}
		case strings.HasPrefix(tok, "-l="):
			labelSel = tok[3:]
		case strings.HasPrefix(tok, "--selector="):
			labelSel = tok[len("--selector="):]
		case tok == "--field-selector":
			if i+1 < len(args) {
				i++
				fieldSel = args[i]
			}
		case strings.HasPrefix(tok, "--field-selector="):
			fieldSel = tok[len("--field-selector="):]
		case tok == "-o" || tok == "--output" || tok == "--request-timeout":
			if i+1 < len(args) {
				i++
			}
		case strings.HasPrefix(tok, "-o=") || strings.HasPrefix(tok, "--output=") || strings.HasPrefix(tok, "--request-timeout="):
		case strings.HasPrefix(tok, "-"):
			return KubectlJSONResult{}, false
		default:
			if i > 1 {
				names = append(names, tok)
			}
		}
	}
	bundle, err := clientForContext(contextName)
	if err != nil {
		return KubectlJSONResult{}, false
	}
	gvr, namespaced, ok := resolveGVR(bundle, args[1])
	if !ok {
		return KubectlJSONResult{}, false
	}
	if timeoutMs <= 0 {
		timeoutMs = 3000
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	var ri dynamic.ResourceInterface
	if namespaced && !allNS {
		if ns == "" {
			if cfg, cok := LoadKubeconfigCluster(); cok {
				ns = kube.NamespaceOf(cfg)
			}
			if ns == "" {
				ns = "default"
			}
		}
		ri = bundle.dynamic.Resource(gvr).Namespace(ns)
	} else {
		ri = bundle.dynamic.Resource(gvr)
	}

	if len(names) == 1 {
		item, gerr := ri.Get(ctx, names[0], metav1.GetOptions{})
		if gerr != nil {
			return KubectlJSONResult{OK: false, Error: stderrFirstLine(gerr.Error())}, true
		}
		raw, merr := json.Marshal(item.Object)
		if merr != nil {
			return KubectlJSONResult{OK: false, Error: merr.Error()}, true
		}
		return KubectlJSONResult{OK: true, JSON: raw}, true
	}
	if len(names) > 1 {
		items := make([]map[string]any, 0, len(names))
		for _, n := range names {
			item, gerr := ri.Get(ctx, n, metav1.GetOptions{})
			if gerr != nil {
				return KubectlJSONResult{OK: false, Error: stderrFirstLine(gerr.Error())}, true
			}
			items = append(items, item.Object)
		}
		raw, merr := json.Marshal(map[string]any{"apiVersion": "v1", "kind": "List", "items": items})
		if merr != nil {
			return KubectlJSONResult{OK: false, Error: merr.Error()}, true
		}
		return KubectlJSONResult{OK: true, JSON: raw}, true
	}
	list, lerr := ri.List(ctx, metav1.ListOptions{
		LabelSelector: labelSel,
		FieldSelector: fieldSel,
	})
	if lerr != nil {
		return KubectlJSONResult{OK: false, Error: stderrFirstLine(lerr.Error())}, true
	}
	raw, merr := json.Marshal(list.Object)
	if merr != nil {
		return KubectlJSONResult{OK: false, Error: merr.Error()}, true
	}
	return KubectlJSONResult{OK: true, JSON: raw}, true
}

// DiscoverClusterResources returns discovered CRDs and non-builtin API resources.
func DiscoverClusterResources(contextName string) []kube.Resource {
	if UsingExecHook() {
		return nil
	}
	bundle, err := clientForContext(contextName)
	if err != nil {
		return nil
	}
	lists, err := bundle.discovery.ServerPreferredResources()
	if err != nil && len(lists) == 0 {
		return nil
	}
	var out []kube.Resource
	seen := map[string]bool{}
	for _, list := range lists {
		for _, r := range list.APIResources {
			if strings.Contains(r.Name, "/") || r.Name == "" || seen[r.Name] {
				continue
			}
			if _, builtin := builtinGVRs[r.Name]; builtin {
				continue
			}
			seen[r.Name] = true
			singular := r.SingularName
			if singular == "" {
				singular = strings.ToLower(r.Kind)
			}
			out = append(out, kube.Resource{
				Name:       r.Name,
				Short:      r.ShortNames,
				Singular:   singular,
				Kind:       singular,
				Detail:     list.GroupVersion,
				Weight:     14,
				Namespaced: r.Namespaced,
				API:        list.GroupVersion,
				APIKind:    r.Kind,
			})
		}
	}
	return out
}

func mapUnstructuredToKItem(u unstructured.Unstructured) kItem {
	raw, err := json.Marshal(u.Object)
	if err != nil {
		return kItem{}
	}
	// Strip Secret/ConfigMap data immediately before mapping into cacheable kItem.
	var item kItem
	_ = json.Unmarshal(raw, &item)
	return item
}

// FetchLiveClusterDirect populates a completion/view snapshot directly via client-go.
func FetchLiveClusterDirect(scope parse.Scope, base kube.Cluster) (kube.Cluster, bool) {
	if UsingExecHook() {
		return kube.Cluster{}, false
	}
	bundle, err := clientForContext(scope.Context)
	if err != nil {
		return kube.Cluster{}, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()

	var (
		wg         sync.WaitGroup
		mu         sync.Mutex
		namespaces []kube.Namespace
		nodes      []kube.Node
		pvs        []kube.PV
		events     []kube.Event
		objects    []kube.KObject
		dynRes     []kube.Resource
	)

	wg.Add(1)
	go func() {
		defer wg.Done()
		dynRes = DiscoverClusterResources(scope.Context)
	}()

	// Fetch cluster-scoped core lists.
	clusterResources := []string{"namespaces", "nodes", "persistentvolumes"}
	for _, rname := range clusterResources {
		rname := rname
		wg.Add(1)
		go func() {
			defer wg.Done()
			entry := builtinGVRs[rname]
			list, lerr := bundle.dynamic.Resource(entry.gvr).List(ctx, metav1.ListOptions{})
			if lerr != nil {
				return
			}
			mu.Lock()
			defer mu.Unlock()
			for _, u := range list.Items {
				item := mapUnstructuredToKItem(u)
				switch rname {
				case "namespaces":
					if item.Metadata != nil && item.Metadata.Name != "" {
						st := "Active"
						if item.Status != nil && item.Status.Phase != "" {
							st = item.Status.Phase
						}
						namespaces = append(namespaces, kube.Namespace{Name: item.Metadata.Name, Status: st, Age: AgeOf(item.Metadata.CreationTimestamp, time.Now())})
					}
				case "nodes":
					if node, ok := MapNode(item); ok {
						nodes = append(nodes, node)
					}
				case "persistentvolumes":
					if item.Metadata != nil && item.Metadata.Name != "" {
						capStr, phase := "", ""
						if item.Spec != nil && item.Spec.Capacity != nil {
							capStr = item.Spec.Capacity.Storage
						}
						if item.Status != nil {
							phase = item.Status.Phase
						}
						pvs = append(pvs, kube.PV{Name: item.Metadata.Name, Capacity: capStr, Status: phase, Age: AgeOf(item.Metadata.CreationTimestamp, time.Now())})
					}
				}
				if mapped, ok := MapListItem(item); ok {
					objects = append(objects, mapped)
				}
			}
		}()
	}

	// Fetch namespaced object projections in parallel without reading Secret/ConfigMap bodies into cache.
	namespacedList := []string{
		"pods", "services", "configmaps", "secrets", "serviceaccounts", "persistentvolumeclaims",
		"deployments", "replicasets", "statefulsets", "daemonsets",
		"jobs", "cronjobs", "ingresses", "networkpolicies", "horizontalpodautoscalers",
		"endpoints", "poddisruptionbudgets", "events",
	}
	targetNS := scope.NS
	if targetNS == "" {
		targetNS = "default"
	}
	for _, rname := range namespacedList {
		rname := rname
		wg.Add(1)
		go func() {
			defer wg.Done()
			entry := builtinGVRs[rname]
			var ri dynamic.ResourceInterface
			if scope.All {
				ri = bundle.dynamic.Resource(entry.gvr)
			} else {
				ri = bundle.dynamic.Resource(entry.gvr).Namespace(targetNS)
			}
			list, lerr := ri.List(ctx, metav1.ListOptions{})
			if lerr != nil {
				return
			}
			mu.Lock()
			defer mu.Unlock()
			for _, u := range list.Items {
				item := mapUnstructuredToKItem(u)
				if rname == "events" {
					if ev, ok := mapEvent(item, targetNS); ok {
						events = append(events, ev)
					}
					continue
				}
				if mapped, ok := MapListItem(item); ok {
					objects = append(objects, mapped)
				}
			}
		}()
	}

	wg.Wait()
	if len(namespaces) == 0 {
		namespaces = base.Namespaces
	}
	if events == nil {
		events = []kube.Event{}
	}
	if nodes == nil {
		nodes = []kube.Node{}
	}
	if pvs == nil {
		pvs = []kube.PV{}
	}
	if objects == nil {
		objects = []kube.KObject{}
	}
	return kube.Cluster{
		Current: base.Current, Contexts: base.Contexts, Namespaces: namespaces,
		Nodes: nodes, PVs: pvs, Events: events, Objects: objects, DynamicResources: dynRes,
	}, true
}
