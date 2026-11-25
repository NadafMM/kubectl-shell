package plan

import (
	"encoding/json"
	"github.com/NadafMM/kubectl-shell/internal/kube"
	"sort"
	"strconv"
	"strings"
	"time"

	// BlastBudgetMS is the wall-clock budget for blast-radius kubectl calls.
	"github.com/NadafMM/kubectl-shell/internal/kube/live"
)

const BlastBudgetMS = 3000

// BlastObject is one affected namespaced object.
type BlastObject struct {
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// BlastService is a Service matched by deleted pods.
type BlastService struct {
	Namespace          string `json:"namespace"`
	Name               string `json:"name"`
	MatchedPods        int    `json:"matchedPods"`
	ReadyPodsRemaining int    `json:"readyPodsRemaining"`
}

// BlastPDB is a PodDisruptionBudget covering deleted pods.
type BlastPDB struct {
	Namespace          string `json:"namespace"`
	Name               string `json:"name"`
	DisruptionsAllowed int    `json:"disruptionsAllowed"`
}

// BlastHPA is an HPA targeting a deleted controller.
type BlastHPA struct {
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
	MinReplicas int    `json:"minReplicas"`
	MaxReplicas int    `json:"maxReplicas"`
	Target      string `json:"target"`
}

// BlastOwner links a pod to its controller.
type BlastOwner struct {
	Namespace string `json:"namespace"`
	Pod       string `json:"pod"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
}

// BlastDryRun is a server-side dry-run summary.
type BlastDryRun struct {
	Status string `json:"status"`
	Count  *int   `json:"count,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// Blast is the blast-radius report for a mutating plan.
type Blast struct {
	Status      string         `json:"status"`
	Reason      string         `json:"reason,omitempty"`
	Controllers []BlastObject  `json:"controllers,omitempty"`
	Objects     []BlastObject  `json:"objects"`
	ObjectCount int            `json:"objectCount"`
	Services    []BlastService `json:"services"`
	ServiceNote string         `json:"serviceNote,omitempty"`
	PDBs        []BlastPDB     `json:"pdbs,omitempty"`
	PDBNote     string         `json:"pdbNote,omitempty"`
	HPAs        []BlastHPA     `json:"hpas,omitempty"`
	Owners      []BlastOwner   `json:"owners,omitempty"`
	DryRun      *BlastDryRun   `json:"dryRun,omitempty"`

	groups    []countGroup `json:"-"`
	kind      string       `json:"-"`
	namespace string       `json:"-"`
}

// BlastClock supplies millisecond timestamps for the blast budget.
type BlastClock interface {
	Now() int64
}

type countGroup struct {
	namespace string
	kind      string
	count     int
}

type kubeObject struct {
	Kind     string `json:"kind"`
	Metadata *struct {
		Name            string            `json:"name"`
		Namespace       string            `json:"namespace"`
		Labels          map[string]string `json:"labels"`
		OwnerReferences []struct {
			Kind       string `json:"kind"`
			Name       string `json:"name"`
			Controller bool   `json:"controller"`
		} `json:"ownerReferences"`
	} `json:"metadata"`
	Spec *struct {
		Selector       json.RawMessage `json:"selector"`
		MinReplicas    *int            `json:"minReplicas"`
		MaxReplicas    *int            `json:"maxReplicas"`
		ScaleTargetRef *struct {
			Kind string `json:"kind"`
			Name string `json:"name"`
		} `json:"scaleTargetRef"`
	} `json:"spec"`
	Status *struct {
		Conditions []struct {
			Type   string `json:"type"`
			Status string `json:"status"`
		} `json:"conditions"`
		DisruptionsAllowed *int `json:"disruptionsAllowed"`
	} `json:"status"`
}

func skipped(reason string) Blast {
	return Blast{Status: "skipped", Reason: reason, Objects: []BlastObject{}, Services: []BlastService{}}
}

func unknown(reason string) Blast {
	return Blast{Status: "unknown", Reason: reason, Objects: []BlastObject{}, Services: []BlastService{}}
}

func noun(kind string, count int) string {
	if count == 1 {
		return kind
	}
	return kind + "s"
}

func orderAndCap(objects []BlastObject) (shown []BlastObject, groups []countGroup) {
	counts := map[string]*countGroup{}
	var order []string
	for _, object := range objects {
		key := object.Namespace + "\x00" + object.Kind
		group, ok := counts[key]
		if !ok {
			group = &countGroup{namespace: object.Namespace, kind: object.Kind}
			counts[key] = group
			order = append(order, key)
		}
		group.count++
	}
	type indexed struct {
		object BlastObject
		index  int
	}
	ordered := make([]indexed, len(objects))
	for i, object := range objects {
		ordered[i] = indexed{object: object, index: i}
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		a, b := ordered[i].object.Namespace, ordered[j].object.Namespace
		if a != b {
			return a < b
		}
		return ordered[i].index < ordered[j].index
	})
	for _, item := range ordered {
		if len(shown) >= 20 {
			break
		}
		shown = append(shown, item.object)
	}
	groups = make([]countGroup, 0, len(order))
	for _, key := range order {
		groups = append(groups, *counts[key])
	}
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].namespace != groups[j].namespace {
			return groups[i].namespace < groups[j].namespace
		}
		return groups[i].kind < groups[j].kind
	})
	if shown == nil {
		shown = []BlastObject{}
	}
	return shown, groups
}

func remember(blast Blast, kind, namespace string, groups []countGroup) Blast {
	blast.kind = kind
	blast.namespace = namespace
	blast.groups = groups
	return blast
}

