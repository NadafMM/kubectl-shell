package plan

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/NadafMM/kubectl-shell/internal/kube"
	"github.com/NadafMM/kubectl-shell/internal/kube/live"
	"github.com/NadafMM/kubectl-shell/internal/kube/parse"
)

// BlastManagedBy flags an object managed by a GitOps or package controller.
type BlastManagedBy struct {
	Namespace string `json:"namespace,omitempty"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Manager   string `json:"manager"`
	Release   string `json:"release,omitempty"`
}

// BlastIngress flags an Ingress or Gateway HTTPRoute routing to an impacted Service.
type BlastIngress struct {
	Kind      string   `json:"kind,omitempty"`
	Namespace string   `json:"namespace"`
	Name      string   `json:"name"`
	Service   string   `json:"service"`
	Hosts     []string `json:"hosts,omitempty"`
}

// BlastPVC captures reclaim and mount state for a deleted PersistentVolumeClaim.
type BlastPVC struct {
	Namespace     string   `json:"namespace"`
	Name          string   `json:"name"`
	VolumeName    string   `json:"volumeName,omitempty"`
	ReclaimPolicy string   `json:"reclaimPolicy,omitempty"`
	MountedBy     []string `json:"mountedBy,omitempty"`
}

// BlastDrain summarizes remaining node headroom for a drain target.
type BlastDrain struct {
	Node                      string `json:"node"`
	EvictedPods               int    `json:"evictedPods"`
	SchedulableNodesRemaining int    `json:"schedulableNodesRemaining"`
	RequestedMilliCPU         int64  `json:"requestedMilliCpu,omitempty"`
	RemainingAllocMilliCPU    int64  `json:"remainingAllocMilliCpu,omitempty"`
	RequestedMemoryBytes      int64  `json:"requestedMemoryBytes,omitempty"`
	RemainingAllocMemoryBytes int64  `json:"remainingAllocMemoryBytes,omitempty"`
	EmptyDirPods              int    `json:"emptyDirPods,omitempty"`
	CapacityWarning           string `json:"capacityWarning,omitempty"`
}

// BlastDiff summarizes a preflight kubectl diff for apply/replace/patch.
type BlastDiff struct {
	Status  string `json:"status"`
	Added   int    `json:"added,omitempty"`
	Removed int    `json:"removed,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

// BlastOptions supplies optional preflight helpers beyond JSON reads.
type BlastOptions struct {
	RunDiff func(args []string) (code int, stdout, stderr string)
}

func detectGitOpsAndFinalizers(items []kubeObject, fallbackNS string) ([]BlastManagedBy, []string) {
	var managed []BlastManagedBy
	var warnings []string
	seenManaged := map[string]bool{}
	seenWarn := map[string]bool{}
	addWarn := func(msg string) {
		if msg == "" || seenWarn[msg] {
			return
		}
		seenWarn[msg] = true
		warnings = append(warnings, msg)
	}
	for _, item := range items {
		if item.Metadata == nil || item.Metadata.Name == "" {
			continue
		}
		ns := item.Metadata.Namespace
		if ns == "" {
			ns = fallbackNS
		}
		kind := strings.ToLower(item.Kind)
		if kind == "" {
			kind = "object"
		}
		manager, release := kube.DetectGitOpsFromMeta(item.Metadata.Labels, item.Metadata.Annotations)
		if manager != "" {
			key := ns + "/" + kind + "/" + item.Metadata.Name + "/" + manager
			if !seenManaged[key] {
				seenManaged[key] = true
				managed = append(managed, BlastManagedBy{
					Namespace: ns, Kind: kind, Name: item.Metadata.Name,
					Manager: manager, Release: release,
				})
			}
		}
		if len(item.Metadata.Finalizers) > 0 {
			addWarn(fmt.Sprintf("%s/%s has finalizers (%s); deletion may stay Terminating", kind, item.Metadata.Name, strings.Join(item.Metadata.Finalizers, ", ")))
		}
	}
	return managed, warnings
}

func withIngressImpact(blast Blast, items []kubeObject, f fetch, fallbackNS string, a asked) Blast {
	affected := map[string]bool{}
	nsSet := map[string]struct{}{}
	for _, svc := range blast.Services {
		if svc.ReadyPodsRemaining == 0 {
			affected[svc.Namespace+"/"+svc.Name] = true
			nsSet[svc.Namespace] = struct{}{}
		}
	}
	for _, item := range items {
		if strings.EqualFold(item.Kind, "Service") && item.Metadata != nil && item.Metadata.Name != "" {
			ns := item.Metadata.Namespace
			if ns == "" {
				ns = fallbackNS
			}
			affected[ns+"/"+item.Metadata.Name] = true
			nsSet[ns] = struct{}{}
		}
	}
	if len(affected) == 0 || len(nsSet) > 5 {
		return blast
	}
	namespaces := make([]string, 0, len(nsSet))
	for ns := range nsSet {
		namespaces = append(namespaces, ns)
	}
	var wg sync.WaitGroup
	var result, routeRes live.KubectlJSONResult
	var ok, routeOK bool
	wg.Add(2)
	go func() {
		defer wg.Done()
		result, ok = fetchJSON(f, jsonArgs("ingresses", nil, withContext(scopeFlags(namespaces), a.context)))
	}()
	go func() {
		defer wg.Done()
		routeRes, routeOK = fetchJSON(f, jsonArgs("httproutes", nil, withContext(scopeFlags(namespaces), a.context)))
	}()
	wg.Wait()

	var out []BlastIngress
	if ok && result.OK {
		if ingItems, iok := itemsFromJSON(result.JSON, "Ingress"); iok {
			for _, ing := range ingItems {
				if ing.Metadata == nil || ing.Metadata.Name == "" || ing.Spec == nil {
					continue
				}
				ns := ing.Metadata.Namespace
				if ns == "" && len(namespaces) == 1 {
					ns = namespaces[0]
				}
				svcHosts := map[string][]string{}
				addSvcHost := func(svc, host string) {
					if svc == "" || !affected[ns+"/"+svc] {
						return
					}
					if host == "" {
						host = "*"
					}
					for _, existing := range svcHosts[svc] {
						if existing == host {
							return
						}
					}
					svcHosts[svc] = append(svcHosts[svc], host)
				}
				if ing.Spec.DefaultBackend != nil && ing.Spec.DefaultBackend.Service != nil {
					addSvcHost(ing.Spec.DefaultBackend.Service.Name, "*")
				}
				for _, rule := range ing.Spec.Rules {
					if rule.HTTP == nil {
						continue
					}
					for _, p := range rule.HTTP.Paths {
						if p.Backend != nil && p.Backend.Service != nil {
							addSvcHost(p.Backend.Service.Name, rule.Host)
						}
					}
				}
				for svc, hosts := range svcHosts {
					out = append(out, BlastIngress{
						Kind: "ingress", Namespace: ns, Name: ing.Metadata.Name, Service: svc, Hosts: hosts,
					})
				}
			}
		}
	}
	if routeOK && routeRes.OK {
		out = append(out, matchHTTPRoutes(routeRes.JSON, namespaces, affected)...)
	}
	if len(out) == 0 {
		return blast
	}
	next := blast
	next.Ingresses = out
	return carry(next, blast)
}

type httpRouteDoc struct {
	Kind     string `json:"kind"`
	Metadata *struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Spec *struct {
		Hostnames []string `json:"hostnames"`
		Rules     []struct {
			BackendRefs []struct {
				Kind      string `json:"kind"`
				Name      string `json:"name"`
				Namespace string `json:"namespace"`
			} `json:"backendRefs"`
		} `json:"rules"`
	} `json:"spec"`
}

func matchHTTPRoutes(raw json.RawMessage, namespaces []string, affected map[string]bool) []BlastIngress {
	var list struct {
		Items []httpRouteDoc `json:"items"`
	}
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil
	}
	var out []BlastIngress
	for _, route := range list.Items {
		if route.Metadata == nil || route.Metadata.Name == "" || route.Spec == nil {
			continue
		}
		ns := route.Metadata.Namespace
		if ns == "" && len(namespaces) == 1 {
			ns = namespaces[0]
		}
		hosts := append([]string{}, route.Spec.Hostnames...)
		if len(hosts) == 0 {
			hosts = []string{"*"}
		}
		seenSvc := map[string]bool{}
		for _, rule := range route.Spec.Rules {
			for _, ref := range rule.BackendRefs {
				if ref.Kind != "" && !strings.EqualFold(ref.Kind, "Service") {
					continue
				}
				refNS := ref.Namespace
				if refNS == "" {
					refNS = ns
				}
				if ref.Name == "" || !affected[refNS+"/"+ref.Name] || seenSvc[refNS+"/"+ref.Name] {
					continue
				}
				seenSvc[refNS+"/"+ref.Name] = true
				out = append(out, BlastIngress{
					Kind: "httproute", Namespace: ns, Name: route.Metadata.Name, Service: ref.Name, Hosts: hosts,
				})
			}
		}
	}
	return out
}

