package undo

import (
	"encoding/json"
	"github.com/NadafMM/kubectl-shell/internal/kube"
	"github.com/NadafMM/kubectl-shell/internal/kube/live"
	"github.com/NadafMM/kubectl-shell/internal/kube/parse"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const undoTTL = 24 * time.Hour

var snapshotVerbs = map[string]bool{
	"apply": true, "replace": true, "patch": true, "scale": true,
	"label": true, "annotate": true, "set": true,
}

var undoWorkloads = map[string]bool{
	"deployment": true, "statefulset": true, "daemonset": true,
}

type UndoRecord struct {
	ID        string          `json:"id"`
	At        int64           `json:"at"`
	Verb      string          `json:"verb"`
	Kind      string          `json:"kind"`
	Name      string          `json:"name"`
	Namespace string          `json:"namespace"`
	Context   string          `json:"context"`
	Revision  string          `json:"revision,omitempty"`
	ManagedBy string          `json:"managedBy,omitempty"`
	Release   string          `json:"release,omitempty"`
	Snapshot  json.RawMessage `json:"snapshot,omitempty"`
	Refused   string          `json:"refused,omitempty"`
}

func undoDir() string {
	return filepath.Join(kube.StateDir(), "undo")
}

func singularKind(token string) string {
	if resource, ok := parse.ResolveResource(token); ok {
		return resource.Singular
	}
	return strings.ToLower(token)
}

// NamedTarget picks a single named object from a mutating kubectl argv.
func NamedTarget(argv []string) (verb, kind, name string, ok bool) {
	words := parse.PositionalWords(argv).Words
	if len(words) == 0 || !snapshotVerbs[words[0]] {
		return "", "", "", false
	}
	verb = words[0]
	token, plain := "", ""
	if verb == "set" {
		if len(words) > 2 {
			token = words[2]
		}
		if len(words) > 3 {
			plain = words[3]
		}
	} else {
		if len(words) > 1 {
			token = words[1]
		}
		if len(words) > 2 {
			plain = words[2]
		}
	}
	if token == "" {
		return "", "", "", false
	}
	if strings.Contains(token, "/") {
		parts := strings.SplitN(token, "/", 2)
		if parts[0] == "" || parts[1] == "" {
			return "", "", "", false
		}
		return verb, singularKind(parts[0]), parts[1], true
	}
	if plain == "" || strings.HasPrefix(plain, "-") {
		return "", "", "", false
	}
	return verb, singularKind(token), plain, true
}

// CleanSnapshot drops status and server metadata so a restore can apply.
func CleanSnapshot(value any) any {
	raw, err := json.Marshal(value)
	if err != nil {
		return value
	}
	var copy map[string]any
	if err := json.Unmarshal(raw, &copy); err != nil {
		return value
	}
	delete(copy, "status")
	if meta, ok := copy["metadata"].(map[string]any); ok {
		for _, key := range []string{"resourceVersion", "uid", "creationTimestamp", "generation", "managedFields"} {
			delete(meta, key)
		}
	}
	return copy
}

func namespaceFromArgv(argv []string, cluster kube.Cluster) string {
	if ns, ok := parse.FlagValue(argv, []string{"-n", "--namespace"}); ok && ns != "" {
		return ns
	}
	return kube.NamespaceOf(cluster)
}

// RememberUndo stores a pre-change snapshot for a named mutating command.
func RememberUndo(argv []string, cluster kube.Cluster, id string, now time.Time) {
	verb, kind, name, ok := NamedTarget(argv)
	if !ok {
		return
	}
	ns := namespaceFromArgv(argv, cluster)
	base := UndoRecord{
		ID: id, At: now.UnixMilli(), Verb: verb, Kind: kind, Name: name,
		Namespace: ns, Context: cluster.Current,
	}
	if kind == "secret" || kind == "configmap" {
		base.Refused = kind + " objects are not snapshotted"
		raw, _ := json.Marshal(base)
		_ = kube.WritePrivate(filepath.Join(undoDir(), id+".json"), string(raw))
		return
	}
	getArgs := []string{"get", kind, name, "-n", ns, "-o", "json"}
	if cluster.Current != "" {
		getArgs = append(getArgs, "--context", cluster.Current)
	}
	code, stdout, _ := live.CaptureLive(getArgs)
	if code != 0 || strings.TrimSpace(stdout) == "" {
		return
	}
	var live map[string]any
	if err := json.Unmarshal([]byte(stdout), &live); err != nil {
		return
	}
	meta, _ := live["metadata"].(map[string]any)
	if meta == nil {
		return
	}
	if _, ok := meta["name"].(string); !ok {
		return
	}
	cleaned, err := json.Marshal(CleanSnapshot(live))
	if err != nil {
		return
	}
	base.Snapshot = cleaned
	labels := toStringMap(meta["labels"])
	annotations := toStringMap(meta["annotations"])
	base.ManagedBy, base.Release = kube.DetectGitOpsFromMeta(labels, annotations)
	if kind == "deployment" {
		if revision := annotations["deployment.kubernetes.io/revision"]; revision != "" {
			base.Revision = revision
		}
	}
	raw, err := json.Marshal(base)
	if err != nil {
		return
	}
	_ = kube.WritePrivate(filepath.Join(undoDir(), id+".json"), string(raw))
	pruneUndo(now)
}

// ListUndo returns recent undo records, newest first.
func ListUndo(now time.Time) []UndoRecord {
	entries, err := os.ReadDir(undoDir())
	if err != nil {
		return nil
	}
	var records []UndoRecord
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(undoDir(), entry.Name()))
		if err != nil {
			continue
		}
		var record UndoRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			continue
		}
		if now.UnixMilli()-record.At <= undoTTL.Milliseconds() {
			records = append(records, record)
		}
	}
	sort.Slice(records, func(i, j int) bool { return records[i].At > records[j].At })
	if len(records) > 20 {
		records = records[:20]
	}
	return records
}