func pack(objects []BlastObject, kind, namespace string, extra Blast) Blast {
	shown, groups := orderAndCap(objects)
	blast := extra
	blast.Status = "ok"
	blast.Objects = shown
	blast.ObjectCount = len(objects)
	if blast.Services == nil {
		blast.Services = []BlastService{}
	}
	return remember(blast, kind, namespace, groups)
}

func formatOk(blast Blast) string {
	lines := []string{"blast"}
	groups := blast.groups
	if blast.ObjectCount == 0 || len(groups) == 0 {
		kind := blast.kind
		if kind == "" {
			kind = "object"
		}
		ns := blast.namespace
		if ns == "" {
			ns = "default"
		}
		lines = append(lines, "  0 "+noun(kind, 0)+" in namespace "+ns+fromSuffix(blast, ns, kind))
	} else {
		for _, group := range groups {
			lines = append(lines, "  "+strconv.Itoa(group.count)+" "+noun(group.kind, group.count)+" in namespace "+group.namespace+fromSuffix(blast, group.namespace, group.kind))
		}
		for _, object := range blast.Objects {
			lines = append(lines, objectLine(blast, object))
		}
		if blast.ObjectCount > 20 {
			lines = append(lines, "  and "+strconv.Itoa(blast.ObjectCount-20)+" more")
		}
	}
	multiNS := map[string]struct{}{}
	for _, group := range groups {
		multiNS[group.namespace] = struct{}{}
	}
	multi := len(multiNS) > 1
	for _, service := range blast.Services {
		lines = append(lines, serviceLine(blast, service, multi))
	}
	if blast.ServiceNote != "" {
		lines = append(lines, "  "+blast.ServiceNote)
	}
	for _, pdb := range blast.PDBs {
		word := "disruptions"
		if pdb.DisruptionsAllowed == 1 {
			word = "disruption"
		}
		lines = append(lines, "  pdb/"+pdb.Name+" allows "+strconv.Itoa(pdb.DisruptionsAllowed)+" "+word)
	}
	if blast.PDBNote != "" {
		lines = append(lines, "  "+blast.PDBNote)
	}
	for _, hpa := range blast.HPAs {
		lines = append(lines, "  hpa/"+hpa.Name+" keeps "+hpa.Target+" between "+strconv.Itoa(hpa.MinReplicas)+" and "+strconv.Itoa(hpa.MaxReplicas))
	}
	if blast.DryRun != nil && blast.DryRun.Status == "ok" {
		count := 0
		if blast.DryRun.Count != nil {
			count = *blast.DryRun.Count
		}
		lines = append(lines, "  dry-run: "+strconv.Itoa(count)+" objects")
	}
	if blast.DryRun != nil && blast.DryRun.Status == "unknown" {
		reason := blast.DryRun.Reason
		if reason == "" {
			reason = "failed"
		}
		lines = append(lines, "  dry-run: unknown ("+reason+")")
	}
	return strings.Join(lines, "\n")
}

func objectLine(blast Blast, object BlastObject) string {
	owned := ""
	for _, item := range blast.Owners {
		if item.Pod == object.Name && item.Namespace == object.Namespace {
			owned = " owned by " + item.Kind + "/" + item.Name
			break
		}
	}
	return "  " + object.Kind + "/" + object.Name + owned
}

func fromSuffix(blast Blast, namespace, kind string) string {
	if kind != "pod" || len(blast.Controllers) == 0 {
		return ""
	}
	var names []string
	for _, item := range blast.Controllers {
		if item.Namespace == namespace {
			names = append(names, item.Kind+"/"+item.Name)
		}
	}
	if len(names) == 0 {
		return ""
	}
	return ", from " + strings.Join(names, ", ")
}

func deletedPodsIn(blast Blast, namespace string) int {
	if blast.groups != nil {
		sum := 0
		for _, group := range blast.groups {
			if group.namespace == namespace && group.kind == "pod" {
				sum += group.count
			}
		}
		return sum
	}
	n := 0
	for _, object := range blast.Objects {
		if object.Kind == "pod" && object.Namespace == namespace {
			n++
		}
	}
	return n
}

func serviceLine(blast Blast, service BlastService, multi bool) string {
	where := ""
	if multi {
		where = " in namespace " + service.Namespace
	}
	deleted := deletedPodsIn(blast, service.Namespace)
	matched := "matches " + strconv.Itoa(service.MatchedPods)
	if service.MatchedPods == deleted && deleted > 1 {
		matched = "matches all " + strconv.Itoa(service.MatchedPods)
	}
	ready := "would keep " + strconv.Itoa(service.ReadyPodsRemaining) + " ready pods"
	if service.ReadyPodsRemaining == 0 {
		ready = "would have 0 ready pods"
	} else if service.ReadyPodsRemaining == 1 {
		ready = "would keep 1 ready pod"
	}
	return "  service/" + service.Name + where + " " + matched + " and " + ready
}

// FormatBlast renders a blast-radius report for the REPL.
func FormatBlast(blast Blast) string {
	if blast.Status == "skipped" && blast.Reason == "not a delete" {
		return ""
	}
	if blast.Status == "skipped" && blast.Reason == "no name or selector" {
		return "blast: not expanded (no name or selector)"
	}
	if blast.Status == "skipped" {
		return "blast: skipped (" + blast.Reason + ")"
	}
	if blast.Status == "unknown" {
		return "blast: unknown (" + blast.Reason + ")"
	}
	return formatOk(blast)
}

