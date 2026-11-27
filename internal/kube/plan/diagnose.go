package plan

import (
	"encoding/json"
	"github.com/NadafMM/kubectl-shell/internal/kube"
	"regexp"
	"sort"
	"strconv"
	"strings"

	// FindingRule names one diagnosis cause.
	"github.com/NadafMM/kubectl-shell/internal/kube/live"
)

type FindingRule string

const (
	FindingOOM     FindingRule = "oom"
	FindingCrash   FindingRule = "crash"
	FindingImage   FindingRule = "image"
	FindingConfig  FindingRule = "config"
	FindingPending FindingRule = "pending"
	FindingProbe   FindingRule = "probe"
)

// Finding is one ranked cause for a pod.
type Finding struct {
	Rule     FindingRule `json:"rule"`
	Cause    string      `json:"cause"`
	Commands []string    `json:"commands"`
}

// Diagnosis ranks up to three causes for one pod.
type Diagnosis struct {
	Pod       string    `json:"pod"`
	Namespace string    `json:"namespace"`
	Findings  []Finding `json:"findings"`
}

type dict = map[string]any

var sensitiveLog = regexp.MustCompile(`(?i)(password|token|secret|apikey|api-key|client-secret|client_secret)=\S+`)

func asDict(value any) dict {
	if m, ok := value.(dict); ok {
		return m
	}
	return nil
}

func asStr(value any) string {
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}

func asNum(value any) (float64, bool) {
	switch n := value.(type) {
	case float64:
		return n, true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

func asList(value any) []dict {
	arr, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]dict, 0, len(arr))
	for _, item := range arr {
		if row := asDict(item); row != nil {
			out = append(out, row)
		}
	}
	return out
}

func logTail(text string) string {
	raw := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	var lines []string
	for _, line := range raw {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) > 30 {
		lines = lines[len(lines)-30:]
	}
	for i, line := range lines {
		lines[i] = sensitiveLog.ReplaceAllStringFunc(line, func(match string) string {
			eq := strings.Index(match, "=")
			if eq < 0 {
				return "***"
			}
			return match[:eq+1] + "***"
		})
	}
	return strings.Join(lines, " | ")
}

func containersOf(pod dict) []dict {
	spec := asDict(pod["spec"])
	if spec == nil {
		return nil
	}
	return append(asList(spec["containers"]), asList(spec["initContainers"])...)
}

func statusesOf(pod dict) []dict {
	status := asDict(pod["status"])
	if status == nil {
		return nil
	}
	return append(asList(status["containerStatuses"]), asList(status["initContainerStatuses"])...)
}

func containerNamed(pod dict, name string) dict {
	for _, item := range containersOf(pod) {
		if asStr(item["name"]) == name {
			return item
		}
	}
	return nil
}

func terminated(status dict) dict {
	if last := asDict(status["lastState"]); last != nil {
		if term := asDict(last["terminated"]); term != nil {
			return term
		}
	}
	if state := asDict(status["state"]); state != nil {
		if term := asDict(state["terminated"]); term != nil {
			return term
		}
	}
	return nil
}

func waiting(status dict) dict {
	state := asDict(status["state"])
	if state == nil {
		return nil
	}
	return asDict(state["waiting"])
}

func isOOM(term dict) bool {
	if term == nil {
		return false
	}
	if asStr(term["reason"]) == "OOMKilled" {
		return true
	}
	exit, ok := asNum(term["exitCode"])
	return ok && exit == 137
}

func memoryLimit(pod dict, name string) string {
	container := containerNamed(pod, name)
	resources := asDict(container["resources"])
	limits := asDict(resources["limits"])
	if mem := asStr(limits["memory"]); mem != "" {
		return mem
	}
	return "none"
}

func imageOf(pod dict, name string) string {
	if img := asStr(containerNamed(pod, name)["image"]); img != "" {
		return img
	}
	return "unknown"
}

