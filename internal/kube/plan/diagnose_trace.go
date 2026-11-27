package plan

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/NadafMM/kubectl-shell/internal/kube"
)

const (
	FindingService       FindingRule = "service"
	FindingPortMismatch  FindingRule = "port"
	FindingNetworkPolicy FindingRule = "networkpolicy"
	FindingIngress       FindingRule = "ingress"
	FindingHTTPRoute     FindingRule = "httproute"
	FindingRollout       FindingRule = "rollout"
	FindingQuota         FindingRule = "quota"
)

var (
	serviceQuestionRe = regexp.MustCompile(`(?i)\b(svc|service|endpoints?|ingress|httproute|503|502|traffic|reach|connect)\b|\bsvc/[a-z0-9][-a-z0-9.]*|\bservice/[a-z0-9][-a-z0-9.]*`)
	rolloutQuestionRe = regexp.MustCompile(`(?i)\b(deploy(ment)?|rollout|stuck|progress(ing)?|replicas?|sts|statefulset)\b|\bdeploy(ment)?/[a-z0-9][-a-z0-9.]*|\bsts/[a-z0-9][-a-z0-9.]*`)
)

func serviceNameFrom(question string, cluster kube.Cluster) string {
	if m := regexp.MustCompile(`(?i)\b(?:svc|service)/([a-z0-9][-a-z0-9.]*)`).FindStringSubmatch(question); len(m) == 2 {
		return m[1]
	}
	if m := regexp.MustCompile(`(?i)\b(?:svc|service)\s+([a-z0-9][-a-z0-9.]*)`).FindStringSubmatch(question); len(m) == 2 {
		if !askStop[strings.ToLower(m[1])] {
			return m[1]
		}
	}
	q := strings.ToLower(question)
	for _, item := range cluster.Objects {
		if item.Kind == "service" && len(item.Name) >= 3 && strings.Contains(q, strings.ToLower(item.Name)) {
			return item.Name
		}
	}
	return ""
}

func deploymentNameFrom(question string, cluster kube.Cluster) string {
	if m := regexp.MustCompile(`(?i)\b(?:deploy|deployment)/([a-z0-9][-a-z0-9.]*)`).FindStringSubmatch(question); len(m) == 2 {
		return m[1]
	}
	if m := regexp.MustCompile(`(?i)\b(?:deploy|deployment)\s+([a-z0-9][-a-z0-9.]*)`).FindStringSubmatch(question); len(m) == 2 {
		if !askStop[strings.ToLower(m[1])] {
			return m[1]
		}
	}
	q := strings.ToLower(question)
	for _, item := range cluster.Objects {
		if item.Kind == "deployment" && len(item.Name) >= 3 && strings.Contains(q, strings.ToLower(item.Name)) {
			return item.Name
		}
	}
	return ""
}

func dictLabelsMatch(selector, labels map[string]any) bool {
	if len(selector) == 0 {
		return false
	}
	for k, v := range selector {
		if asStr(labels[k]) != asStr(v) {
			return false
		}
	}
	return true
}

func podMatchesSelector(pod dict, selector map[string]any) bool {
	return dictLabelsMatch(selector, asDict(asDict(pod["metadata"])["labels"]))
}

func podIsReady(pod dict) bool {
	for _, cond := range asList(asDict(pod["status"])["conditions"]) {
		if asStr(cond["type"]) == "Ready" && asStr(cond["status"]) == "True" {
			return true
		}
	}
	return false
}