func itemsFromJSON(raw json.RawMessage, expectedKind string) ([]kubeObject, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, false
	}
	var doc struct {
		Kind     string `json:"kind"`
		Metadata *struct {
			Name string `json:"name"`
		} `json:"metadata"`
		Items []kubeObject `json:"items"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, false
	}
	if expectedKind != "" && doc.Kind == expectedKind && doc.Metadata != nil && doc.Metadata.Name != "" {
		var single kubeObject
		if err := json.Unmarshal(raw, &single); err != nil {
			return nil, false
		}
		return []kubeObject{single}, true
	}
	if expectedKind == "" && doc.Kind != "" && doc.Metadata != nil && doc.Metadata.Name != "" {
		var single kubeObject
		if err := json.Unmarshal(raw, &single); err != nil {
			return nil, false
		}
		return []kubeObject{single}, true
	}
	if doc.Kind == "" || (doc.Kind != "List" && !strings.HasSuffix(doc.Kind, "List")) {
		return nil, false
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return nil, false
	}
	itemsRaw, ok := probe["items"]
	if !ok || !jsonArray(itemsRaw) {
		return nil, false
	}
	var listItems []kubeObject
	if err := json.Unmarshal(itemsRaw, &listItems); err != nil {
		return nil, false
	}
	items := make([]kubeObject, 0, len(listItems))
	for _, object := range listItems {
		if object.Metadata == nil || object.Metadata.Name == "" {
			return nil, false
		}
		if expectedKind != "" {
			if object.Kind == "" {
				object.Kind = expectedKind
			}
			if object.Kind != expectedKind {
				return nil, false
			}
		} else if object.Kind == "" {
			return nil, false
		}
		items = append(items, object)
	}
	return items, true
}

type asked struct {
	namespace     string
	allNamespaces bool
	context       string
}

func askedOf(args []string) asked {
	var a asked
	for i := 0; i < len(args); i++ {
		token := args[i]
		if token == "-A" || token == "--all-namespaces" || token == "-A=true" || token == "--all-namespaces=true" {
			a.allNamespaces = true
		} else if strings.HasPrefix(token, "-n=") {
			a.namespace = token[len("-n="):]
		} else if (token == "-n" || token == "--namespace") && i+1 < len(args) {
			i++
			a.namespace = args[i]
		} else if strings.HasPrefix(token, "--namespace=") {
			a.namespace = token[len("--namespace="):]
		} else if token == "--context" && i+1 < len(args) {
			i++
			a.context = args[i]
		} else if strings.HasPrefix(token, "--context=") {
			a.context = token[len("--context="):]
		}
	}
	return a
}

func askedFlags(a asked) []string {
	var flags []string
	if a.namespace != "" {
		flags = append(flags, "-n", a.namespace)
	}
	if a.context != "" {
		flags = append(flags, "--context", a.context)
	}
	if a.allNamespaces {
		flags = append(flags, "-A")
	}
	return flags
}

func withContext(flags []string, context string) []string {
	if context == "" {
		return flags
	}
	return append(append([]string{}, flags...), "--context", context)
}

type target struct {
	token    string
	names    []string
	resource kube.Resource
	hasRes   bool
	selector string
	all      bool
}

func targetOf(args []string) (target, string) {
	rest := kube.PositionalWords(args).Words
	if len(rest) > 0 {
		rest = rest[1:]
	}
	selector, _ := kube.FlagValue(args, []string{"-l", "--selector"})
	all := false
	for _, token := range args {
		if token == "--all" || token == "--all=true" {
			all = true
			break
		}
	}
	var slash []string
	for _, word := range rest {
		if strings.Contains(word, "/") {
			slash = append(slash, word)
		}
	}
	if len(slash) > 0 {
		if len(slash) != len(rest) {
			return target{}, "mixed"
		}
		kinds := make([]string, len(slash))
		names := make([]string, len(slash))
		kindSet := map[string]struct{}{}
		for i, word := range slash {
			idx := strings.Index(word, "/")
			kinds[i] = word[:idx]
			names[i] = word[idx+1:]
			kindSet[kinds[i]] = struct{}{}
			if names[i] == "" {
				return target{}, "mixed"
			}
		}
		if len(kindSet) != 1 {
			return target{}, "mixed"
		}
		token := kinds[0]
		res, ok := kube.ResolveResource(token)
		return target{token: token, names: names, resource: res, hasRes: ok, all: false}, "ok"
	}
	if len(rest) == 0 {
		return target{}, "null"
	}
	token := rest[0]
	res, ok := kube.ResolveResource(token)
	names := append([]string{}, rest[1:]...)
	if !all && selector == "" && len(names) == 0 {
		return target{}, "null"
	}
	if all {
		return target{token: token, names: []string{}, resource: res, hasRes: ok, all: true}, "ok"
	}
	if selector != "" {
		return target{token: token, names: []string{}, resource: res, hasRes: ok, selector: selector, all: false}, "ok"
	}
	return target{token: token, names: names, resource: res, hasRes: ok, all: false}, "ok"
}

type podRef struct {
	namespace string
	name      string
	labels    map[string]string
	ready     bool
}

type fetch struct {
	getJSON func([]string) live.KubectlJSONResult
	now     func() int64
	started int64
}

func expired(f fetch) bool {
	return f.now()-f.started >= BlastBudgetMS
}

func jsonArgs(resource string, middle, scope []string) []string {
	args := []string{"get", resource}
	args = append(args, middle...)
	args = append(args, scope...)
	args = append(args, "-o", "json", "--request-timeout=4s")
	return args
}

func fetchJSON(f fetch, args []string) (live.KubectlJSONResult, bool) {
	if expired(f) {
		return live.KubectlJSONResult{}, false
	}
	result := f.getJSON(args)
	if expired(f) {
		return live.KubectlJSONResult{}, false
	}
	return result, true
}

func scopeFlags(namespaces []string) []string {
	if len(namespaces) > 1 {
		return []string{"-A"}
	}
	if len(namespaces) == 1 && namespaces[0] != "" {
		return []string{"-n", namespaces[0]}
	}
	return []string{}
}

func stringMap(value any) map[string]string {
	obj, ok := value.(map[string]any)
	if !ok || len(obj) == 0 {
		return nil
	}
	out := map[string]string{}
	for key, entry := range obj {
		s, ok := entry.(string)
		if !ok {
			return nil
		}
		out[key] = s
	}
	return out
}

func labelsMatch(selector, labels map[string]string) bool {
	for key, value := range selector {
		if labels[key] != value {
			return false
		}
	}
	return true
}

func makePodRef(item kubeObject, fallbackNS string) (podRef, bool) {
	if item.Metadata == nil || item.Metadata.Name == "" {
		return podRef{}, false
	}
	ns := item.Metadata.Namespace
	if ns == "" {
		ns = fallbackNS
	}
	labels := item.Metadata.Labels
	if labels == nil {
		labels = map[string]string{}
	}
	ready := false
	if item.Status != nil {
		for _, condition := range item.Status.Conditions {
			if condition.Type == "Ready" && condition.Status == "True" {
				ready = true
				break
			}
		}
	}
	return podRef{namespace: ns, name: item.Metadata.Name, labels: labels, ready: ready}, true
}

var controllers = map[string]bool{
	"Deployment": true, "StatefulSet": true, "DaemonSet": true, "ReplicaSet": true,
}

func selectorOf(item kubeObject) map[string]string {
	if item.Spec == nil || len(item.Spec.Selector) == 0 || string(item.Spec.Selector) == "null" {
		return nil
	}
	var raw any
	if err := json.Unmarshal(item.Spec.Selector, &raw); err != nil {
		return nil
	}
	obj, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	if exprs, ok := obj["matchExpressions"].([]any); ok && len(exprs) > 0 {
		return nil
	}
	return stringMap(obj["matchLabels"])
}

func labelSelector(labels map[string]string) string {
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+labels[key])
	}
	return strings.Join(parts, ",")
}

func expandControllers(items []kubeObject, controllerObjects []BlastObject, f fetch, a asked, fallbackNS, kind, namespace string) Blast {
	named := Blast{Controllers: controllerObjects}
	if len(named.Controllers) > 20 {
		named.Controllers = named.Controllers[:20]
	}
	var selectors []map[string]string
	namespaces := map[string]struct{}{}
	for _, item := range items {
		selector := selectorOf(item)
		if selector == nil {
			return withExtras(pack(controllerObjects, kind, namespace, named), nil, f, a)
		}
		selectors = append(selectors, selector)
		ns := fallbackNS
		if item.Metadata != nil && item.Metadata.Namespace != "" {
			ns = item.Metadata.Namespace
		}
		namespaces[ns] = struct{}{}
	}
	encoded := map[string]struct{}{}
	var firstEncoded string
	for _, selector := range selectors {
		enc := labelSelector(selector)
		if firstEncoded == "" {
			firstEncoded = enc
		}
		encoded[enc] = struct{}{}
	}
	if len(encoded) != 1 || (len(namespaces) > 1 && !a.allNamespaces) {
		return withExtras(pack(controllerObjects, kind, namespace, named), nil, f, a)
	}
	var scope []string
	if a.allNamespaces {
		scope = withContext([]string{"-A"}, a.context)
	} else {
		nsList := make([]string, 0, len(namespaces))
		for ns := range namespaces {
			nsList = append(nsList, ns)
		}
		scope = withContext(scopeFlags(nsList), a.context)
	}
	result, ok := fetchJSON(f, jsonArgs("pods", []string{"-l", firstEncoded}, scope))
	if !ok {
		return unknown("timed out")
	}
	if !result.OK {
		err := result.Error
		if err == "" {
			err = "failed"
		}
		return unknown("kubectl get pods: " + err)
	}
	podItems, ok := itemsFromJSON(result.JSON, "Pod")
	if !ok {
		return unknown("kubectl get pods: not a Pod object or list")
	}
	var objects []BlastObject
	var pods []podRef
	for _, item := range podItems {
		if item.Metadata == nil || item.Metadata.Name == "" {
			continue
		}
		ns := fallbackNS
		if item.Metadata.Namespace != "" {
			ns = item.Metadata.Namespace
		}
		objects = append(objects, BlastObject{Kind: "pod", Namespace: ns, Name: item.Metadata.Name})
		if pod, ok := makePodRef(item, fallbackNS); ok {
			pods = append(pods, pod)
		}
	}
	podBase := pack(objects, "pod", fallbackNS, named)
	withServices := attachServices(podBase, pods, f, fallbackNS, a.context, false)
	if withServices.Status != "ok" {
		return withServices
	}
	withServices = remember(withServices, podBase.kind, podBase.namespace, podBase.groups)
	return withExtras(withServices, pods, f, a)
}

func serviceNote(base Blast, errorMsg string) Blast {
	next := base
	next.Services = []BlastService{}
	if errorMsg == "" {
		errorMsg = "failed"
	}
	next.ServiceNote = "services unknown (" + errorMsg + ")"
	return next
}

func attachServices(base Blast, pods []podRef, f fetch, fallbackNS, context string, reuse bool) Blast {
	if len(pods) == 0 {
		return base
	}
	if len(pods) > 200 {
		next := base
		next.Services = []BlastService{}
		next.ServiceNote = "service check skipped (" + strconv.Itoa(len(pods)) + " pods exceeds 200)"
		return next
	}
	nsSet := map[string]struct{}{}
	var namespaces []string
	for _, pod := range pods {
		if _, ok := nsSet[pod.namespace]; !ok {
			nsSet[pod.namespace] = struct{}{}
			namespaces = append(namespaces, pod.namespace)
		}
	}
	if len(namespaces) > 5 {
		next := base
		next.Services = []BlastService{}
		next.ServiceNote = "service check skipped (" + strconv.Itoa(len(namespaces)) + " namespaces exceeds 5)"
		return next
	}
	services, ok := fetchJSON(f, jsonArgs("services", nil, withContext(scopeFlags(namespaces), context)))
	if !ok {
		return serviceNote(base, "timed out")
	}
	if !services.OK {
		return serviceNote(base, services.Error)
	}
	items, ok := itemsFromJSON(services.JSON, "Service")
	if !ok {
		return serviceNote(base, "not a Service object or list")
	}
	type matchedSvc struct {
		namespace   string
		name        string
		selector    map[string]string
		matchedPods int
	}
	var matched []matchedSvc
	for _, item := range items {
		var selector map[string]string
		if item.Spec != nil && len(item.Spec.Selector) > 0 && string(item.Spec.Selector) != "null" {
			var raw any
			if err := json.Unmarshal(item.Spec.Selector, &raw); err == nil {
				selector = stringMap(raw)
			}
		}
		if item.Metadata == nil {
			continue
		}
		name := item.Metadata.Name
		ns := item.Metadata.Namespace
		if ns == "" && len(namespaces) == 1 {
			ns = namespaces[0]
		}
		if selector == nil || name == "" || ns == "" {
			continue
		}
		matchedPods := 0
		for _, pod := range pods {
			if pod.namespace == ns && labelsMatch(selector, pod.labels) {
				matchedPods++
			}
		}
		if matchedPods == 0 {
			continue
		}
		matched = append(matched, matchedSvc{namespace: ns, name: name, selector: selector, matchedPods: matchedPods})
	}
	if len(matched) == 0 {
		return base
	}
	alive := pods
	if !reuse {
		svcNS := map[string]struct{}{}
		var serviceNamespaces []string
		for _, item := range matched {
			if _, ok := svcNS[item.namespace]; !ok {
				svcNS[item.namespace] = struct{}{}
				serviceNamespaces = append(serviceNamespaces, item.namespace)
			}
		}
		survivors, ok := fetchJSON(f, jsonArgs("pods", nil, withContext(scopeFlags(serviceNamespaces), context)))
		if !ok {
			return serviceNote(base, "timed out")
		}
		if !survivors.OK {
			return serviceNote(base, survivors.Error)
		}
		survivorItems, ok := itemsFromJSON(survivors.JSON, "Pod")
		if !ok {
			return serviceNote(base, "not a Pod object or list")
		}
		alive = nil
		for _, item := range survivorItems {
			if pod, ok := makePodRef(item, fallbackNS); ok {
				alive = append(alive, pod)
			}
		}
	}
	deleted := map[string]struct{}{}
	for _, pod := range pods {
		deleted[pod.namespace+"/"+pod.name] = struct{}{}
	}
	next := base
	next.Services = make([]BlastService, 0, len(matched))
	for _, item := range matched {
		ready := 0
		for _, pod := range alive {
			if pod.namespace != item.namespace {
				continue
			}
			if _, gone := deleted[pod.namespace+"/"+pod.name]; gone {
				continue
			}
			if labelsMatch(item.selector, pod.labels) && pod.ready {
				ready++
			}
		}
		next.Services = append(next.Services, BlastService{
			Namespace: item.namespace, Name: item.name,
			MatchedPods: item.matchedPods, ReadyPodsRemaining: ready,
		})
	}
	return next
}

func carry(next, prev Blast) Blast {
	next.groups = prev.groups
	next.kind = prev.kind
	next.namespace = prev.namespace
	return next
}

func podBudget(pods []podRef) bool {
	if len(pods) == 0 || len(pods) > 200 {
		return false
	}
	ns := map[string]struct{}{}
	for _, pod := range pods {
		ns[pod.namespace] = struct{}{}
	}
	return len(ns) <= 5
}

func withPdbs(blast Blast, pods []podRef, f fetch, a asked) Blast {
	if !podBudget(pods) {
		return blast
	}
	nsSet := map[string]struct{}{}
	var namespaces []string
	for _, pod := range pods {
		if _, ok := nsSet[pod.namespace]; !ok {
			nsSet[pod.namespace] = struct{}{}
			namespaces = append(namespaces, pod.namespace)
		}
	}
	result, ok := fetchJSON(f, jsonArgs("pdb", nil, withContext(scopeFlags(namespaces), a.context)))
	if !ok {
		next := blast
		next.PDBNote = "pdbs unknown (timed out)"
		return carry(next, blast)
	}
	if !result.OK {
		err := result.Error
		if err == "" {
			err = "failed"
		}
		next := blast
		next.PDBNote = "pdbs unknown (" + err + ")"
		return carry(next, blast)
	}
	items, ok := itemsFromJSON(result.JSON, "PodDisruptionBudget")
	if !ok {
		next := blast
		next.PDBNote = "pdbs unknown (not a PodDisruptionBudget object or list)"
		return carry(next, blast)
	}
	var pdbs []BlastPDB
	for _, item := range items {
		selector := selectorOf(item)
		if item.Metadata == nil {
			continue
		}
		name := item.Metadata.Name
		ns := item.Metadata.Namespace
		if ns == "" && len(namespaces) == 1 {
			ns = namespaces[0]
		}
		if selector == nil || name == "" || ns == "" || item.Status == nil || item.Status.DisruptionsAllowed == nil {
			continue
		}
		matched := false
		for _, pod := range pods {
			if pod.namespace == ns && labelsMatch(selector, pod.labels) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		pdbs = append(pdbs, BlastPDB{Namespace: ns, Name: name, DisruptionsAllowed: *item.Status.DisruptionsAllowed})
	}
	next := blast
	next.PDBs = pdbs
	return carry(next, blast)
}

func withHpas(blast Blast, f fetch, a asked) Blast {
	controllerList := blast.Controllers
	if len(controllerList) == 0 {
		return blast
	}
	nsSet := map[string]struct{}{}
	var namespaces []string
	for _, item := range controllerList {
		if _, ok := nsSet[item.Namespace]; !ok {
			nsSet[item.Namespace] = struct{}{}
			namespaces = append(namespaces, item.Namespace)
		}
	}
	if len(namespaces) > 5 {
		return blast
	}
	result, ok := fetchJSON(f, jsonArgs("hpa", nil, withContext(scopeFlags(namespaces), a.context)))
	if !ok || !result.OK {
		return blast
	}
	items, ok := itemsFromJSON(result.JSON, "HorizontalPodAutoscaler")
	if !ok {
		return blast
	}
	var hpas []BlastHPA
	for _, item := range items {
		if item.Spec == nil || item.Spec.ScaleTargetRef == nil || item.Metadata == nil {
			continue
		}
		ref := item.Spec.ScaleTargetRef
		name := item.Metadata.Name
		if ref.Name == "" || name == "" {
			continue
		}
		var targetCtrl *BlastObject
		refKind := strings.ToLower(ref.Kind)
		for i := range controllerList {
			controller := &controllerList[i]
			if controller.Name == ref.Name && controller.Kind == refKind {
				targetCtrl = controller
				break
			}
		}
		if targetCtrl == nil {
			continue
		}
		minReplicas := 1
		if item.Spec.MinReplicas != nil {
			minReplicas = *item.Spec.MinReplicas
		}
		maxReplicas := minReplicas
		if item.Spec.MaxReplicas != nil {
			maxReplicas = *item.Spec.MaxReplicas
		}
		ns := item.Metadata.Namespace
		if ns == "" {
			ns = targetCtrl.Namespace
		}
		hpas = append(hpas, BlastHPA{
			Namespace: ns, Name: name, MinReplicas: minReplicas, MaxReplicas: maxReplicas,
			Target: targetCtrl.Kind + "/" + targetCtrl.Name,
		})
	}
	if len(hpas) == 0 {
		return blast
	}
	next := blast
	next.HPAs = hpas
	return carry(next, blast)
}

func withExtras(blast Blast, pods []podRef, f fetch, a asked) Blast {
	return withHpas(withPdbs(blast, pods, f, a), f, a)
}

func dryRunArgs(args []string) []string {
	next := append([]string{}, args...)
	hasDry := false
	for _, token := range next {
		if token == "--dry-run" || strings.HasPrefix(token, "--dry-run=") {
			hasDry = true
			break
		}
	}
	if !hasDry {
		next = append(next, "--dry-run=server")
	}
	hasOut := false
	for _, token := range next {
		if token == "-o" || strings.HasPrefix(token, "--output") {
			hasOut = true
			break
		}
	}
	if !hasOut {
		next = append(next, "-o", "json")
	}
	hasTimeout := false
	for _, token := range next {
		if token == "--request-timeout" || strings.HasPrefix(token, "--request-timeout=") {
			hasTimeout = true
			break
		}
	}
	if !hasTimeout {
		next = append(next, "--request-timeout=4s")
	}
	return next
}

func jsonArray(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	return len(trimmed) > 0 && trimmed[0] == '['
}

func countOf(raw json.RawMessage) (int, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return 0, false
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return 0, false
	}
	if itemsRaw, ok := probe["items"]; ok && jsonArray(itemsRaw) {
		var items []json.RawMessage
		if err := json.Unmarshal(itemsRaw, &items); err != nil {
			return 0, false
		}
		return len(items), true
	}
	var doc struct {
		Metadata *struct {
			Name string `json:"name"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return 0, false
	}
	if doc.Metadata != nil && doc.Metadata.Name != "" {
		return 1, true
	}
	return 0, false
}