func describeCmd(pod, ns string) string {
	return "describe pod " + pod + " -n " + ns
}

func eventRows(events []any, podName string) []dict {
	var rows []dict
	for _, event := range events {
		item := asDict(event)
		if item == nil {
			continue
		}
		involved := asStr(asDict(item["involvedObject"])["name"])
		if involved != "" && involved != podName {
			continue
		}
		rows = append(rows, item)
	}
	return rows
}

func messagesOf(rows []dict) []string {
	var out []string
	for _, row := range rows {
		if msg := asStr(row["message"]); msg != "" {
			out = append(out, msg)
		}
	}
	return out
}

func pullKind(message string) string {
	if regexp.MustCompile(`(?i)unauthorized|authentication|forbidden|\b401\b|\b403\b`).MatchString(message) {
		return "auth"
	}
	if regexp.MustCompile(`(?i)not found|manifest unknown|\b404\b`).MatchString(message) {
		return "tag not found"
	}
	if regexp.MustCompile(`(?i)timed out|timeout|i/o timeout`).MatchString(message) {
		return "registry timeout"
	}
	return "pull failed"
}

func shortMessage(message string) string {
	for _, line := range strings.Split(strings.ReplaceAll(message, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if len(line) > 160 {
			return line[:160]
		}
		return line
	}
	return ""
}

type configRefKind string

const (
	refSecret    configRefKind = "Secret"
	refConfigMap configRefKind = "ConfigMap"
)

type configRef struct {
	Kind configRefKind
	Name string
	Key  string
}

func specRef(pod dict, containerName string) *configRef {
	container := containerNamed(pod, containerName)
	for _, env := range asList(container["env"]) {
		from := asDict(env["valueFrom"])
		if secret := asDict(from["secretKeyRef"]); asStr(secret["name"]) != "" {
			return &configRef{Kind: refSecret, Name: asStr(secret["name"]), Key: asStr(secret["key"])}
		}
		if config := asDict(from["configMapKeyRef"]); asStr(config["name"]) != "" {
			return &configRef{Kind: refConfigMap, Name: asStr(config["name"]), Key: asStr(config["key"])}
		}
	}
	for _, from := range asList(container["envFrom"]) {
		if name := asStr(asDict(from["secretRef"])["name"]); name != "" {
			return &configRef{Kind: refSecret, Name: name}
		}
		if name := asStr(asDict(from["configMapRef"])["name"]); name != "" {
			return &configRef{Kind: refConfigMap, Name: name}
		}
	}
	for _, volume := range asList(asDict(pod["spec"])["volumes"]) {
		if name := asStr(asDict(volume["secret"])["secretName"]); name != "" {
			return &configRef{Kind: refSecret, Name: name}
		}
		if name := asStr(asDict(volume["configMap"])["name"]); name != "" {
			return &configRef{Kind: refConfigMap, Name: name}
		}
	}
	return nil
}

func configRefFrom(pod dict, containerName, message string) *configRef {
	if m := regexp.MustCompile(`(?i)key\s+(\S+)\s+in\s+Secret\s+(?:\S+/)?([A-Za-z0-9.-]+)`).FindStringSubmatch(message); len(m) == 3 {
		return &configRef{Kind: refSecret, Name: m[2], Key: strings.TrimRight(m[1], ".,")}
	}
	if m := regexp.MustCompile(`(?i)key\s+(\S+)\s+in\s+ConfigMap\s+(?:\S+/)?([A-Za-z0-9.-]+)`).FindStringSubmatch(message); len(m) == 3 {
		return &configRef{Kind: refConfigMap, Name: m[2], Key: strings.TrimRight(m[1], ".,")}
	}
	if m := regexp.MustCompile(`(?i)secret\s+"([^"]+)"\s+not found`).FindStringSubmatch(message); len(m) == 2 {
		return &configRef{Kind: refSecret, Name: m[1]}
	}
	if m := regexp.MustCompile(`(?i)configmap\s+"([^"]+)"\s+not found`).FindStringSubmatch(message); len(m) == 2 {
		return &configRef{Kind: refConfigMap, Name: m[1]}
	}
	return specRef(pod, containerName)
}

func isPending(pod dict) bool {
	status := asDict(pod["status"])
	if asStr(status["phase"]) == "Pending" {
		return true
	}
	for _, condition := range asList(status["conditions"]) {
		if asStr(condition["type"]) == "PodScheduled" && asStr(condition["status"]) == "False" {
			return true
		}
	}
	return false
}

func claimName(pod dict, messages []string) string {
	for _, volume := range asList(asDict(pod["spec"])["volumes"]) {
		if name := asStr(asDict(volume["persistentVolumeClaim"])["claimName"]); name != "" {
			return name
		}
	}
	blob := strings.Join(messages, "\n")
	if m := regexp.MustCompile(`(?i)persistentvolumeclaim\s+"([^"]+)"`).FindStringSubmatch(blob); len(m) == 2 {
		return m[1]
	}
	return "unknown"
}

func pendingCause(pod dict, messages []string) string {
	blob := strings.Join(messages, "\n")
	switch {
	case regexp.MustCompile(`(?i)insufficient cpu`).MatchString(blob):
		return "Pending: insufficient cpu"
	case regexp.MustCompile(`(?i)insufficient memory`).MatchString(blob):
		return "Pending: insufficient memory"
	case regexp.MustCompile(`(?i)persistentvolumeclaim|unbound immediate`).MatchString(blob):
		return "Pending: unbound PersistentVolumeClaim " + claimName(pod, messages)
	case regexp.MustCompile(`(?i)taint`).MatchString(blob):
		return "Pending: node taint"
	case regexp.MustCompile(`(?i)affinity`).MatchString(blob):
		return "Pending: node affinity mismatch"
	default:
		return "Pending: unschedulable"
	}
}

func portOf(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case float64:
		return trimFloat(v)
	default:
		if n, ok := asNum(value); ok {
			return trimFloat(n)
		}
		return ""
	}
}