func targetPortMatchesContainers(svc dict, pods []dict) (mismatch string, ok bool) {
	ports := asList(asDict(svc["spec"])["ports"])
	if len(ports) == 0 || len(pods) == 0 {
		return "", true
	}
	// Check whether matching pods declare explicit containerPorts.
	hasDeclared := false
	declaredNums := map[string]bool{}
	declaredNames := map[string]bool{}
	for _, pod := range pods {
		for _, c := range containersOf(pod) {
			for _, cp := range asList(c["ports"]) {
				hasDeclared = true
				if n, nok := asNum(cp["containerPort"]); nok {
					declaredNums[trimFloat(n)] = true
				}
				if cname := asStr(cp["name"]); cname != "" {
					declaredNames[cname] = true
				}
			}
		}
	}
	if !hasDeclared {
		return "", true
	}
	for _, sp := range ports {
		tp := sp["targetPort"]
		if tp == nil {
			tp = sp["port"]
		}
		switch v := tp.(type) {
		case string:
			if v != "" && !declaredNames[v] {
				if _, err := strconv.Atoi(v); err != nil {
					return fmt.Sprintf("service targetPort %q does not match any containerPort name", v), false
				}
				if !declaredNums[v] {
					return fmt.Sprintf("service targetPort %s does not match any containerPort", v), false
				}
			}
		default:
			if n, nok := asNum(tp); nok {
				numStr := trimFloat(n)
				if !declaredNums[numStr] {
					return fmt.Sprintf("service targetPort %s does not match any containerPort", numStr), false
				}
			}
		}
	}
	return "", true
}

func traceServiceFromObjects(svc dict, ns string, pods []dict, ingresses []dict, netpols []dict, fetchJSON FetchJSON, fetchText FetchText) *Diagnosis {
	return traceServiceWithRoutes(svc, ns, pods, ingresses, nil, netpols, fetchJSON, fetchText)
}