func withDryRun(blast Blast, args []string, f fetch) Blast {
	if len(args) == 0 || args[0] != "delete" || blast.Status == "skipped" {
		return blast
	}
	result, ok := fetchJSON(f, dryRunArgs(args))
	if !ok {
		next := blast
		next.DryRun = &BlastDryRun{Status: "unknown", Reason: "timed out"}
		return carry(next, blast)
	}
	if !result.OK {
		reason := result.Error
		if reason == "" {
			reason = "failed"
		}
		next := blast
		next.DryRun = &BlastDryRun{Status: "unknown", Reason: reason}
		return carry(next, blast)
	}
	count, ok := countOf(result.JSON)
	if !ok {
		next := blast
		next.DryRun = &BlastDryRun{Status: "unknown", Reason: "not a delete result"}
		return carry(next, blast)
	}
	next := blast
	c := count
	next.DryRun = &BlastDryRun{Status: "ok", Count: &c}
	return carry(next, blast)
}

func ownerOf(item kubeObject, namespace, name string) []BlastOwner {
	if item.Metadata == nil || len(item.Metadata.OwnerReferences) == 0 {
		return nil
	}
	var owner *struct {
		Kind       string `json:"kind"`
		Name       string `json:"name"`
		Controller bool   `json:"controller"`
	}
	for i := range item.Metadata.OwnerReferences {
		ref := &item.Metadata.OwnerReferences[i]
		if ref.Controller {
			owner = ref
			break
		}
	}
	if owner == nil {
		owner = &item.Metadata.OwnerReferences[0]
	}
	if owner.Kind == "" || owner.Name == "" {
		return nil
	}
	return []BlastOwner{{
		Namespace: namespace, Pod: name,
		Kind: strings.ToLower(owner.Kind), Name: owner.Name,
	}}
}