func probeTarget(pod dict, kind string) string {
	field := "readinessProbe"
	if kind == "liveness" {
		field = "livenessProbe"
	}
	containers := containersOf(pod)
	var container dict
	for _, item := range containers {
		if asDict(item[field]) != nil {
			container = item
			break
		}
	}
	if container == nil && len(containers) > 0 {
		container = containers[0]
	}
	probe := asDict(container[field])
	if probe == nil {
		return "probe"
	}
	if http := asDict(probe["httpGet"]); http != nil {
		path := asStr(http["path"])
		if path == "" {
			path = "/"
		}
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		return "http://:" + portOf(http["port"]) + path
	}
	if tcp := asDict(probe["tcpSocket"]); tcp != nil {
		return "tcp:" + portOf(tcp["port"])
	}
	if execProbe := asDict(probe["exec"]); execProbe != nil {
		var parts []string
		if arr, ok := execProbe["command"].([]any); ok {
			for _, item := range arr {
				if s := asStr(item); s != "" {
					parts = append(parts, s)
				}
			}
		}
		if len(parts) == 0 {
			return "exec"
		}
		return "exec " + strings.Join(parts, " ")
	}
	if grpc := asDict(probe["grpc"]); grpc != nil {
		return "grpc:" + portOf(grpc["port"])
	}
	return "probe"
}