func traceServiceWithRoutes(svc dict, ns string, pods []dict, ingresses []dict, httpRoutes []dict, netpols []dict, fetchJSON FetchJSON, fetchText FetchText) *Diagnosis {
	name := asStr(asDict(svc["metadata"])["name"])
	if name == "" {
		return nil
	}
	spec := asDict(svc["spec"])
	selector := asDict(spec["selector"])
	var findings []Finding

	if len(selector) == 0 && asStr(spec["type"]) != "ExternalName" {
		findings = append(findings, Finding{
			Rule:     FindingService,
			Cause:    "service/" + name + " has no pod selector (endpoints must be managed manually)",
			Commands: []string{"describe svc " + name + " -n " + ns, "get endpoints " + name + " -n " + ns},
		})
	} else {
		var matched []dict
		var readyPods []dict
		for _, pod := range pods {
			if podMatchesSelector(pod, selector) {
				matched = append(matched, pod)
				if podIsReady(pod) {
					readyPods = append(readyPods, pod)
				}
			}
		}
		if len(matched) == 0 {
			findings = append(findings, Finding{
				Rule:     FindingService,
				Cause:    "service/" + name + " selector matches 0 pods in namespace " + ns,
				Commands: []string{"describe svc " + name + " -n " + ns, "get pods --show-labels -n " + ns},
			})
		} else {
			if len(readyPods) == 0 {
				var podCause string
				for _, pod := range matched {
					if diag := diagnoseFetched(pod, ns, fetchJSON, fetchText); diag != nil && len(diag.Findings) > 0 {
						podCause = " (" + diag.Pod + ": " + diag.Findings[0].Cause + ")"
						break
					}
				}
				findings = append(findings, Finding{
					Rule:     FindingService,
					Cause:    fmt.Sprintf("service/%s matches %d pod(s) but 0 are Ready%s", name, len(matched), podCause),
					Commands: []string{"get endpointslices -l kubernetes.io/service-name=" + name + " -n " + ns, "get pods -n " + ns},
				})
			}
			if msg, ok := targetPortMatchesContainers(svc, matched); !ok {
				findings = append(findings, Finding{
					Rule:     FindingPortMismatch,
					Cause:    "service/" + name + ": " + msg,
					Commands: []string{"get svc " + name + " -o yaml -n " + ns},
				})
			}
		}

		// Check restrictive NetworkPolicies targeting the service's pods.
		for _, np := range netpols {
			npName := asStr(asDict(np["metadata"])["name"])
			npSpec := asDict(np["spec"])
			if npSpec == nil {
				continue
			}
			podSel := asDict(asDict(npSpec["podSelector"])["matchLabels"])
			selectsServicePods := len(podSel) == 0
			if !selectsServicePods {
				for _, pod := range matched {
					if podMatchesSelector(pod, podSel) {
						selectsServicePods = true
						break
					}
				}
			}
			if !selectsServicePods {
				continue
			}
			blocksIngress := false
			if pts, ok := npSpec["policyTypes"].([]any); ok {
				for _, pt := range pts {
					if asStr(pt) == "Ingress" {
						blocksIngress = true
						break
					}
				}
			}
			ingressRules := asList(npSpec["ingress"])
			if blocksIngress && len(ingressRules) == 0 {
				findings = append(findings, Finding{
					Rule:     FindingNetworkPolicy,
					Cause:    "networkpolicy/" + npName + " denies all ingress to service/" + name + " pods",
					Commands: []string{"describe networkpolicy " + npName + " -n " + ns},
				})
			}
		}
	}

	// Check Ingress port/service references pointing to this service.
	svcPorts := map[string]bool{}
	svcPortNames := map[string]bool{}
	for _, sp := range asList(spec["ports"]) {
		if n, nok := asNum(sp["port"]); nok {
			svcPorts[trimFloat(n)] = true
		}
		if pname := asStr(sp["name"]); pname != "" {
			svcPortNames[pname] = true
		}
	}
	for _, ing := range ingresses {
		ingName := asStr(asDict(ing["metadata"])["name"])
		ingSpec := asDict(ing["spec"])
		for _, rule := range asList(ingSpec["rules"]) {
			for _, p := range asList(asDict(rule["http"])["paths"]) {
				svcBackend := asDict(asDict(p["backend"])["service"])
				if asStr(svcBackend["name"]) != name {
					continue
				}
				portObj := asDict(svcBackend["port"])
				if n, nok := asNum(portObj["number"]); nok && !svcPorts[trimFloat(n)] {
					findings = append(findings, Finding{
						Rule:     FindingIngress,
						Cause:    fmt.Sprintf("ingress/%s targets service/%s port %s, which is not exposed by the Service", ingName, name, trimFloat(n)),
						Commands: []string{"describe ingress " + ingName + " -n " + ns, "get svc " + name + " -o yaml -n " + ns},
					})
				} else if pname := asStr(portObj["name"]); pname != "" && !svcPortNames[pname] {
					findings = append(findings, Finding{
						Rule:     FindingIngress,
						Cause:    fmt.Sprintf("ingress/%s targets service/%s port %q, which is not defined on the Service", ingName, name, pname),
						Commands: []string{"describe ingress " + ingName + " -n " + ns, "get svc " + name + " -o yaml -n " + ns},
					})
				}
			}
		}
	}

	// Check Gateway API HTTPRoute backendRefs pointing to this service.
	for _, route := range httpRoutes {
		routeName := asStr(asDict(route["metadata"])["name"])
		for _, rule := range asList(asDict(route["spec"])["rules"]) {
			for _, ref := range asList(rule["backendRefs"]) {
				kind := asStr(ref["kind"])
				if kind != "" && !strings.EqualFold(kind, "Service") {
					continue
				}
				refNS := asStr(ref["namespace"])
				if refNS == "" {
					refNS = ns
				}
				if asStr(ref["name"]) != name || refNS != ns {
					continue
				}
				if n, nok := asNum(ref["port"]); nok && !svcPorts[trimFloat(n)] {
					findings = append(findings, Finding{
						Rule:     FindingHTTPRoute,
						Cause:    fmt.Sprintf("httproute/%s targets service/%s port %s, which is not exposed by the Service", routeName, name, trimFloat(n)),
						Commands: []string{"describe httproute " + routeName + " -n " + ns, "get svc " + name + " -o yaml -n " + ns},
					})
				}
			}
		}
	}

	if len(findings) == 0 {
		return nil
	}
	if len(findings) > 3 {
		findings = findings[:3]
	}
	return &Diagnosis{Pod: "service/" + name, Namespace: ns, Findings: findings}
}

