package plan

import (
	"encoding/json"
	"github.com/NadafMM/kubectl-shell/internal/kube"
	"github.com/NadafMM/kubectl-shell/internal/kube/live"
	"github.com/NadafMM/kubectl-shell/internal/kube/parse"
	"strings"
	"testing"
)

func raw(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

func podJSON(name string, labels map[string]string, ready bool) map[string]any {
	status := "False"
	if ready {
		status = "True"
	}
	return map[string]any{
		"kind": "Pod",
		"metadata": map[string]any{
			"name":      name,
			"namespace": "default",
			"labels":    labels,
		},
		"status": map[string]any{
			"conditions": []map[string]any{{"type": "Ready", "status": status}},
		},
	}
}

func serviceJSON(name string, selector map[string]any, ns string) map[string]any {
	if ns == "" {
		ns = "default"
	}
	item := map[string]any{
		"kind": "Service",
		"metadata": map[string]any{
			"name":      name,
			"namespace": ns,
		},
		"spec": map[string]any{},
	}
	if selector != nil {
		item["spec"] = map[string]any{"selector": selector}
	}
	return item
}

func TestBlastSkipsGet(t *testing.T) {
	plan := PlanCommand("kubectl get pods", kube.InitialCluster())
	calls := 0
	blast := BlastRadius(plan, []string{"get", "pods"}, func([]string) live.KubectlJSONResult {
		calls++
		return live.KubectlJSONResult{OK: false, Error: "no"}
	}, nil)
	if blast.Status != "skipped" || blast.Reason != "not a delete" || FormatBlast(blast) != "" || calls != 0 {
		t.Fatalf("%#v calls=%d", blast, calls)
	}
}

func TestBlastNoName(t *testing.T) {
	for _, args := range [][]string{{"delete"}, {"delete", "pod"}} {
		plan := PlanCommand("kubectl "+strings.Join(args, " "), kube.InitialCluster())
		blast := BlastRadius(plan, args, func([]string) live.KubectlJSONResult {
			t.Fatal("kubectl")
			return live.KubectlJSONResult{}
		}, nil)
		if blast.Status != "skipped" || blast.Reason != "no name or selector" {
			t.Fatal(blast)
		}
		if FormatBlast(blast) != "blast: not expanded (no name or selector)" {
			t.Fatal(FormatBlast(blast))
		}
	}
}

func TestBlastKustomizeSkipped(t *testing.T) {
	for _, args := range [][]string{{"delete", "-k", "dir"}, {"delete", "--kustomize=dir"}} {
		plan := PlanCommand("kubectl "+strings.Join(args, " "), kube.InitialCluster())
		blast := BlastRadius(plan, args, func([]string) live.KubectlJSONResult {
			t.Fatal("kubectl")
			return live.KubectlJSONResult{}
		}, nil)
		if blast.Reason != "kustomize delete (-k) is not expanded" {
			t.Fatal(args, blast.Reason)
		}
	}
}

func TestBlastUnknownTarget(t *testing.T) {
	plan := PlanCommand("kubectl delete pod missing", kube.InitialCluster())
	blast := BlastRadius(plan, []string{"delete", "pod", "missing"}, func([]string) live.KubectlJSONResult {
		return live.KubectlJSONResult{OK: false, Error: `pods "missing" not found`}
	}, nil)
	if blast.Status != "unknown" || blast.Reason != `kubectl get pod: pods "missing" not found` {
		t.Fatal(blast)
	}
	if FormatBlast(blast) != `blast: unknown (kubectl get pod: pods "missing" not found)` {
		t.Fatal(FormatBlast(blast))
	}
}

func TestBlastConfigMapNoServices(t *testing.T) {
	plan := PlanCommand("kubectl delete configmap app", kube.InitialCluster())
	var gets int
	blast := BlastRadius(plan, []string{"delete", "configmap", "app"}, func(args []string) live.KubectlJSONResult {
		if args[0] == "get" {
			gets++
			return live.KubectlJSONResult{OK: true, JSON: raw(map[string]any{
				"kind": "ConfigMap", "metadata": map[string]any{"name": "app", "namespace": "default"},
			})}
		}
		return live.KubectlJSONResult{OK: true, JSON: raw(map[string]any{"kind": "ConfigMap", "metadata": map[string]any{"name": "app"}})}
	}, nil)
	if blast.Status != "ok" || blast.ObjectCount != 1 || gets != 1 {
		t.Fatalf("%#v gets=%d", blast, gets)
	}
	if len(blast.Objects) != 1 || blast.Objects[0].Kind != "configmap" || blast.Objects[0].Name != "app" {
		t.Fatal(blast.Objects)
	}
	text := FormatBlast(blast)
	if !strings.Contains(text, "1 configmap") || !strings.Contains(text, "configmap/app") {
		t.Fatal(text)
	}
}

func TestBlastServiceReadyPods(t *testing.T) {
	pod := podJSON("checkout-abc", map[string]string{"app": "checkout"}, true)
	sibling := podJSON("checkout-def", map[string]string{"app": "checkout"}, true)
	plan := PlanCommand("kubectl delete pod checkout-abc", kube.InitialCluster())
	blast := BlastRadius(plan, []string{"delete", "pod", "checkout-abc"}, func(args []string) live.KubectlJSONResult {
		joined := strings.Join(args, " ")
		switch {
		case strings.HasPrefix(joined, "get pod checkout-abc"):
			return live.KubectlJSONResult{OK: true, JSON: raw(pod)}
		case strings.HasPrefix(joined, "get services"):
			return live.KubectlJSONResult{OK: true, JSON: raw(map[string]any{
				"kind":  "ServiceList",
				"items": []any{serviceJSON("checkout", map[string]any{"app": "checkout"}, "")},
			})}
		case strings.HasPrefix(joined, "get pods -n default"):
			return live.KubectlJSONResult{OK: true, JSON: raw(map[string]any{"kind": "PodList", "items": []any{pod, sibling}})}
		case strings.HasPrefix(joined, "get pdb"):
			return live.KubectlJSONResult{OK: true, JSON: raw(map[string]any{"kind": "PodDisruptionBudgetList", "items": []any{}})}
		case args[0] == "delete":
			return live.KubectlJSONResult{OK: true, JSON: raw(map[string]any{"kind": "Pod", "metadata": map[string]any{"name": "checkout-abc"}})}
		default:
			t.Fatalf("unexpected %v", args)
			return live.KubectlJSONResult{}
		}
	}, nil)
	if blast.Status != "ok" || len(blast.Services) != 1 {
		t.Fatal(blast)
	}
	if blast.Services[0].MatchedPods != 1 || blast.Services[0].ReadyPodsRemaining != 1 {
		t.Fatal(blast.Services)
	}
	if !strings.Contains(FormatBlast(blast), "would keep 1 ready pod") {
		t.Fatal(FormatBlast(blast))
	}
}

func TestBlastMixedSlashSkipped(t *testing.T) {
	plan := PlanCommand("kubectl delete pod/a deploy/b", kube.InitialCluster())
	blast := BlastRadius(plan, []string{"delete", "pod/a", "deploy/b"}, func([]string) live.KubectlJSONResult {
		t.Fatal("kubectl")
		return live.KubectlJSONResult{}
	}, nil)
	if blast.Reason != "mixed resource kinds" {
		t.Fatal(blast)
	}
}

func TestBlastRemoteManifestSkipped(t *testing.T) {
	plan := PlanCommand("kubectl delete -f https://example.com/a.yaml", kube.InitialCluster())
	blast := BlastRadius(plan, []string{"delete", "-f", "https://example.com/a.yaml"}, func([]string) live.KubectlJSONResult {
		t.Fatal("kubectl")
		return live.KubectlJSONResult{}
	}, nil)
	if blast.Reason != "remote manifest URLs are not expanded" {
		t.Fatal(blast)
	}
}

func TestParseManifestYAML(t *testing.T) {
	refs := parse.ParseManifest("kind: Deployment\nmetadata:\n  name: web\n  namespace: app\n")
	if len(refs) != 1 || refs[0].Kind != "Deployment" || refs[0].Name != "web" || refs[0].Namespace != "app" {
		t.Fatal(refs)
	}
}