// DiagnosePod ranks up to three causes for one pod. No kubectl and no network.
func DiagnosePod(pod any, events []any, previousLogs map[string]string) *Diagnosis {
	body := asDict(pod)
	name := asStr(asDict(body["metadata"])["name"])
	if body == nil || name == "" {
		return nil
	}
	ns := asStr(asDict(body["metadata"])["namespace"])
	if ns == "" {
		ns = "default"
	}
	rows := eventRows(events, name)
	var findings []Finding

	for _, status := range statusesOf(body) {
		container := asStr(status["name"])
		if container == "" {
			container = "container"
		}
		term := terminated(status)
		if !isOOM(term) {
			continue
		}
		exit := 137.0
		if n, ok := asNum(term["exitCode"]); ok {
			exit = n
		}
		findings = append(findings, Finding{
			Rule:  FindingOOM,
			Cause: "container " + container + " was OOMKilled (exit " + trimFloat(exit) + "); memory limit " + memoryLimit(body, container),
			Commands: []string{
				describeCmd(name, ns),
				"logs " + name + " -c " + container + " --previous -n " + ns,
			},
		})
	}

	for _, status := range statusesOf(body) {
		container := asStr(status["name"])
		if container == "" {
			container = "container"
		}
		term := terminated(status)
		if isOOM(term) {
			continue
		}
		reason := asStr(waiting(status)["reason"])
		exit, hasExit := asNum(term["exitCode"])
		crashed := reason == "CrashLoopBackOff" || (hasExit && exit != 0)
		if !crashed {
			continue
		}
		tail := logTail(previousLogs[container])
		code := "unknown"
		if hasExit {
			code = trimFloat(exit)
		}
		cause := "container " + container + " exited " + code
		if tail != "" {
			cause += "; " + tail
		}
		findings = append(findings, Finding{
			Rule:     FindingCrash,
			Cause:    cause,
			Commands: []string{"logs " + name + " -c " + container + " --previous -n " + ns},
		})
	}

	for _, status := range statusesOf(body) {
		container := asStr(status["name"])
		if container == "" {
			container = "container"
		}
		wait := waiting(status)
		reason := asStr(wait["reason"])
		if reason != "ImagePullBackOff" && reason != "ErrImagePull" {
			continue
		}
		eventMessage := ""
		for _, message := range messagesOf(rows) {
			if regexp.MustCompile(`(?i)pull|manifest|unauthorized|timeout|not found`).MatchString(message) {
				eventMessage = message
				break
			}
		}
		message := eventMessage
		if message == "" {
			message = asStr(wait["message"])
		}
		shown := shortMessage(message)
		cause := "image " + imageOf(body, container) + " failed to pull (" + pullKind(message) + ")"
		if shown != "" {
			cause += "; " + shown
		}
		findings = append(findings, Finding{
			Rule:     FindingImage,
			Cause:    cause,
			Commands: []string{describeCmd(name, ns)},
		})
	}

	for _, status := range statusesOf(body) {
		container := asStr(status["name"])
		if container == "" {
			container = "container"
		}
		wait := waiting(status)
		if asStr(wait["reason"]) != "CreateContainerConfigError" {
			continue
		}
		ref := configRefFrom(body, container, asStr(wait["message"])+"\n"+strings.Join(messagesOf(rows), "\n"))
		what := "missing Secret or ConfigMap"
		var extra []string
		if ref != nil {
			kind := string(ref.Kind)
			resource := "secret"
			if ref.Kind == refConfigMap {
				resource = "configmap"
			}
			what = "missing " + kind + " " + ref.Name
			if ref.Key != "" {
				what += " key " + ref.Key
			}
			extra = []string{"get " + resource + " " + ref.Name + " -n " + ns}
		}
		findings = append(findings, Finding{
			Rule:     FindingConfig,
			Cause:    what,
			Commands: append([]string{describeCmd(name, ns)}, extra...),
		})
	}

	unscheduled := false
	for _, condition := range asList(asDict(body["status"])["conditions"]) {
		if asStr(condition["type"]) == "PodScheduled" && asStr(condition["status"]) == "False" {
			unscheduled = true
			break
		}
	}
	schedulingEvent := false
	for _, row := range rows {
		if asStr(row["reason"]) == "FailedScheduling" {
			schedulingEvent = true
			break
		}
	}
	if unscheduled || schedulingEvent {
		var conditionMessages []string
		for _, condition := range asList(asDict(body["status"])["conditions"]) {
			if asStr(condition["type"]) == "PodScheduled" {
				if msg := asStr(condition["message"]); msg != "" {
					conditionMessages = append(conditionMessages, msg)
				}
			}
		}
		findings = append(findings, Finding{
			Rule:     FindingPending,
			Cause:    pendingCause(body, append(messagesOf(rows), conditionMessages...)),
			Commands: []string{describeCmd(name, ns), "get events -n " + ns},
		})
	}

	var unhealthy []dict
	for _, row := range rows {
		if asStr(row["reason"]) == "Unhealthy" {
			unhealthy = append(unhealthy, row)
		}
	}
	if len(unhealthy) > 0 {
		messages := messagesOf(unhealthy)
		live, ready := 0, 0
		for _, message := range messages {
			if regexp.MustCompile(`(?i)liveness`).MatchString(message) {
				live++
			}
			if regexp.MustCompile(`(?i)readiness`).MatchString(message) {
				ready++
			}
		}
		kind := "readiness"
		if live > ready {
			kind = "liveness"
		}
		label := "1 Unhealthy warning"
		if len(unhealthy) != 1 {
			label = trimFloat(float64(len(unhealthy))) + " Unhealthy warnings"
		}
		findings = append(findings, Finding{
			Rule:     FindingProbe,
			Cause:    kind + " probe " + probeTarget(body, kind) + " failed (" + label + ")",
			Commands: []string{describeCmd(name, ns)},
		})
	}

	if len(findings) == 0 {
		return nil
	}
	if len(findings) > 3 {
		findings = findings[:3]
	}
	return &Diagnosis{Pod: name, Namespace: ns, Findings: findings}
}