func fromItems(items []kubeObject, resource kube.Resource, hasRes bool, a asked, f fetch, fallbackNS string, reuse bool) Blast {
	var owners []BlastOwner
	var objects []BlastObject
	for _, item := range items {
		if item.Metadata == nil || item.Metadata.Name == "" {
			continue
		}
		kind := "object"
		if hasRes {
			kind = resource.Singular
		} else if item.Kind != "" {
			kind = strings.ToLower(item.Kind)
		}
		ns := item.Metadata.Namespace
		if ns == "" {
			ns = fallbackNS
		}
		if kind == "pod" {
			owners = append(owners, ownerOf(item, ns, item.Metadata.Name)...)
		}
		objects = append(objects, BlastObject{Kind: kind, Namespace: ns, Name: item.Metadata.Name})
	}
	singular := "object"
	if hasRes {
		singular = resource.Singular
	} else {
		allPod := len(objects) > 0
		for _, object := range objects {
			if object.Kind != "pod" {
				allPod = false
				break
			}
		}
		if allPod {
			singular = "pod"
		} else if len(objects) > 0 {
			singular = objects[0].Kind
		}
	}
	partialKind, partialNS := singular, fallbackNS
	if len(items) > 0 {
		allControllers := true
		for _, item := range items {
			if !controllers[item.Kind] {
				allControllers = false
				break
			}
		}
		if allControllers {
			return expandControllers(items, objects, f, a, fallbackNS, partialKind, partialNS)
		}
	}
	extra := Blast{}
	if len(owners) > 0 {
		extra.Owners = owners
	}
	base := pack(objects, partialKind, partialNS, extra)
	hasPodObject := false
	for _, object := range objects {
		if object.Kind == "pod" {
			hasPodObject = true
			break
		}
	}
	if singular != "pod" && !(!hasRes && hasPodObject) {
		return base
	}
	var pods []podRef
	for _, item := range items {
		isPod := item.Kind == "Pod" || (hasRes && resource.Singular == "pod")
		if !isPod {
			continue
		}
		if pod, ok := makePodRef(item, fallbackNS); ok {
			pods = append(pods, pod)
		}
	}
	withServices := attachServices(base, pods, f, fallbackNS, a.context, reuse)
	if withServices.Status != "ok" {
		return withServices
	}
	withServices = remember(withServices, base.kind, base.namespace, base.groups)
	return withExtras(withServices, pods, f, a)
}