func withPVCImpact(blast Blast, items []kubeObject, f fetch, fallbackNS string, a asked) Blast {
	var pvcItems []kubeObject
	nsSet := map[string]struct{}{}
	for _, item := range items {
		if strings.EqualFold(item.Kind, "PersistentVolumeClaim") && item.Metadata != nil && item.Metadata.Name != "" {
			pvcItems = append(pvcItems, item)
			ns := item.Metadata.Namespace
			if ns == "" {
				ns = fallbackNS
			}
			nsSet[ns] = struct{}{}
		}
	}
	if len(pvcItems) == 0 || len(nsSet) > 5 {
		return blast
	}
	namespaces := make([]string, 0, len(nsSet))
	for ns := range nsSet {
		namespaces = append(namespaces, ns)
	}
	var wg sync.WaitGroup
	var podsRes live.KubectlJSONResult
	var podsOK bool
	var pvRes live.KubectlJSONResult
	var pvOK bool

	needPV := false
	for _, pvc := range pvcItems {
		if pvc.Spec != nil && pvc.Spec.VolumeName != "" {
			needPV = true
			break
		}
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		podsRes, podsOK = fetchJSON(f, jsonArgs("pods", nil, withContext(scopeFlags(namespaces), a.context)))
	}()
	if needPV {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pvRes, pvOK = fetchJSON(f, jsonArgs("pv", nil, withContext(nil, a.context)))
		}()
	}
	wg.Wait()

	reclaimByPV := map[string]string{}
	if pvOK && pvRes.OK {
		if pvs, ok := itemsFromJSON(pvRes.JSON, "PersistentVolume"); ok {
			for _, pv := range pvs {
				if pv.Metadata != nil && pv.Spec != nil && pv.Spec.PersistentVolumeReclaimPolicy != "" {
					reclaimByPV[pv.Metadata.Name] = pv.Spec.PersistentVolumeReclaimPolicy
				}
			}
		}
	}
	mounts := map[string][]string{}
	if podsOK && podsRes.OK {
		if pods, ok := itemsFromJSON(podsRes.JSON, "Pod"); ok {
			for _, pod := range pods {
				if pod.Metadata == nil || pod.Spec == nil {
					continue
				}
				if pod.Status != nil && (pod.Status.Phase == "Succeeded" || pod.Status.Phase == "Failed") {
					continue
				}
				ns := pod.Metadata.Namespace
				if ns == "" && len(namespaces) == 1 {
					ns = namespaces[0]
				}
				for _, vol := range pod.Spec.Volumes {
					if vol.PersistentVolumeClaim != nil && vol.PersistentVolumeClaim.ClaimName != "" {
						key := ns + "/" + vol.PersistentVolumeClaim.ClaimName
						mounts[key] = append(mounts[key], pod.Metadata.Name)
					}
				}
			}
		}
	}
	out := make([]BlastPVC, 0, len(pvcItems))
	for _, pvc := range pvcItems {
		ns := pvc.Metadata.Namespace
		if ns == "" {
			ns = fallbackNS
		}
		vol := ""
		if pvc.Spec != nil {
			vol = pvc.Spec.VolumeName
		}
		out = append(out, BlastPVC{
			Namespace:     ns,
			Name:          pvc.Metadata.Name,
			VolumeName:    vol,
			ReclaimPolicy: reclaimByPV[vol],
			MountedBy:     mounts[ns+"/"+pvc.Metadata.Name],
		})
	}
	next := blast
	next.PVCs = out
	return carry(next, blast)
}