func trimFloat(n float64) string {
	if n == float64(int64(n)) {
		return strconv.FormatInt(int64(n), 10)
	}
	return strconv.FormatFloat(n, 'f', -1, 64)
}

var healthQuestion = regexp.MustCompile(`(?i)crash|backoff|restart|oom|not starting|failing|pending|unschedul|cannot schedule|can't schedule|insufficient|imagepull|errimage|pull back|registry|not ready|unhealthy|readiness|liveness|probe|createcontainer|configerror`)

var askStop = map[string]bool{
	"why": true, "is": true, "the": true, "a": true, "an": true, "pod": true, "pods": true,
	"how": true, "do": true, "i": true, "what": true, "my": true, "this": true, "that": true,
	"in": true, "on": true, "for": true, "to": true, "and": true, "or": true, "it": true,
	"its": true, "of": true, "with": true, "was": true, "are": true, "not": true, "from": true,
	"crashing": true, "crash": true, "pending": true, "failing": true, "failed": true,
	"unhealthy": true, "restarting": true, "down": true, "stuck": true, "oom": true,
	"backoff": true, "imagepull": true, "probe": true,
}

// PodNameFrom picks a pod name from the question or known cluster names.
func PodNameFrom(question string, knownNames []string) string {
	if m := regexp.MustCompile(`(?i)\bpod/([a-z0-9][-a-z0-9.]*)`).FindStringSubmatch(question); len(m) == 2 {
		return m[1]
	}
	ranked := append([]string{}, knownNames...)
	sort.SliceStable(ranked, func(i, j int) bool { return len(ranked[i]) > len(ranked[j]) })
	q := strings.ToLower(question)
	for _, known := range ranked {
		if len(known) >= 3 && strings.Contains(q, strings.ToLower(known)) {
			return known
		}
	}
	if m := regexp.MustCompile(`(?i)\bpod\s+([a-z0-9][-a-z0-9.]*)`).FindStringSubmatch(question); len(m) == 2 {
		if !askStop[strings.ToLower(m[1])] {
			return m[1]
		}
	}
	return ""
}