func traceService(input InspectPodsInput, svcName string) *Diagnosis {
	ns := input.Namespace
	if ns == "" {
		ns = "default"
	}
	var svc dict
	if svcName != "" {
		ok, parsed := input.FetchJSON([]string{"get", "service", svcName, "-n", ns, "-o", "json"})
		if !ok {
			return nil
		}
		svc = asDict(parsed)
	} else {
		ok, parsed := input.FetchJSON([]string{"get", "services", "-n", ns, "-o", "json"})
		if !ok {
			return nil
		}
		for _, item := range itemsOfJSON(parsed) {
			if row := asDict(item); row != nil && len(asDict(asDict(row["spec"])["selector"])) > 0 {
				svc = row
				break
			}
		}
	}
	if svc == nil {
		return nil
	}
	var pods []dict
	if ok, parsed := input.FetchJSON([]string{"get", "pods", "-n", ns, "-o", "json"}); ok {
		for _, item := range itemsOfJSON(parsed) {
			if d := asDict(item); d != nil {
				pods = append(pods, d)
			}
		}
	}
	var ingresses []dict
	if ok, parsed := input.FetchJSON([]string{"get", "ingresses", "-n", ns, "-o", "json"}); ok {
		for _, item := range itemsOfJSON(parsed) {
			if d := asDict(item); d != nil {
				ingresses = append(ingresses, d)
			}
		}
	}
	var httpRoutes []dict
	if ok, parsed := input.FetchJSON([]string{"get", "httproutes", "-n", ns, "-o", "json"}); ok {
		for _, item := range itemsOfJSON(parsed) {
			if d := asDict(item); d != nil {
				httpRoutes = append(httpRoutes, d)
			}
		}
	}
	var netpols []dict
	if ok, parsed := input.FetchJSON([]string{"get", "networkpolicies", "-n", ns, "-o", "json"}); ok {
		for _, item := range itemsOfJSON(parsed) {
			if d := asDict(item); d != nil {
				netpols = append(netpols, d)
			}
		}
	}
	return traceServiceWithRoutes(svc, ns, pods, ingresses, httpRoutes, netpols, input.FetchJSON, input.FetchText)
}

// NamespaceHealth summarizes unhealthy workloads, services, and quotas in a namespace.
type NamespaceHealth struct {
	Namespace        string      `json:"namespace"`
	FailingPods      []Diagnosis `json:"failingPods,omitempty"`
	UnhealthySvcs    []Diagnosis `json:"unhealthyServices,omitempty"`
	StuckDeployments []Diagnosis `json:"stuckDeployments,omitempty"`
	ExhaustedQuotas  []string    `json:"exhaustedQuotas,omitempty"`
}