func isKustomize(args []string) bool {
	for _, token := range args {
		name := token
		if eq := strings.Index(token, "="); eq >= 0 {
			name = token[:eq]
		}
		if name == "-k" || name == "--kustomize" {
			return true
		}
	}
	return false
}

func manifestFiles(args []string) []string {
	var files []string
	for i := 0; i < len(args); i++ {
		token := args[i]
		if token == "-f" || token == "--filename" {
			if i+1 < len(args) {
				i++
				files = append(files, args[i])
			}
		} else if strings.HasPrefix(token, "--filename=") {
			files = append(files, token[len("--filename="):])
		} else if strings.HasPrefix(token, "-f=") {
			files = append(files, token[len("-f="):])
		}
	}
	return files
}

func isDaemon(item kubeObject) bool {
	if item.Metadata == nil {
		return false
	}
	for _, owner := range item.Metadata.OwnerReferences {
		if owner.Kind == "DaemonSet" {
			return true
		}
	}
	return false
}

func blastFiles(files []string, plan kube.Plan, a asked, f fetch) Blast {
	fallbackNS := a.namespace
	if fallbackNS == "" {
		fallbackNS = plan.Namespace
	}
	type groupKey struct {
		kind string
		ns   string
	}
	groups := map[groupKey][]kube.ManifestRef{}
	var order []groupKey
	for _, file := range files {
		refs, errMsg, ok := kube.ManifestRefs(file)
		if !ok {
			return unknown("manifest: " + errMsg)
		}
		for _, ref := range refs {
			ns := ref.Namespace
			if ns == "" {
				ns = fallbackNS
			}
			key := groupKey{kind: strings.ToLower(ref.Kind), ns: ns}
			if _, exists := groups[key]; !exists {
				order = append(order, key)
			}
			ref.Namespace = ns
			groups[key] = append(groups[key], ref)
		}
	}
	if len(groups) == 0 {
		return skipped("no name or selector")
	}
	var items []kubeObject
	for _, key := range order {
		group := groups[key]
		token := group[0].Kind
		token = strings.ToLower(token)
		res, hasRes := kube.ResolveResource(token)
		ns := group[0].Namespace
		if ns == "" {
			ns = fallbackNS
		}
		names := make([]string, len(group))
		for i, ref := range group {
			names[i] = ref.Name
		}
		resourceToken := token
		if hasRes {
			resourceToken = res.Singular
		}
		result, ok := fetchJSON(f, jsonArgs(resourceToken, names, withContext([]string{"-n", ns}, a.context)))
		if !ok {
			if len(items) > 0 {
				return fromItems(items, kube.Resource{}, false, a, f, fallbackNS, false)
			}
			return unknown("timed out")
		}
		if !result.OK {
			if len(items) > 0 {
				return fromItems(items, kube.Resource{}, false, a, f, fallbackNS, false)
			}
			err := result.Error
			if err == "" {
				err = "failed"
			}
			return unknown("kubectl get " + token + ": " + err)
		}
		expectedKind := ""
		if hasRes {
			expectedKind = res.APIKind
		}
		parsed, ok := itemsFromJSON(result.JSON, expectedKind)
		if !ok {
			if len(items) > 0 {
				return fromItems(items, kube.Resource{}, false, a, f, fallbackNS, false)
			}
			kind := "object"
			if hasRes {
				kind = res.APIKind
			}
			return unknown("kubectl get " + token + ": not a " + kind + " object or list")
		}
		items = append(items, parsed...)
	}
	return fromItems(items, kube.Resource{}, false, a, f, fallbackNS, false)
}