// AdviceFrom turns a diagnosis into Advice, including findings for --json.
func AdviceFrom(diagnosis Diagnosis) Advice {
	var commands []string
	for _, finding := range diagnosis.Findings {
		for _, command := range finding.Commands {
			found := false
			for _, existing := range commands {
				if existing == command {
					found = true
					break
				}
			}
			if !found {
				commands = append(commands, command)
			}
		}
	}
	var whyParts []string
	for _, finding := range diagnosis.Findings {
		whyParts = append(whyParts, finding.Cause)
	}
	return Advice{
		Why:      strings.Join(whyParts, "\n"),
		Commands: commands,
		Findings: diagnosis.Findings,
	}
}

// FetchJSON returns parsed kubectl JSON.
type FetchJSON func(args []string) (ok bool, json any)

// FetchText returns kubectl stdout text.
type FetchText func(args []string) string

func itemsOfJSON(value any) []any {
	if arr, ok := value.([]any); ok {
		return arr
	}
	body := asDict(value)
	if body == nil {
		return nil
	}
	if arr, ok := body["items"].([]any); ok {
		return arr
	}
	return nil
}

func needsPreviousLog(status dict) bool {
	term := terminated(status)
	if isOOM(term) {
		return false
	}
	exit, hasExit := asNum(term["exitCode"])
	return asStr(waiting(status)["reason"]) == "CrashLoopBackOff" || (hasExit && exit != 0)
}

func candidatePod(pod any) bool {
	body := asDict(pod)
	if body == nil {
		return false
	}
	if isPending(body) {
		return true
	}
	for _, status := range statusesOf(body) {
		if ready, ok := status["ready"].(bool); ok && !ready {
			return true
		}
		if asStr(waiting(status)["reason"]) != "" {
			return true
		}
		if exit, ok := asNum(terminated(status)["exitCode"]); ok && exit != 0 {
			return true
		}
	}
	return false
}

func eventsFor(name, ns string, fetchJSON FetchJSON) []any {
	ok, parsed := fetchJSON([]string{
		"get", "events", "-n", ns,
		"--field-selector", "involvedObject.name=" + name,
		"-o", "json",
	})
	if !ok {
		return nil
	}
	return itemsOfJSON(parsed)
}

func previousLogsFor(pod dict, ns string, fetchText FetchText) map[string]string {
	logs := map[string]string{}
	podName := asStr(asDict(pod["metadata"])["name"])
	for _, status := range statusesOf(pod) {
		container := asStr(status["name"])
		if podName == "" || container == "" || !needsPreviousLog(status) {
			continue
		}
		logs[container] = fetchText([]string{"logs", podName, "-n", ns, "-c", container, "--previous"})
	}
	return logs
}

func diagnoseFetched(pod any, ns string, fetchJSON FetchJSON, fetchText FetchText) *Diagnosis {
	body := asDict(pod)
	name := asStr(asDict(body["metadata"])["name"])
	if body == nil || name == "" || !candidatePod(body) {
		return nil
	}
	return DiagnosePod(body, eventsFor(name, ns, fetchJSON), previousLogsFor(body, ns, fetchText))
}

// InspectPodsInput drives live diagnosis for ask.
type InspectPodsInput struct {
	Question   string
	Namespace  string
	KnownNames []string
	FetchJSON  FetchJSON
	FetchText  FetchText
}

// InspectPods fetches at most one named pod, or ranks all failing pods (top 5)
// in the namespace and returns the highest-priority diagnosis.
func InspectPods(input InspectPodsInput) *Diagnosis {
	if !healthQuestion.MatchString(input.Question) {
		return nil
	}
	ns := input.Namespace
	if ns == "" {
		ns = "default"
	}
	named := PodNameFrom(input.Question, input.KnownNames)
	if named != "" {
		ok, parsed := input.FetchJSON([]string{"get", "pod", named, "-n", ns, "-o", "json"})
		if !ok || parsed == nil {
			return nil
		}
		return diagnoseFetched(parsed, ns, input.FetchJSON, input.FetchText)
	}
	ok, parsed := input.FetchJSON([]string{"get", "pods", "-n", ns, "-o", "json"})
	if !ok {
		return nil
	}
	items := itemsOfJSON(parsed)
	var failing []any
	for _, pod := range items {
		if candidatePod(pod) {
			failing = append(failing, pod)
		}
	}
	sort.SliceStable(failing, func(i, j int) bool {
		return podSeverity(asDict(failing[i])) > podSeverity(asDict(failing[j]))
	})
	if len(failing) > 5 {
		failing = failing[:5]
	}
	for _, pod := range failing {
		if found := diagnoseFetched(pod, ns, input.FetchJSON, input.FetchText); found != nil {
			return found
		}
	}
	return nil
}