func parseMilliCPU(raw string) int64 {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0
	}
	if strings.HasSuffix(s, "m") {
		v, err := strconv.ParseInt(strings.TrimSuffix(s, "m"), 10, 64)
		if err == nil {
			return v
		}
		return 0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err == nil {
		return int64(f * 1000)
	}
	return 0
}

func parseMemoryBytes(raw string) int64 {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0
	}
	units := []struct {
		suffix string
		mult   int64
	}{
		{"Ki", 1024},
		{"Mi", 1024 * 1024},
		{"Gi", 1024 * 1024 * 1024},
		{"Ti", 1024 * 1024 * 1024 * 1024},
		{"K", 1000},
		{"M", 1000 * 1000},
		{"G", 1000 * 1000 * 1000},
		{"T", 1000 * 1000 * 1000 * 1000},
	}
	for _, u := range units {
		if strings.HasSuffix(s, u.suffix) {
			num := strings.TrimSuffix(s, u.suffix)
			if v, err := strconv.ParseInt(num, 10, 64); err == nil {
				return v * u.mult
			}
			if f, err := strconv.ParseFloat(num, 64); err == nil {
				return int64(f * float64(u.mult))
			}
			return 0
		}
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err == nil {
		return v
	}
	return 0
}

func formatMi(bytes int64) string {
	return strconv.FormatInt(bytes/(1024*1024), 10) + "Mi"
}