func blastDrain(args []string, plan kube.Plan, f fetch, a asked) Blast {
	words := kube.PositionalWords(args).Words
	if len(words) < 2 {
		return skipped("no name or selector")
	}
	node := words[1]
	fallbackNS := a.namespace
	if fallbackNS == "" {
		fallbackNS = plan.Namespace
	}
	scope := []string{"-A"}
	if a.namespace != "" && !a.allNamespaces {
		scope = []string{"-n", a.namespace}
	}
	result, ok := fetchJSON(f, jsonArgs("pods", []string{"--field-selector", "spec.nodeName=" + node}, withContext(scope, a.context)))
	if !ok {
		return unknown("timed out")
	}
	if !result.OK {
		err := result.Error
		if err == "" {
			err = "failed"
		}
		return unknown("kubectl get pods: " + err)
	}
	parsed, ok := itemsFromJSON(result.JSON, "Pod")
	if !ok {
		return unknown("kubectl get pods: not a Pod object or list")
	}
	var filtered []kubeObject
	for _, item := range parsed {
		if !isDaemon(item) {
			filtered = append(filtered, item)
		}
	}
	res, _ := kube.ResolveResource("pod")
	return fromItems(filtered, res, true, a, f, fallbackNS, false)
}

// BlastRadius expands a mutating plan into the objects that would be affected.
func BlastRadius(plan kube.Plan, args []string, getJSON func([]string) live.KubectlJSONResult, now func() int64) Blast {
	if now == nil {
		now = func() int64 { return time.Now().UnixMilli() }
	}
	started := now()
	f := fetch{getJSON: getJSON, now: now, started: started}
	a := askedOf(args)
	scaleZero := false
	if plan.Verb == "scale" {
		if value, ok := kube.FlagValue(args, []string{"--replicas"}); ok && value == "0" {
			scaleZero = true
		}
	}
	if plan.Verb == "drain" {
		return blastDrain(args, plan, f, a)
	}
	if plan.Verb != "delete" && !scaleZero {
		return skipped("not a delete")
	}
	if isKustomize(args) {
		return skipped("kustomize delete (-k) is not expanded")
	}
	files := manifestFiles(args)
	for _, file := range files {
		lower := strings.ToLower(file)
		if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
			return skipped("remote manifest URLs are not expanded")
		}
	}
	if len(files) > 0 {
		return withDryRun(blastFiles(files, plan, a, f), args, f)
	}
	tgt, kind := targetOf(args)
	if kind == "mixed" {
		return skipped("mixed resource kinds")
	}
	if kind == "null" {
		return skipped("no name or selector")
	}
	var middle []string
	if tgt.selector != "" {
		middle = []string{"-l", tgt.selector}
	} else if tgt.all {
		middle = []string{}
	} else {
		middle = tgt.names
	}
	result, ok := fetchJSON(f, jsonArgs(tgt.token, middle, askedFlags(a)))
	if !ok {
		return unknown("timed out")
	}
	if !result.OK {
		err := result.Error
		if err == "" {
			err = "failed"
		}
		return unknown("kubectl get " + tgt.token + ": " + err)
	}
	expectedKind := ""
	if tgt.hasRes {
		expectedKind = tgt.resource.APIKind
	}
	items, ok := itemsFromJSON(result.JSON, expectedKind)
	if !ok {
		kindName := "object"
		if tgt.hasRes {
			kindName = tgt.resource.APIKind
		}
		return unknown("kubectl get " + tgt.token + ": not a " + kindName + " object or list")
	}
	fallbackNS := a.namespace
	if fallbackNS == "" {
		fallbackNS = plan.Namespace
	}
	reuse := tgt.all && (tgt.hasRes && tgt.resource.Singular == "pod" || tgt.token == "pods" || tgt.token == "pod")
	finished := fromItems(items, tgt.resource, tgt.hasRes, a, f, fallbackNS, reuse)
	return withDryRun(finished, args, f)
}