func podSeverity(pod dict) int {
	if pod == nil {
		return 0
	}
	score := 0
	if isPending(pod) {
		score += 10
	}
	for _, status := range statusesOf(pod) {
		if isOOM(terminated(status)) {
			score += 100
		}
		if reason := asStr(waiting(status)["reason"]); reason != "" {
			switch reason {
			case "CrashLoopBackOff", "ImagePullBackOff", "ErrImagePull", "CreateContainerConfigError":
				score += 80
			default:
				score += 30
			}
		}
		if restarts, ok := asNum(status["restartCount"]); ok {
			score += int(restarts)
		}
	}
	return score
}

// AskAdvice prefers live multi-hop or pod diagnosis, then falls back to static Advise.
func AskAdvice(question string, cluster kube.Cluster, ctx []string) Advice {
	var known []string
	for _, item := range cluster.Objects {
		if item.Kind == "pod" {
			known = append(known, item.Name)
		}
	}
	named := PodNameFrom(question, known)
	ns := kube.NamespaceOf(cluster)
	for _, item := range cluster.Objects {
		if item.Kind == "pod" && item.Name == named && item.NS != "" {
			ns = item.NS
			break
		}
	}
	withCtx := func(args []string) []string {
		if len(ctx) == 0 {
			return args
		}
		out := append([]string{}, args...)
		return append(out, ctx...)
	}
	input := InspectPodsInput{
		Question:   question,
		Namespace:  ns,
		KnownNames: known,
		FetchJSON: func(args []string) (bool, any) {
			result := live.RunJson(withCtx(args), 3000)
			if !result.OK {
				return false, nil
			}
			var parsed any
			if err := json.Unmarshal(result.JSON, &parsed); err != nil {
				return false, nil
			}
			return true, parsed
		},
		FetchText: func(args []string) string {
			_, stdout, _ := live.CaptureLive(withCtx(args))
			return stdout
		},
	}
	explicitPod := regexp.MustCompile(`(?i)\bpod[/\s]+[a-z0-9]`).MatchString(question)
	if !explicitPod {
		svcName := serviceNameFrom(question, cluster)
		if svcName != "" || serviceQuestionRe.MatchString(question) {
			traceInput := input
			if svcName != "" {
				for _, item := range cluster.Objects {
					if item.Kind == "service" && item.Name == svcName && item.NS != "" {
						traceInput.Namespace = item.NS
						break
					}
				}
			}
			if diagnosis := traceService(traceInput, svcName); diagnosis != nil {
				return AdviceFrom(*diagnosis)
			}
		}
		depName := deploymentNameFrom(question, cluster)
		if depName != "" || rolloutQuestionRe.MatchString(question) {
			traceInput := input
			if depName != "" {
				for _, item := range cluster.Objects {
					if item.Kind == "deployment" && item.Name == depName && item.NS != "" {
						traceInput.Namespace = item.NS
						break
					}
				}
			}
			if diagnosis := traceRollout(traceInput, depName); diagnosis != nil {
				return AdviceFrom(*diagnosis)
			}
		}
	}
	if diagnosis := InspectPods(input); diagnosis != nil {
		return AdviceFrom(*diagnosis)
	}
	return Advise(question, cluster)
}