// DiagnoseNamespace runs a deterministic, zero-LLM health sweep over a namespace.
func DiagnoseNamespace(ns string, fetchJSON FetchJSON, fetchText FetchText) NamespaceHealth {
	if ns == "" {
		ns = "default"
	}
	out := NamespaceHealth{Namespace: ns}

	var pods []dict
	if ok, parsed := fetchJSON([]string{"get", "pods", "-n", ns, "-o", "json"}); ok {
		for _, item := range itemsOfJSON(parsed) {
			if d := asDict(item); d != nil {
				pods = append(pods, d)
			}
		}
	}
	var failing []dict
	for _, pod := range pods {
		if candidatePod(pod) {
			failing = append(failing, pod)
		}
	}
	// Cap follow-up events/logs reads to top 5 failing pods.
	if len(failing) > 5 {
		failing = failing[:5]
	}
	for _, pod := range failing {
		if diag := diagnoseFetched(pod, ns, fetchJSON, fetchText); diag != nil {
			out.FailingPods = append(out.FailingPods, *diag)
		}
	}

	if ok, parsed := fetchJSON([]string{"get", "services", "-n", ns, "-o", "json"}); ok {
		for _, item := range itemsOfJSON(parsed) {
			svc := asDict(item)
			if svc == nil || len(asDict(asDict(svc["spec"])["selector"])) == 0 {
				continue
			}
			if diag := traceServiceWithRoutes(svc, ns, pods, nil, nil, nil, fetchJSON, fetchText); diag != nil {
				out.UnhealthySvcs = append(out.UnhealthySvcs, *diag)
			}
		}
	}

	var quotas []dict
	if ok, parsed := fetchJSON([]string{"get", "resourcequotas", "-n", ns, "-o", "json"}); ok {
		for _, item := range itemsOfJSON(parsed) {
			if q := asDict(item); q != nil {
				quotas = append(quotas, q)
				qName := asStr(asDict(q["metadata"])["name"])
				qStatus := asDict(q["status"])
				hard := asDict(qStatus["hard"])
				used := asDict(qStatus["used"])
				for k, hv := range hard {
					if uv, uok := used[k]; uok && asStr(hv) != "" && asStr(hv) == asStr(uv) {
						out.ExhaustedQuotas = append(out.ExhaustedQuotas, fmt.Sprintf("%s: %s=%s/%s", qName, k, asStr(uv), asStr(hv)))
						break
					}
				}
			}
		}
	}

	if ok, parsed := fetchJSON([]string{"get", "deployments", "-n", ns, "-o", "json"}); ok {
		for _, item := range itemsOfJSON(parsed) {
			dep := asDict(item)
			if dep == nil {
				continue
			}
			desired, ok1 := asNum(asDict(dep["spec"])["replicas"])
			ready, _ := asNum(asDict(dep["status"])["readyReplicas"])
			if ok1 && ready < desired {
				if diag := traceRolloutFromObjects(dep, ns, pods, nil, quotas, nil, fetchJSON, fetchText); diag != nil {
					out.StuckDeployments = append(out.StuckDeployments, *diag)
				}
			}
		}
	}
	return out
}