func evaluateDrainCapacity(node string, evicted []kubeObject, nodesRes live.KubectlJSONResult, nodesOK bool) *BlastDrain {
	var reqCPU, reqMem int64
	emptyDirPods := 0
	for _, pod := range evicted {
		if pod.Spec == nil {
			continue
		}
		hasLocal := false
		for _, v := range pod.Spec.Volumes {
			if v.EmptyDir != nil || v.HostPath != nil {
				hasLocal = true
				break
			}
		}
		if hasLocal {
			emptyDirPods++
		}
		for _, c := range pod.Spec.Containers {
			if c.Resources != nil && c.Resources.Requests != nil {
				reqCPU += parseMilliCPU(c.Resources.Requests["cpu"])
				reqMem += parseMemoryBytes(c.Resources.Requests["memory"])
			}
		}
	}
	report := &BlastDrain{
		Node:                 node,
		EvictedPods:          len(evicted),
		RequestedMilliCPU:    reqCPU,
		RequestedMemoryBytes: reqMem,
		EmptyDirPods:         emptyDirPods,
	}
	if !nodesOK || !nodesRes.OK {
		return report
	}
	nodes, ok := itemsFromJSON(nodesRes.JSON, "Node")
	if !ok {
		return report
	}
	var remNodes int
	var remCPU, remMem int64
	for _, n := range nodes {
		if n.Metadata == nil || n.Metadata.Name == node {
			continue
		}
		if n.Spec != nil && n.Spec.Unschedulable {
			continue
		}
		ready := false
		if n.Status != nil {
			for _, cond := range n.Status.Conditions {
				if cond.Type == "Ready" && cond.Status == "True" {
					ready = true
					break
				}
			}
			if n.Status.Allocatable != nil {
				remCPU += parseMilliCPU(n.Status.Allocatable["cpu"])
				remMem += parseMemoryBytes(n.Status.Allocatable["memory"])
			}
		}
		if ready {
			remNodes++
		}
	}
	report.SchedulableNodesRemaining = remNodes
	report.RemainingAllocMilliCPU = remCPU
	report.RemainingAllocMemoryBytes = remMem
	switch {
	case remNodes == 0 && len(evicted) > 0:
		report.CapacityWarning = "no remaining Ready schedulable nodes to absorb evicted pods"
	case reqCPU > 0 && remCPU > 0 && reqCPU > remCPU:
		report.CapacityWarning = fmt.Sprintf("evicted CPU requests (%dm) exceed total remaining node allocatable (%dm)", reqCPU, remCPU)
	case reqMem > 0 && remMem > 0 && reqMem > remMem:
		report.CapacityWarning = fmt.Sprintf("evicted memory requests (%s) exceed total remaining node allocatable (%s)", formatMi(reqMem), formatMi(remMem))
	}
	return report
}