// LatestUndo returns the newest record, if any.
func LatestUndo(now time.Time) (UndoRecord, bool) {
	records := ListUndo(now)
	if len(records) == 0 {
		return UndoRecord{}, false
	}
	return records[0], true
}

// UndoRefusal explains why a record cannot be restored.
func UndoRefusal(record UndoRecord) string {
	if record.Kind == "secret" || record.Kind == "configmap" {
		return "kube undo does not restore Secrets or ConfigMaps. They are not snapshotted, because a restore without their data would destroy it."
	}
	if record.Kind == "pod" {
		return "kube undo cannot restore a Pod. A controller recreates pods."
	}
	return ""
}

// UndoGitOpsWarning returns a drift warning when the saved resource is owned by Argo CD, Flux, or Helm.
func UndoGitOpsWarning(record UndoRecord) string {
	if record.ManagedBy == "" {
		return ""
	}
	rel := ""
	if record.Release != "" {
		rel = " (" + record.Release + ")"
	}
	return "warning: " + record.Kind + "/" + record.Name + " is managed by " + record.ManagedBy + rel + "; manual undo may drift or be reverted — prefer reverting in Git"
}

func toStringMap(v any) map[string]string {
	raw, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	out := make(map[string]string, len(raw))
	for k, val := range raw {
		if s, ok := val.(string); ok {
			out[k] = s
		}
	}
	return out
}

// RestoreArgs builds the kubectl argv that undoes a record.
func RestoreArgs(record UndoRecord) []string {
	var args []string
	if record.Revision != "" && undoWorkloads[record.Kind] {
		args = []string{
			"rollout", "undo", record.Kind + "/" + record.Name,
			"-n", record.Namespace, "--to-revision=" + record.Revision,
		}
	} else {
		args = []string{"apply", "-n", record.Namespace, "-f", "-"}
	}
	if record.Context != "" {
		args = append(args, "--context", record.Context)
	}
	return args
}

func pruneUndo(now time.Time) {
	keep := map[string]bool{}
	for _, record := range ListUndo(now) {
		keep[record.ID] = true
	}
	entries, err := os.ReadDir(undoDir())
	if err != nil {
		return
	}
	for _, entry := range entries {
		id := strings.TrimSuffix(entry.Name(), ".json")
		if !keep[id] {
			_ = os.Remove(filepath.Join(undoDir(), entry.Name()))
		}
	}
}
