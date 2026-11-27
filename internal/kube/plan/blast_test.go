package plan

import (
	"encoding/json"
	"github.com/NadafMM/kubectl-shell/internal/kube"
	"github.com/NadafMM/kubectl-shell/internal/kube/live"
	"github.com/NadafMM/kubectl-shell/internal/kube/parse"
	"os"
	"path/filepath"
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

func TestBlastGitOpsFinalizersAndIngressWhenZeroReady(t *testing.T) {
	pod := podJSON("checkout-abc", map[string]string{"app": "checkout"}, true)
	pod["metadata"].(map[string]any)["annotations"] = map[string]any{
		"argocd.argoproj.io/tracking-id": "checkout-prod:v1/Pod:default/checkout-abc",
	}
	pod["metadata"].(map[string]any)["finalizers"] = []string{"custom.io/cleanup"}
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
			return live.KubectlJSONResult{OK: true, JSON: raw(map[string]any{"kind": "PodList", "items": []any{pod}})}
		case strings.HasPrefix(joined, "get ingresses"):
			return live.KubectlJSONResult{OK: true, JSON: raw(map[string]any{
				"kind": "IngressList",
				"items": []any{map[string]any{
					"kind":     "Ingress",
					"metadata": map[string]any{"name": "public-web", "namespace": "default"},
					"spec": map[string]any{
						"rules": []any{map[string]any{
							"host": "shop.example.com",
							"http": map[string]any{"paths": []any{map[string]any{
								"backend": map[string]any{"service": map[string]any{"name": "checkout"}},
							}}},
						}},
					},
				}},
			})}
		case strings.HasPrefix(joined, "get httproutes"):
			return live.KubectlJSONResult{OK: true, JSON: raw(map[string]any{
				"kind": "HTTPRouteList",
				"items": []any{map[string]any{
					"kind":     "HTTPRoute",
					"metadata": map[string]any{"name": "gw-route", "namespace": "default"},
					"spec": map[string]any{
						"hostnames": []string{"api.example.com"},
						"rules": []any{map[string]any{
							"backendRefs": []any{map[string]any{"kind": "Service", "name": "checkout"}},
						}},
					},
				}},
			})}
		case strings.HasPrefix(joined, "get pdb"):
			return live.KubectlJSONResult{OK: true, JSON: raw(map[string]any{"kind": "PodDisruptionBudgetList", "items": []any{}})}
		case args[0] == "delete":
			return live.KubectlJSONResult{OK: true, JSON: raw(map[string]any{"kind": "Pod", "metadata": map[string]any{"name": "checkout-abc"}})}
		default:
			t.Fatalf("unexpected %v", args)
			return live.KubectlJSONResult{}
		}
	}, nil)
	text := FormatBlast(blast)
	if len(blast.ManagedBy) != 1 || blast.ManagedBy[0].Manager != "argocd" {
		t.Fatalf("expected argocd warning, got %#v", blast.ManagedBy)
	}
	if len(blast.Ingresses) != 2 {
		t.Fatalf("expected ingress + httproute impact, got %#v", blast.Ingresses)
	}
	if !strings.Contains(text, "ingress/public-web (shop.example.com)") || !strings.Contains(text, "httproute/gw-route (api.example.com)") || !strings.Contains(text, "managed by argocd") || !strings.Contains(text, "finalizers") {
		t.Fatalf("missing output lines: %s", text)
	}
}

func TestBlastPVCReclaimAndMount(t *testing.T) {
	plan := PlanCommand("kubectl delete pvc db-data", kube.InitialCluster())
	blast := BlastRadius(plan, []string{"delete", "pvc", "db-data"}, func(args []string) live.KubectlJSONResult {
		joined := strings.Join(args, " ")
		switch {
		case strings.HasPrefix(joined, "get pvc db-data"):
			return live.KubectlJSONResult{OK: true, JSON: raw(map[string]any{
				"kind":     "PersistentVolumeClaim",
				"metadata": map[string]any{"name": "db-data", "namespace": "default"},
				"spec":     map[string]any{"volumeName": "pv-123"},
			})}
		case strings.HasPrefix(joined, "get pv "):
			return live.KubectlJSONResult{OK: true, JSON: raw(map[string]any{
				"kind": "PersistentVolumeList",
				"items": []any{map[string]any{
					"kind":     "PersistentVolume",
					"metadata": map[string]any{"name": "pv-123"},
					"spec":     map[string]any{"persistentVolumeReclaimPolicy": "Delete"},
				}},
			})}
		case strings.HasPrefix(joined, "get pods"):
			return live.KubectlJSONResult{OK: true, JSON: raw(map[string]any{
				"kind": "PodList",
				"items": []any{map[string]any{
					"kind":     "Pod",
					"metadata": map[string]any{"name": "postgres-0", "namespace": "default"},
					"spec": map[string]any{
						"volumes": []any{map[string]any{
							"name":                  "data",
							"persistentVolumeClaim": map[string]any{"claimName": "db-data"},
						}},
					},
					"status": map[string]any{"phase": "Running"},
				}},
			})}
		case args[0] == "delete":
			return live.KubectlJSONResult{OK: true, JSON: raw(map[string]any{"kind": "PersistentVolumeClaim", "metadata": map[string]any{"name": "db-data"}})}
		default:
			t.Fatalf("unexpected %v", args)
			return live.KubectlJSONResult{}
		}
	}, nil)
	if len(blast.PVCs) != 1 || blast.PVCs[0].ReclaimPolicy != "Delete" || len(blast.PVCs[0].MountedBy) != 1 {
		t.Fatalf("unexpected pvc blast: %#v", blast.PVCs)
	}
	if !strings.Contains(FormatBlast(blast), "reclaim=Delete mounted by postgres-0") {
		t.Fatal(FormatBlast(blast))
	}
}