func blastMutatePreflight(plan kube.Plan, args []string, f fetch, a asked, opts BlastOptions) Blast {
	if isKustomize(args) {
		return skipped("kustomize (-k) is not expanded")
	}
	files := manifestFiles(args)
	for _, file := range files {
		lower := strings.ToLower(file)
		if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
			return skipped("remote manifest URLs are not expanded")
		}
	}
	fallbackNS := a.namespace
	if fallbackNS == "" {
		fallbackNS = plan.Namespace
	}
	if fallbackNS == "" {
		fallbackNS = "default"
	}

	var refs []parse.ManifestRef
	if len(files) > 0 {
		for _, file := range files {
			parsed, errMsg, ok := parse.ManifestRefs(file)
			if !ok {
				return unknown("manifest: " + errMsg)
			}
			for _, ref := range parsed {
				if ref.Namespace == "" {
					ref.Namespace = fallbackNS
				}
				refs = append(refs, ref)
			}
		}
	} else if plan.Verb == "patch" {
		words := parse.PositionalWords(args).Words
		if len(words) >= 2 {
			token := words[1]
			if strings.Contains(token, "/") {
				parts := strings.SplitN(token, "/", 2)
				refs = append(refs, parse.ManifestRef{Kind: parts[0], Name: parts[1], Namespace: fallbackNS})
			} else if len(words) >= 3 {
				refs = append(refs, parse.ManifestRef{Kind: token, Name: words[2], Namespace: fallbackNS})
			}
		}
	}
	if len(refs) == 0 {
		return skipped("no name or selector")
	}

	var objects []BlastObject
	var liveItems []kubeObject
	var warnings []string
	var diff *BlastDiff

	var wg sync.WaitGroup
	var mu sync.Mutex

	// Group live gets by (kind, namespace)
	type gk struct{ kind, ns string }
	grouped := map[gk][]string{}
	var order []gk
	for _, ref := range refs {
		res, ok := parse.ResolveResource(strings.ToLower(ref.Kind))
		kind := strings.ToLower(ref.Kind)
		if ok {
			kind = res.Singular
		}
		objects = append(objects, BlastObject{Kind: kind, Namespace: ref.Namespace, Name: ref.Name})
		k := gk{kind: kind, ns: ref.Namespace}
		if _, exists := grouped[k]; !exists {
			order = append(order, k)
		}
		grouped[k] = append(grouped[k], ref.Name)
	}

	for _, k := range order {
		k := k
		names := grouped[k]
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, ok := fetchJSON(f, jsonArgs(k.kind, names, withContext([]string{"-n", k.ns}, a.context)))
			if !ok || !res.OK {
				return
			}
			if items, iok := itemsFromJSON(res.JSON, ""); iok {
				mu.Lock()
				liveItems = append(liveItems, items...)
				mu.Unlock()
			}
		}()
	}

	// ConfigMap/Secret reference check against namespace workloads.
	cfgNames := map[string]string{}
	cfgNS := map[string]bool{}
	for _, obj := range objects {
		if obj.Kind == "configmap" || obj.Kind == "secret" {
			cfgNames[obj.Kind+"/"+obj.Name] = obj.Namespace
			cfgNS[obj.Namespace] = true
		}
	}
	if len(cfgNames) > 0 && len(cfgNS) <= 5 {
		nsList := make([]string, 0, len(cfgNS))
		for ns := range cfgNS {
			nsList = append(nsList, ns)
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, ok := fetchJSON(f, jsonArgs("deployments.apps,statefulsets.apps", nil, withContext(scopeFlags(nsList), a.context)))
			if !ok || !res.OK {
				return
			}
			workloads, wok := itemsFromJSON(res.JSON, "")
			if !wok {
				return
			}
			for _, w := range workloads {
				if w.Metadata == nil || w.Spec == nil || w.Spec.Template == nil || w.Spec.Template.Spec == nil {
					continue
				}
				wns := w.Metadata.Namespace
				if wns == "" && len(nsList) == 1 {
					wns = nsList[0]
				}
				wkind := strings.ToLower(w.Kind)
				refsCfg := workloadConfigRefs(w.Spec.Template.Spec)
				for _, r := range refsCfg {
					if targetNS, hit := cfgNames[r]; hit && targetNS == wns {
						mu.Lock()
						warnings = append(warnings, fmt.Sprintf("updating %s does not auto-restart referencing %s/%s", r, wkind, w.Metadata.Name))
						mu.Unlock()
					}
				}
			}
		}()
	}

	// Immutable field heuristics for patch -p
	if plan.Verb == "patch" {
		if p, ok := parse.FlagValue(args, []string{"-p", "--patch"}); ok {
			if strings.Contains(p, `"selector"`) {
				warnings = append(warnings, "patching selector on an existing Deployment/StatefulSet fails as immutable")
			}
			if strings.Contains(p, `"clusterIP"`) {
				warnings = append(warnings, "patching Service clusterIP is immutable")
			}
		}
	}

	if opts.RunDiff != nil && (plan.Verb == "apply" || plan.Verb == "replace") && len(files) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			diffArgs := append([]string{"diff"}, args[1:]...)
			code, stdout, stderr := opts.RunDiff(diffArgs)
			switch code {
			case 0:
				diff = &BlastDiff{Status: "unchanged"}
			case 1:
				added, removed := countDiffLines(stdout)
				diff = &BlastDiff{Status: "changed", Added: added, Removed: removed}
			default:
				reason := live.StderrFirstLine(stderr)
				if reason == "" {
					reason = "diff unavailable"
				}
				diff = &BlastDiff{Status: "unknown", Reason: reason}
			}
		}()
	}

	wg.Wait()
	managed, finalizerWarns := detectGitOpsAndFinalizers(liveItems, fallbackNS)
	// Finalizers only block delete; drop finalizer warnings for apply/patch.
	_ = finalizerWarns

	kindLabel := objects[0].Kind
	for _, o := range objects[1:] {
		if o.Kind != kindLabel {
			kindLabel = "object"
			break
		}
	}
	extra := Blast{
		ManagedBy: managed,
		Warnings:  warnings,
		Diff:      diff,
	}
	return pack(objects, kindLabel, fallbackNS, extra)
}

