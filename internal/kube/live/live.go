package live

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/NadafMM/kubectl-shell/internal/kube"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const liveTTL = 12 * time.Second

const ProjectedJSONPath = `{range .items[*]}{.kind}{"\t"}{.metadata.name}{"\t"}{.metadata.namespace}{"\t"}{.metadata.creationTimestamp}{"\t"}{.type}{"\t"}{.status.phase}{"\t"}{.status.readyReplicas}{"\t"}{.status.replicas}{"\t"}{.spec.type}{"\t"}{.spec.schedule}{"\t"}{.spec.capacity.storage}{"\t"}{.status.nodeInfo.kubeletVersion}{"\t"}{.reason}{"\t"}{.message}{"\t"}{.involvedObject.kind}{"\t"}{.involvedObject.name}{"\t"}{.involvedObject.namespace}{"\t"}{range .status.containerStatuses[*]}{.state.waiting.reason}{","}{end}{"\t"}{range .spec.containers[*]}{.name}{","}{end}{range .spec.template.spec.containers[*]}{.name}{","}{end}{"\t"}{range $k,$v := .metadata.labels}{$k}={$v}{","}{end}{"\n"}{end}`

var objectGroups = [][]string{
	{"pods", "services", "configmaps", "serviceaccounts", "persistentvolumeclaims"},
	{"secrets"},
	{"deployments.apps", "replicasets.apps", "statefulsets.apps", "daemonsets.apps"},
	{"jobs.batch", "cronjobs.batch"},
	{"ingresses.networking.k8s.io", "networkpolicies.networking.k8s.io"},
	{"horizontalpodautoscalers.autoscaling"},
	{"endpoints", "poddisruptionbudgets.policy", "resourcequotas", "limitranges"},
}

var kindMap = map[string]string{
	"Pod": "pod", "Deployment": "deployment", "Service": "service", "ConfigMap": "configmap",
	"Secret": "secret", "Ingress": "ingress", "PersistentVolumeClaim": "pvc",
	"ServiceAccount": "serviceaccount", "ReplicaSet": "replicaset", "StatefulSet": "statefulset",
	"DaemonSet": "daemonset", "Job": "job", "CronJob": "cronjob",
	"HorizontalPodAutoscaler": "hpa", "Endpoints": "endpoints",
	"PodDisruptionBudget": "poddisruptionbudget", "NetworkPolicy": "networkpolicy",
	"ResourceQuota": "resourcequota", "LimitRange": "limitrange", "StorageClass": "storageclass",
	"PriorityClass": "priorityclass", "CustomResourceDefinition": "customresourcedefinition",
}