func traceRolloutFromObjects(dep dict, ns string, pods []dict, events []any, quotas []dict, pdbs []dict, fetchJSON FetchJSON, fetchText FetchText) *Diagnosis {
	name := asStr(asDict(dep["metadata"])["name"])
	if name == "" {
		return nil
	}
	var findings []Finding
	status := asDict(dep["status"])
	for _, cond := range asList(status["conditions"]) {
		if asStr(cond["status"]) == "False" && (asStr(cond["type"]) == "Progressing" || asStr(cond["type"]) == "ReplicaFailure") {
			msg := shortMessage(asStr(cond["message"]))
			if msg == "" {
				msg = asStr(cond["reason"])
			}
			findings = append(findings, Finding{
				Rule:     FindingRollout,
				Cause:    "deployment/" + name + " " + asStr(cond["type"]) + " failed: " + msg,
				Commands: []string{"rollout status deploy/" + name + " -n " + ns, "describe deploy " + name + " -n " + ns},
			})
		}
	}
	// Check ResourceQuotas blocking new pod/RS creation.
	for _, q := range quotas {
		qName := asStr(asDict(q["metadata"])["name"])
		qStatus := asDict(q["status"])
		hard := asDict(qStatus["hard"])
		used := asDict(qStatus["used"])
		for k, hv := range hard {
			if uv, ok := used[k]; ok && asStr(hv) != "" && asStr(hv) == asStr(uv) {
				findings = append(findings, Finding{
					Rule:     FindingQuota,
					Cause:    fmt.Sprintf("resourcequota/%s exhausted for %s (%s/%s)", qName, k, asStr(uv), asStr(hv)),
					Commands: []string{"describe resourcequota " + qName + " -n " + ns},
				})
				break
			}
		}
	}
	// Inspect failing pods matching deployment selector.
	selector := asDict(asDict(asDict(dep["spec"])["selector"])["matchLabels"])
	if len(selector) > 0 {
		for _, pod := range pods {
			if podMatchesSelector(pod, selector) && candidatePod(pod) {
				if diag := diagnoseFetched(pod, ns, fetchJSON, fetchText); diag != nil && len(diag.Findings) > 0 {
					findings = append(findings, diag.Findings[0])
					break
				}
			}
		}
		for _, pdb := range pdbs {
			pdbName := asStr(asDict(pdb["metadata"])["name"])
			pdbSel := asDict(asDict(asDict(pdb["spec"])["selector"])["matchLabels"])
			allowed, hasAllowed := asNum(asDict(pdb["status"])["disruptionsAllowed"])
			if len(pdbSel) > 0 && hasAllowed && allowed == 0 && dictLabelsMatch(pdbSel, selector) {
				findings = append(findings, Finding{
					Rule:     FindingRollout,
					Cause:    "pdb/" + pdbName + " allows 0 disruptions on deployment/" + name + " pods",
					Commands: []string{"describe pdb " + pdbName + " -n " + ns},
				})
			}
		}
	}
	for _, ev := range eventRows(events, name) {
		if asStr(ev["type"]) == "Warning" && strings.Contains(strings.ToLower(asStr(ev["reason"])), "fail") {
			findings = append(findings, Finding{
				Rule:     FindingRollout,
				Cause:    "deployment/" + name + " event: " + shortMessage(asStr(ev["message"])),
				Commands: []string{"get events -n " + ns},
			})
			break
		}
	}
	if len(findings) == 0 {
		return nil
	}
	if len(findings) > 3 {
		findings = findings[:3]
	}
	return &Diagnosis{Pod: "deployment/" + name, Namespace: ns, Findings: findings}
}

func traceRollout(input InspectPodsInput, deployName string) *Diagnosis {
	ns := input.Namespace
	if ns == "" {
		ns = "default"
	}
	var dep dict
	if deployName != "" {
		ok, parsed := input.FetchJSON([]string{"get", "deployment", deployName, "-n", ns, "-o", "json"})
		if !ok {
			return nil
		}
		dep = asDict(parsed)
	} else {
		ok, parsed := input.FetchJSON([]string{"get", "deployments", "-n", ns, "-o", "json"})
		if !ok {
			return nil
		}
		for _, item := range itemsOfJSON(parsed) {
			d := asDict(item)
			if d == nil {
				continue
			}
			desired, ok1 := asNum(asDict(d["spec"])["replicas"])
			ready, _ := asNum(asDict(d["status"])["readyReplicas"])
			if ok1 && ready < desired {
				dep = d
				break
			}
		}
	}
	if dep == nil {
		return nil
	}
	var pods []dict
	if ok, parsed := input.FetchJSON([]string{"get", "pods", "-n", ns, "-o", "json"}); ok {
		for _, item := range itemsOfJSON(parsed) {
			if d := asDict(item); d != nil {
				pods = append(pods, d)
			}
		}
	}
	var quotas []dict
	if ok, parsed := input.FetchJSON([]string{"get", "resourcequotas", "-n", ns, "-o", "json"}); ok {
		for _, item := range itemsOfJSON(parsed) {
			if d := asDict(item); d != nil {
				quotas = append(quotas, d)
			}
		}
	}
	var pdbs []dict
	if ok, parsed := input.FetchJSON([]string{"get", "pdb", "-n", ns, "-o", "json"}); ok {
		for _, item := range itemsOfJSON(parsed) {
			if d := asDict(item); d != nil {
				pdbs = append(pdbs, d)
			}
		}
	}
	events := eventsFor(asStr(asDict(dep["metadata"])["name"]), ns, input.FetchJSON)
	return traceRolloutFromObjects(dep, ns, pods, events, quotas, pdbs, input.FetchJSON, input.FetchText)
}