func TestBlastDrainCapacity(t *testing.T) {
	plan := PlanCommand("kubectl drain node-1", kube.InitialCluster())
	blast := BlastRadius(plan, []string{"drain", "node-1"}, func(args []string) live.KubectlJSONResult {
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "spec.nodeName=node-1"):
			return live.KubectlJSONResult{OK: true, JSON: raw(map[string]any{
				"kind": "PodList",
				"items": []any{map[string]any{
					"kind":     "Pod",
					"metadata": map[string]any{"name": "heavy-0", "namespace": "default"},
					"spec": map[string]any{
						"volumes": []any{map[string]any{"name": "scratch", "emptyDir": map[string]any{}}},
						"containers": []any{map[string]any{
							"name":      "app",
							"resources": map[string]any{"requests": map[string]any{"cpu": "4000m", "memory": "4Gi"}},
						}},
					},
				}},
			})}
		case strings.HasPrefix(joined, "get nodes"):
			return live.KubectlJSONResult{OK: true, JSON: raw(map[string]any{
				"kind": "NodeList",
				"items": []any{
					map[string]any{
						"kind":     "Node",
						"metadata": map[string]any{"name": "node-1"},
						"status":   map[string]any{"conditions": []any{map[string]any{"type": "Ready", "status": "True"}}},
					},
					map[string]any{
						"kind":     "Node",
						"metadata": map[string]any{"name": "node-2"},
						"status": map[string]any{
							"allocatable": map[string]any{"cpu": "2000m", "memory": "2Gi"},
							"conditions":  []any{map[string]any{"type": "Ready", "status": "True"}},
						},
					},
				},
			})}
		case strings.HasPrefix(joined, "get services") || strings.HasPrefix(joined, "get pdb"):
			return live.KubectlJSONResult{OK: true, JSON: raw(map[string]any{"kind": "List", "items": []any{}})}
		default:
			t.Fatalf("unexpected %v", args)
			return live.KubectlJSONResult{}
		}
	}, nil)
	if blast.Drain == nil || blast.Drain.SchedulableNodesRemaining != 1 || blast.Drain.EmptyDirPods != 1 {
		t.Fatalf("unexpected drain blast: %#v", blast.Drain)
	}
	text := FormatBlast(blast)
	if !strings.Contains(text, "1 schedulable nodes remaining") || !strings.Contains(text, "evicted CPU requests") || !strings.Contains(text, "emptyDir/hostPath") {
		t.Fatal(text)
	}
}

func TestBlastApplyConfigMapWarnsReferencingDeploymentAndDiff(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(dir, "cm.yaml")
	if err := os.WriteFile(manifest, []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: app-cfg\n  namespace: default\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	plan := PlanCommand("kubectl apply -f "+manifest, kube.InitialCluster())
	blast := BlastRadiusWithOptions(plan, []string{"apply", "-f", manifest}, func(args []string) live.KubectlJSONResult {
		joined := strings.Join(args, " ")
		switch {
		case strings.HasPrefix(joined, "get configmap app-cfg"):
			return live.KubectlJSONResult{OK: true, JSON: raw(map[string]any{
				"kind": "ConfigMap",
				"metadata": map[string]any{
					"name": "app-cfg", "namespace": "default",
					"labels": map[string]any{"app.kubernetes.io/managed-by": "Helm", "app.kubernetes.io/instance": "web"},
				},
			})}
		case strings.HasPrefix(joined, "get deployments.apps,statefulsets.apps"):
			return live.KubectlJSONResult{OK: true, JSON: raw(map[string]any{
				"kind": "List",
				"items": []any{map[string]any{
					"kind":     "Deployment",
					"metadata": map[string]any{"name": "web", "namespace": "default"},
					"spec": map[string]any{
						"template": map[string]any{
							"spec": map[string]any{
								"containers": []any{map[string]any{
									"name":    "app",
									"envFrom": []any{map[string]any{"configMapRef": map[string]any{"name": "app-cfg"}}},
								}},
							},
						},
					},
				}},
			})}
		default:
			t.Fatalf("unexpected %v", args)
			return live.KubectlJSONResult{}
		}
	}, nil, BlastOptions{
		RunDiff: func(args []string) (int, string, string) {
			return 1, "--- live\n+++ local\n-  FOO: old\n+  FOO: new\n", ""
		},
	})
	text := FormatBlast(blast)
	if !strings.Contains(text, "managed by helm (web)") || !strings.Contains(text, "does not auto-restart referencing deployment/web") || !strings.Contains(text, "diff: +1 -1 lines") {
		t.Fatalf("unexpected apply preflight output:\n%s", text)
	}
}