type kItem struct {
	Kind     string `json:"kind"`
	Metadata *struct {
		Name              string            `json:"name"`
		Namespace         string            `json:"namespace"`
		CreationTimestamp string            `json:"creationTimestamp"`
		Labels            map[string]string `json:"labels"`
	} `json:"metadata"`
	Type           string `json:"type"`
	Reason         string `json:"reason"`
	Message        string `json:"message"`
	InvolvedObject *struct {
		Kind      string `json:"kind"`
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"involvedObject"`
	Spec *struct {
		Type          string `json:"type"`
		Schedule      string `json:"schedule"`
		Unschedulable bool   `json:"unschedulable"`
		Capacity      *struct {
			Storage string `json:"storage"`
		} `json:"capacity"`
		Containers []struct {
			Name string `json:"name"`
		} `json:"containers"`
	} `json:"spec"`
	Status *struct {
		Phase         string `json:"phase"`
		ReadyReplicas int    `json:"readyReplicas"`
		Replicas      int    `json:"replicas"`
		NodeInfo      *struct {
			KubeletVersion string `json:"kubeletVersion"`
		} `json:"nodeInfo"`
		Conditions []struct {
			Type   string `json:"type"`
			Status string `json:"status"`
		} `json:"conditions"`
		ContainerStatuses []struct {
			State *struct {
				Waiting *struct {
					Reason string `json:"reason"`
				} `json:"waiting"`
			} `json:"state"`
		} `json:"containerStatuses"`
	} `json:"status"`
}

type cacheRow struct {
	At      int64        `json:"at"`
	Key     string       `json:"key"`
	Cluster kube.Cluster `json:"cluster"`
}

var (
	memory        *cacheRow
	contextMemory *struct {
		at      time.Time
		cluster kube.Cluster
	}
)

func cachePath() string {
	return filepath.Join(kube.StateDir(), "live.json")
}

func kubectl(args []string, timeout time.Duration) (int, string) {
	name := KubectlBin()
	argv := append([]string{}, args...)
	if script := os.Getenv("KUBE_SHELL_NODE_SCRIPT"); script != "" {
		argv = append([]string{script}, args...)
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, argv...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &bytes.Buffer{}
	err := cmd.Run()
	if err == nil {
		return 0, stdout.String()
	}
	if exit, ok := err.(*exec.ExitError); ok {
		return exit.ExitCode(), stdout.String()
	}
	return 1, stdout.String()
}

// KubectlJSONResult is one kubectl -o json invocation.
type KubectlJSONResult struct {
	OK    bool
	JSON  json.RawMessage
	Error string
}

func stderrFirstLine(stderr string) string {
	stderr = strings.ReplaceAll(stderr, "\r\n", "\n")
	stderr = strings.ReplaceAll(stderr, "\r", "\n")
	for _, line := range strings.Split(stderr, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if len(line) > 120 {
			return line[:120]
		}
		return line
	}
	return ""
}

// RunJson shells out to kubectl, parses stdout as JSON, and surfaces the first
// stderr line (trimmed to 120 chars) on failure. Does not touch the live cache.
func RunJson(args []string, timeoutMs int) KubectlJSONResult {
	if timeoutMs <= 0 {
		timeoutMs = 3000
	}
	name := KubectlBin()
	argv := append([]string{}, args...)
	if script := os.Getenv("KUBE_SHELL_NODE_SCRIPT"); script != "" {
		argv = append([]string{script}, args...)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, argv...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	var err error
	withChildInterrupt(func() { err = cmd.Run() })
	if err != nil {
		line := stderrFirstLine(stderr.String())
		if line != "" {
			return KubectlJSONResult{OK: false, Error: line}
		}
		if ctx.Err() == context.DeadlineExceeded {
			return KubectlJSONResult{OK: false, Error: "timed out"}
		}
		return KubectlJSONResult{OK: false, Error: "failed"}
	}
	raw := stdout.Bytes()
	var parsed any
	if uerr := json.Unmarshal(raw, &parsed); uerr != nil {
		return KubectlJSONResult{OK: false, Error: "invalid JSON"}
	}
	out := make(json.RawMessage, len(raw))
	copy(out, raw)
	return KubectlJSONResult{OK: true, JSON: out}
}

// CaptureLive runs kubectl with stdout and stderr collected.
func CaptureLive(args []string) (code int, stdout, stderr string) {
	name := KubectlBin()
	argv := append([]string{}, args...)
	if script := os.Getenv("KUBE_SHELL_NODE_SCRIPT"); script != "" {
		argv = append([]string{script}, args...)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, argv...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	var err error
	withChildInterrupt(func() { err = cmd.Run() })
	if err == nil {
		return 0, outBuf.String(), errBuf.String()
	}
	if exit, ok := err.(*exec.ExitError); ok {
		return exit.ExitCode(), outBuf.String(), errBuf.String()
	}
	return 1, outBuf.String(), errBuf.String()
}

// RunLiveOptions controls how RunLive attaches stdio.
type RunLiveOptions struct {
	InheritStdin bool
	Stdin        *os.File
	StdinReader  io.Reader
	Stdout       *os.File
	Stderr       *os.File
}

// RunLive runs kubectl with inherited stdio. The completion cache is cleared
// only when the argv can change names the snapshot holds.
// When KUBE_SHELL_REDACT_STDOUT=1 and stdout is not a TTY, sensitive Secret,
// ConfigMap, and kubeconfig payloads are captured and redacted before writing.
func RunLive(args []string, options RunLiveOptions) int {
	stdin := options.Stdin
	if stdin == nil {
		stdin = os.Stdin
	}
	stdout := options.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := options.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}
	stdoutTTY := false
	if fi, err := stdout.Stat(); err == nil {
		stdoutTTY = fi.Mode()&os.ModeCharDevice != 0
	}
	if kube.WantsRedactedStdout(args) && !stdoutTTY {
		code, out, errOut := CaptureLive(args)
		if errOut != "" {
			_, _ = io.WriteString(stderr, errOut)
		}
		_, _ = io.WriteString(stdout, kube.RedactKubectlStdout(args, out))
		if InvalidatesLiveCache(args) {
			InvalidateLive()
		}
		return code
	}
	name := KubectlBin()
	argv := append([]string{}, args...)
	if script := os.Getenv("KUBE_SHELL_NODE_SCRIPT"); script != "" {
		argv = append([]string{script}, args...)
	}
	cmd := exec.Command(name, argv...)
	switch {
	case options.StdinReader != nil:
		cmd.Stdin = options.StdinReader
	case options.InheritStdin:
		cmd.Stdin = stdin
	default:
		if fi, err := stdin.Stat(); err == nil && fi.Mode()&os.ModeCharDevice != 0 {
			cmd.Stdin = stdin
		} else {
			cmd.Stdin = nil
		}
	}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	var err error
	withChildInterrupt(func() { err = cmd.Run() })
	if InvalidatesLiveCache(args) {
		InvalidateLive()
	}
	if err == nil {
		return 0
	}
	if exit, ok := err.(*exec.ExitError); ok {
		return exit.ExitCode()
	}
	_, _ = io.WriteString(stderr, "kubectl: "+err.Error()+"\n")
	return 127
}

// ApplyLiveContext refreshes the known context list from kubeconfig without
// overwriting the shell's current context (scope may pin --context separately).
func ApplyLiveContext(cluster kube.Cluster) kube.Cluster {
	base, ok := contextCluster()
	if !ok {
		return cluster
	}
	kept := cluster.Current
	cluster.Contexts = base.Contexts
	if kept != "" {
		cluster.Current = kept
	} else {
		cluster.Current = base.Current
	}
	return cluster
}

// ProbeLive reports whether a cluster answered recently. A fresh completion
// snapshot on disk counts, including after this process forgot its memory copy.
func ProbeLive() bool {
	if memory != nil && time.Since(time.UnixMilli(memory.At)) < liveTTL && memory.Cluster.Objects != nil {
		return true
	}
	if row, ok := readFreshDisk(""); ok {
		memory = &row
		return true
	}
	code, stdout := kubectl([]string{"get", "ns", "--request-timeout=4s", "-o", "name"}, 5*time.Second)
	return code == 0 && strings.TrimSpace(stdout) != ""
}

func readFreshDisk(key string) (cacheRow, bool) {
	raw, err := os.ReadFile(cachePath())
	if err != nil {
		return cacheRow{}, false
	}
	var parsed cacheRow
	if err := json.Unmarshal(raw, &parsed); err != nil || parsed.Cluster.Objects == nil {
		return cacheRow{}, false
	}
	if time.Since(time.UnixMilli(parsed.At)) >= liveTTL {
		return cacheRow{}, false
	}
	if key != "" && parsed.Key != key {
		return cacheRow{}, false
	}
	return parsed, true
}

func readCache(key string) (kube.Cluster, bool) {
	if memory != nil && memory.Key == key && time.Since(time.UnixMilli(memory.At)) < liveTTL && memory.Cluster.Objects != nil {
		return memory.Cluster, true
	}
	parsed, ok := readFreshDisk(key)
	if !ok {
		return kube.Cluster{}, false
	}
	memory = &parsed
	return parsed.Cluster, true
}

func writeCache(key string, cluster kube.Cluster) {
	if cluster.Objects == nil {
		cluster.Objects = []kube.KObject{}
	}
	row := cacheRow{At: time.Now().UnixMilli(), Key: key, Cluster: cluster}
	memory = &row
	raw, err := json.Marshal(row)
	if err != nil {
		return
	}
	_ = kube.WritePrivate(cachePath(), string(raw))
}

func InvalidateLive() {
	memory = nil
	contextMemory = nil
	_ = os.Remove(cachePath())
}

// InvalidatesLiveCache is true when a kubectl argv can change names the
// completion snapshot holds. Reads and dry-runs keep the cache.
func InvalidatesLiveCache(args []string) bool {
	if dryRunKeeps(args) {
		return false
	}
	words := kube.PositionalWords(args).Words
	if len(words) == 0 {
		return true
	}
	verb, sub := words[0], ""
	if len(words) > 1 {
		sub = words[1]
	}
	switch verb {
	case "get", "describe", "logs", "explain", "top", "version", "api-resources", "api-versions",
		"cluster-info", "diff", "wait", "options", "help", "kustomize", "completion",
		"exec", "cp", "attach", "port-forward", "proxy":
		return false
	case "auth":
		return sub != "can-i"
	case "rollout":
		return sub != "status" && sub != "history"
	case "config":
		switch sub {
		case "view", "get-contexts", "current-context", "get-clusters", "get-users":
			return false
		}
		return true
	default:
		return true
	}
}

func dryRunKeeps(args []string) bool {
	for _, arg := range args {
		if arg == "--dry-run" {
			return true
		}
		if strings.HasPrefix(arg, "--dry-run=") && arg != "--dry-run=none" {
			return true
		}
	}
	return false
}

type configView struct {
	CurrentContext string `json:"current-context"`
	Contexts       []struct {
		Name    string `json:"name"`
		Context struct {
			Namespace string `json:"namespace"`
			Cluster   string `json:"cluster"`
			User      string `json:"user"`
		} `json:"context"`
	} `json:"contexts"`
}

func contextCluster() (kube.Cluster, bool) {
	if contextMemory != nil && time.Since(contextMemory.at) < time.Minute {
		return contextMemory.cluster, true
	}
	code, stdout := kubectl([]string{"config", "view", "-o", "json"}, 4*time.Second)
	if code != 0 || strings.TrimSpace(stdout) == "" {
		return kube.Cluster{}, false
	}
	var parsed configView
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		return kube.Cluster{}, false
	}
	current := strings.TrimSpace(parsed.CurrentContext)
	if current == "" {
		return kube.Cluster{}, false
	}
	_, namesOut := kubectl([]string{"config", "get-contexts", "-o", "name"}, 4*time.Second)
	var names []string
	for _, line := range strings.Split(namesOut, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			names = append(names, line)
		}
	}
	if !kube.ContainsString(names, current) {
		names = append([]string{current}, names...)
	}
	nsOf := func(name string) string {
		for _, item := range parsed.Contexts {
			if item.Name == name && item.Context.Namespace != "" {
				return item.Context.Namespace
			}
		}
		return "default"
	}
	contexts := make([]kube.Context, 0, len(names))
	for _, name := range names {
		clusterName, user := name, name
		for _, item := range parsed.Contexts {
			if item.Name == name {
				if item.Context.Cluster != "" {
					clusterName = item.Context.Cluster
				}
				if item.Context.User != "" {
					user = item.Context.User
				}
			}
		}
		contexts = append(contexts, kube.Context{Name: name, Cluster: clusterName, User: user, Namespace: nsOf(name)})
	}
	cluster := kube.Cluster{Current: current, Contexts: contexts, Namespaces: []kube.Namespace{}, Nodes: []kube.Node{}, PVs: []kube.PV{}, Events: []kube.Event{}, Objects: []kube.KObject{}}
	contextMemory = &struct {
		at      time.Time
		cluster kube.Cluster
	}{at: time.Now(), cluster: cluster}
	return cluster, true
}

func contextArgs(contextName, current string) []string {
	if contextName != "" && contextName != current {
		return []string{"--context", contextName}
	}
	return nil
}

func itemsOf(args []string) []kItem {
	code, stdout := kubectl(append(append([]string{}, args...), "-o", "json", "--request-timeout=6s"), 8*time.Second)
	if code != 0 || strings.TrimSpace(stdout) == "" {
		return nil
	}
	var parsed struct {
		Items []kItem `json:"items"`
	}
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		return nil
	}
	return parsed.Items
}

func AgeOf(timestamp string, now time.Time) string {
	if timestamp == "" {
		return ""
	}
	then, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return ""
	}
	seconds := int(now.Sub(then).Round(time.Second) / time.Second)
	if seconds < 0 {
		seconds = 0
	}
	if seconds < 90 {
		return strconv.Itoa(seconds) + "s"
	}
	minutes := (seconds + 30) / 60
	if minutes < 90 {
		return strconv.Itoa(minutes) + "m"
	}
	hours := (minutes + 30) / 60
	if hours < 48 {
		return strconv.Itoa(hours) + "h"
	}
	return strconv.Itoa((hours+12)/24) + "d"
}

func detailOf(item kItem) string {
	if item.Status != nil {
		for _, container := range item.Status.ContainerStatuses {
			if container.State != nil && container.State.Waiting != nil && container.State.Waiting.Reason != "" {
				return container.State.Waiting.Reason
			}
		}
	}
	switch item.Kind {
	case "Deployment", "ReplicaSet", "StatefulSet":
		ready, replicas := 0, 0
		if item.Status != nil {
			ready, replicas = item.Status.ReadyReplicas, item.Status.Replicas
		}
		return strconv.Itoa(ready) + "/" + strconv.Itoa(replicas)
	case "Service":
		if item.Spec != nil && item.Spec.Type != "" {
			return item.Spec.Type
		}
		return "Service"
	case "Secret":
		if item.Type != "" {
			return item.Type
		}
		return "Secret"
	case "CronJob":
		if item.Spec != nil && item.Spec.Schedule != "" {
			return item.Spec.Schedule
		}
		return "CronJob"
	case "Node":
		if item.Status != nil && item.Status.NodeInfo != nil && item.Status.NodeInfo.KubeletVersion != "" {
			return item.Status.NodeInfo.KubeletVersion
		}
		return "Ready"
	case "PersistentVolume":
		storage, phase := "", ""
		if item.Spec != nil && item.Spec.Capacity != nil {
			storage = item.Spec.Capacity.Storage
		}
		if item.Status != nil {
			phase = item.Status.Phase
		}
		return strings.TrimSpace(storage + " " + phase)
	default:
		if item.Status != nil && item.Status.Phase != "" {
			return item.Status.Phase
		}
		return item.Kind
	}
}

func MapListItem(item kItem) (kube.KObject, bool) {
	kind := kindMap[item.Kind]
	if kind == "" || item.Metadata == nil || item.Metadata.Name == "" {
		return kube.KObject{}, false
	}
	ns := item.Metadata.Namespace
	if ns == "" {
		ns = "default"
	}
	labels := item.Metadata.Labels
	if labels == nil {
		labels = map[string]string{}
	}
	object := kube.KObject{
		Kind:   kind,
		Name:   item.Metadata.Name,
		NS:     ns,
		Age:    AgeOf(item.Metadata.CreationTimestamp, time.Now()),
		Labels: labels,
		Status: detailOf(item),
	}
	if item.Kind == "Service" && item.Spec != nil {
		object.SvcType = item.Spec.Type
	}
	if item.Spec != nil {
		for _, container := range item.Spec.Containers {
			if container.Name != "" {
				object.Containers = append(object.Containers, kube.Container{Name: container.Name})
			}
		}
	}
	return object, true
}

func MapNode(item kItem) (kube.Node, bool) {
	if item.Metadata == nil || item.Metadata.Name == "" {
		return kube.Node{}, false
	}
	if item.Kind != "" && item.Kind != "Node" {
		return kube.Node{}, false
	}
	status := "Unknown"
	if item.Status != nil {
		for _, condition := range item.Status.Conditions {
			if condition.Type == "Ready" {
				switch condition.Status {
				case "True":
					status = "Ready"
				case "False":
					status = "NotReady"
				}
			}
		}
	}
	version := ""
	if item.Status != nil && item.Status.NodeInfo != nil {
		version = item.Status.NodeInfo.KubeletVersion
	}
	schedulable := true
	if item.Spec != nil && item.Spec.Unschedulable {
		schedulable = false
	}
	created := ""
	if item.Metadata != nil {
		created = item.Metadata.CreationTimestamp
	}
	return kube.Node{
		Name: item.Metadata.Name, Status: status, Age: AgeOf(created, time.Now()),
		Version: version, Schedulable: schedulable,
	}, true
}

func mapEvent(item kItem, fallbackNS string) (kube.Event, bool) {
	if item.Kind != "Event" {
		return kube.Event{}, false
	}
	object := ""
	ns := fallbackNS
	if item.InvolvedObject != nil {
		if item.InvolvedObject.Name != "" {
			kind := item.InvolvedObject.Kind
			if kind == "" {
				kind = "object"
			}
			object = strings.ToLower(kind) + "/" + item.InvolvedObject.Name
		}
		if item.InvolvedObject.Namespace != "" {
			ns = item.InvolvedObject.Namespace
		}
	}
	if item.Metadata != nil && item.Metadata.Namespace != "" {
		ns = item.Metadata.Namespace
	}
	created := ""
	if item.Metadata != nil {
		created = item.Metadata.CreationTimestamp
	}
	return kube.Event{NS: ns, Last: AgeOf(created, time.Now()), Type: item.Type, Reason: item.Reason, Object: object, Message: item.Message}, true
}

func ParseProjected(stdout string) []kItem {
	if strings.TrimSpace(stdout) == "" {
		return nil
	}
	var items []kItem
	for _, line := range strings.Split(stdout, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) != 20 {
			continue
		}
		labels := map[string]string{}
		if cols[19] != "" {
			for _, pair := range strings.Split(cols[19], ",") {
				if pair == "" {
					continue
				}
				eq := strings.Index(pair, "=")
				if eq >= 0 {
					labels[pair[:eq]] = pair[eq+1:]
				}
			}
		}
		items = append(items, projectedItem(cols, labels))
	}
	return items
}

func projectedItem(cols []string, labels map[string]string) kItem {
	var containers []struct {
		Name string `json:"name"`
	}
	if cols[18] != "" {
		for _, name := range strings.Split(cols[18], ",") {
			if name != "" {
				containers = append(containers, struct {
					Name string `json:"name"`
				}{Name: name})
			}
		}
	}
	var statuses []struct {
		State *struct {
			Waiting *struct {
				Reason string `json:"reason"`
			} `json:"waiting"`
		} `json:"state"`
	}
	if cols[17] != "" {
		for _, reason := range strings.Split(cols[17], ",") {
			if reason == "" {
				continue
			}
			statuses = append(statuses, struct {
				State *struct {
					Waiting *struct {
						Reason string `json:"reason"`
					} `json:"waiting"`
				} `json:"state"`
			}{State: &struct {
				Waiting *struct {
					Reason string `json:"reason"`
				} `json:"waiting"`
			}{Waiting: &struct {
				Reason string `json:"reason"`
			}{Reason: reason}}})
		}
	}
	ready, _ := strconv.Atoi(cols[6])
	replicas, _ := strconv.Atoi(cols[7])
	if cols[6] == "" {
		ready = 0
	}
	item := kItem{
		Kind: cols[0],
		Metadata: &struct {
			Name              string            `json:"name"`
			Namespace         string            `json:"namespace"`
			CreationTimestamp string            `json:"creationTimestamp"`
			Labels            map[string]string `json:"labels"`
		}{Name: cols[1], Namespace: cols[2], CreationTimestamp: cols[3], Labels: labels},
		Type:    cols[4],
		Reason:  cols[12],
		Message: cols[13],
		InvolvedObject: &struct {
			Kind      string `json:"kind"`
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		}{Kind: cols[14], Name: cols[15], Namespace: cols[16]},
		Spec: &struct {
			Type          string `json:"type"`
			Schedule      string `json:"schedule"`
			Unschedulable bool   `json:"unschedulable"`
			Capacity      *struct {
				Storage string `json:"storage"`
			} `json:"capacity"`
			Containers []struct {
				Name string `json:"name"`
			} `json:"containers"`
		}{Type: cols[8], Schedule: cols[9], Containers: containers},
		Status: &struct {
			Phase         string `json:"phase"`
			ReadyReplicas int    `json:"readyReplicas"`
			Replicas      int    `json:"replicas"`
			NodeInfo      *struct {
				KubeletVersion string `json:"kubeletVersion"`
			} `json:"nodeInfo"`
			Conditions []struct {
				Type   string `json:"type"`
				Status string `json:"status"`
			} `json:"conditions"`
			ContainerStatuses []struct {
				State *struct {
					Waiting *struct {
						Reason string `json:"reason"`
					} `json:"waiting"`
				} `json:"state"`
			} `json:"containerStatuses"`
		}{Phase: cols[5], ReadyReplicas: ready, Replicas: replicas, ContainerStatuses: statuses},
	}
	if cols[10] != "" {
		item.Spec.Capacity = &struct {
			Storage string `json:"storage"`
		}{Storage: cols[10]}
	}
	if cols[11] != "" {
		item.Status.NodeInfo = &struct {
			KubeletVersion string `json:"kubeletVersion"`
		}{KubeletVersion: cols[11]}
	}
	return item
}

func fetchProjectedLists(ctx, scope []string, groups [][]string) [][]kItem {
	var flat []string
	for _, group := range groups {
		flat = append(flat, group...)
	}
	code, stdout := kubectl(append(append(append([]string{}, ctx...), "get", strings.Join(flat, ",")), append(scope, "-o", ProjectedJSONPath, "--request-timeout=6s")...), 8*time.Second)
	if code == 0 {
		return [][]kItem{ParseProjected(stdout)}
	}
	var lists [][]kItem
	for _, group := range groups {
		code, stdout = kubectl(append(append(append([]string{}, ctx...), "get", strings.Join(group, ",")), append(scope, "-o", ProjectedJSONPath, "--request-timeout=6s")...), 8*time.Second)
		if code == 0 {
			lists = append(lists, ParseProjected(stdout))
			continue
		}
		for _, kind := range group {
			code, stdout = kubectl(append(append(append([]string{}, ctx...), "get", kind), append(scope, "-o", ProjectedJSONPath, "--request-timeout=6s")...), 8*time.Second)
			if code == 0 {
				lists = append(lists, ParseProjected(stdout))
			}
		}
	}
	return lists
}

func fetchEvents(ctx []string, ns string, all bool) []kube.Event {
	scope := []string{"-n", ns}
	if ns == "" {
		scope = []string{"-n", "default"}
	}
	if all {
		scope = []string{"--all-namespaces"}
	}
	fallback := ns
	if fallback == "" {
		fallback = "default"
	}
	var events []kube.Event
	for _, item := range itemsOf(append(append([]string{}, ctx...), append([]string{"get", "events"}, scope...)...)) {
		if event, ok := mapEvent(item, fallback); ok {
			events = append(events, event)
		}
	}
	return events
}

func fetchObjects(ctx []string, ns string, all bool) ([]kube.KObject, []kube.Event) {
	scope := []string{"-n", ns}
	if ns == "" {
		scope = []string{"-n", "default"}
	}
	if all {
		scope = []string{"--all-namespaces"}
	}
	var objects []kube.KObject
	for _, list := range fetchProjectedLists(ctx, scope, objectGroups) {
		for _, item := range list {
			mapped, ok := MapListItem(item)
			if !ok {
				continue
			}
			if mapped.NS == "" {
				mapped.NS = ns
				if mapped.NS == "" {
					mapped.NS = "default"
				}
			}
			objects = append(objects, mapped)
		}
	}
	return objects, fetchEvents(ctx, ns, all)
}

// LiveCluster builds a completion snapshot from kubeconfig and the API.
// The second result is false when kubectl cannot describe the current context.
func LiveCluster(engineLine string) (kube.Cluster, bool) {
	base, ok := contextCluster()
	if !ok {
		return kube.Cluster{}, false
	}
	var tokens []string
	for _, token := range kube.Tokenize(engineLine) {
		tokens = append(tokens, kube.Unquote(token.Text))
	}
	if len(tokens) == 0 {
		tokens = []string{"kubectl"}
	}
	scope := kube.ScopeOf(tokens, base)
	keyNS := scope.NS
	if scope.All {
		keyNS = "*"
	}
	key := base.Current + "|" + scope.Context + "|" + keyNS
	if cached, hit := readCache(key); hit {
		return cached, true
	}
	ctx := contextArgs(scope.Context, base.Current)
	scoped := itemsOf(append(append([]string{}, ctx...), "get", "ns,nodes,pv,storageclasses.storage.k8s.io,priorityclasses.scheduling.k8s.io,customresourcedefinitions.apiextensions.k8s.io"))
	var namespaces []kube.Namespace
	var nodes []kube.Node
	var pvs []kube.PV
	var clusterObjects []kube.KObject
	for _, item := range scoped {
		if item.Kind == "Namespace" && item.Metadata != nil && item.Metadata.Name != "" {
			status := "Active"
			if item.Status != nil && item.Status.Phase != "" {
				status = item.Status.Phase
			}
			namespaces = append(namespaces, kube.Namespace{Name: item.Metadata.Name, Status: status, Age: AgeOf(item.Metadata.CreationTimestamp, time.Now())})
		}
		if item.Kind == "Node" {
			if node, ok := MapNode(item); ok {
				nodes = append(nodes, node)
			}
		}
		if item.Kind == "PersistentVolume" && item.Metadata != nil && item.Metadata.Name != "" {
			capacity := ""
			if item.Spec != nil && item.Spec.Capacity != nil {
				capacity = item.Spec.Capacity.Storage
			}
			phase := ""
			if item.Status != nil {
				phase = item.Status.Phase
			}
			pvs = append(pvs, kube.PV{Name: item.Metadata.Name, Capacity: capacity, Status: phase, Age: AgeOf(item.Metadata.CreationTimestamp, time.Now())})
		}
		if mapped, ok := MapListItem(item); ok {
			clusterObjects = append(clusterObjects, mapped)
		}
	}
	fetched, events := fetchObjects(ctx, scope.NS, scope.All)
	if len(namespaces) == 0 {
		namespaces = base.Namespaces
	}
	objects := append([]kube.KObject{}, clusterObjects...)
	objects = append(objects, fetched...)
	if events == nil {
		events = []kube.Event{}
	}
	if nodes == nil {
		nodes = []kube.Node{}
	}
	if pvs == nil {
		pvs = []kube.PV{}
	}
	cluster := kube.Cluster{
		Current: base.Current, Contexts: base.Contexts, Namespaces: namespaces,
		Nodes: nodes, PVs: pvs, Events: events, Objects: objects,
	}
	writeCache(key, cluster)
	return cluster, true
}