func workloadConfigRefs(spec *podTemplatePodSpec) []string {
	if spec == nil {
		return nil
	}
	seen := map[string]bool{}
	var refs []string
	add := func(kind, name string) {
		if name == "" {
			return
		}
		k := kind + "/" + name
		if !seen[k] {
			seen[k] = true
			refs = append(refs, k)
		}
	}
	for _, v := range spec.Volumes {
		if v.ConfigMap != nil {
			add("configmap", v.ConfigMap.Name)
		}
		if v.Secret != nil {
			add("secret", v.Secret.SecretName)
		}
	}
	for _, c := range spec.Containers {
		for _, ef := range c.EnvFrom {
			if ef.ConfigMapRef != nil {
				add("configmap", ef.ConfigMapRef.Name)
			}
			if ef.SecretRef != nil {
				add("secret", ef.SecretRef.Name)
			}
		}
		for _, e := range c.Env {
			if e.ValueFrom != nil {
				if e.ValueFrom.ConfigMapKeyRef != nil {
					add("configmap", e.ValueFrom.ConfigMapKeyRef.Name)
				}
				if e.ValueFrom.SecretKeyRef != nil {
					add("secret", e.ValueFrom.SecretKeyRef.Name)
				}
			}
		}
	}
	return refs
}

func countDiffLines(stdout string) (added, removed int) {
	for _, line := range strings.Split(stdout, "\n") {
		if strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---") || strings.HasPrefix(line, "@@") {
			continue
		}
		if strings.HasPrefix(line, "+") {
			added++
		} else if strings.HasPrefix(line, "-") {
			removed++
		}
	}
	return added, removed
}

func mergeBlastExtras(dst, src Blast) Blast {
	if len(src.Services) > 0 || src.ServiceNote != "" {
		dst.Services = src.Services
		dst.ServiceNote = src.ServiceNote
	}
	if len(src.PDBs) > 0 || src.PDBNote != "" {
		dst.PDBs = src.PDBs
		dst.PDBNote = src.PDBNote
	}
	if len(src.HPAs) > 0 {
		dst.HPAs = src.HPAs
	}
	if len(src.Ingresses) > 0 {
		dst.Ingresses = src.Ingresses
	}
	if len(src.PVCs) > 0 {
		dst.PVCs = src.PVCs
	}
	if src.Drain != nil {
		dst.Drain = src.Drain
	}
	if len(src.ManagedBy) > 0 {
		dst.ManagedBy = append(dst.ManagedBy, src.ManagedBy...)
	}
	if len(src.Warnings) > 0 {
		dst.Warnings = append(dst.Warnings, src.Warnings...)
	}
	if src.Diff != nil {
		dst.Diff = src.Diff
	}
	if src.DryRun != nil {
		dst.DryRun = src.DryRun
	}
	return dst
}
