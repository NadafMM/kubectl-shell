package plan

import (
	"strconv"
	"strings"
	"testing"

	"github.com/NadafMM/kubectl-shell/internal/kube"
)

func checkout(status any, spec ...any) map[string]any {
	body := map[string]any{
		"kind": "Pod",
		"metadata": map[string]any{
			"name":      "checkout",
			"namespace": "default",
		},
		"status": status,
	}
	if len(spec) > 0 {
		body["spec"] = spec[0]
	} else {
		body["spec"] = map[string]any{
			"containers": []any{
				map[string]any{
					"name":  "app",
					"image": "nginx:missing",
					"resources": map[string]any{
						"limits": map[string]any{"memory": "64Mi"},
					},
				},
			},
		}
	}
	return body
}

func mustDiagnose(t *testing.T, found *Diagnosis) *Diagnosis {
	t.Helper()
	if found == nil {
		t.Fatal("expected a diagnosis")
	}
	return found
}

func TestDiagnoseOOMKilled(t *testing.T) {
	found := mustDiagnose(t, DiagnosePod(checkout(map[string]any{
		"phase": "Running",
		"containerStatuses": []any{
			map[string]any{
				"name":  "app",
				"ready": false,
				"state": map[string]any{"waiting": map[string]any{"reason": "CrashLoopBackOff"}},
				"lastState": map[string]any{
					"terminated": map[string]any{"reason": "OOMKilled", "exitCode": float64(137)},
				},
			},
		},
	}), nil, nil))
	if found.Pod != "checkout" || found.Namespace != "default" || len(found.Findings) != 1 {
		t.Fatalf("%#v", found)
	}
	if found.Findings[0].Rule != FindingOOM {
		t.Fatal(found.Findings[0].Rule)
	}
	if found.Findings[0].Cause != "container app was OOMKilled (exit 137); memory limit 64Mi" {
		t.Fatal(found.Findings[0].Cause)
	}
}

func TestDiagnoseExit137IsOOM(t *testing.T) {
	found := mustDiagnose(t, DiagnosePod(checkout(map[string]any{
		"phase": "Running",
		"containerStatuses": []any{
			map[string]any{
				"name":  "app",
				"ready": false,
				"state": map[string]any{"waiting": map[string]any{"reason": "CrashLoopBackOff"}},
				"lastState": map[string]any{
					"terminated": map[string]any{"reason": "Error", "exitCode": float64(137)},
				},
			},
		},
	}), nil, nil))
	if found.Findings[0].Rule != FindingOOM {
		t.Fatal(found.Findings)
	}
}

func TestDiagnoseCrashLogTail(t *testing.T) {
	lines := []string{"first"}
	for i := 1; i <= 30; i++ {
		lines = append(lines, "l"+strconv.Itoa(i)+" password=secret")
	}
	found := mustDiagnose(t, DiagnosePod(checkout(map[string]any{
		"phase": "Running",
		"containerStatuses": []any{
			map[string]any{
				"name":  "app",
				"ready": false,
				"state": map[string]any{"waiting": map[string]any{"reason": "CrashLoopBackOff"}},
				"lastState": map[string]any{
					"terminated": map[string]any{"reason": "Error", "exitCode": float64(1)},
				},
			},
		},
	}), nil, map[string]string{"app": strings.Join(lines, "\n")}))
	cause := found.Findings[0].Cause
	if !strings.HasPrefix(cause, "container app exited 1; ") {
		t.Fatal(cause)
	}
	if strings.Contains(cause, "first") || strings.Contains(cause, "password=secret") {
		t.Fatal(cause)
	}
	if !strings.Contains(cause, "l30 password=***") {
		t.Fatal(cause)
	}
}

func TestDiagnoseImagePull(t *testing.T) {
	found := mustDiagnose(t, DiagnosePod(checkout(map[string]any{
		"phase": "Pending",
		"containerStatuses": []any{
			map[string]any{
				"name":  "app",
				"ready": false,
				"state": map[string]any{
					"waiting": map[string]any{"reason": "ImagePullBackOff", "message": "manifest unknown"},
				},
			},
		},
	}), []any{
		map[string]any{"reason": "Failed", "message": "manifest unknown", "involvedObject": map[string]any{"name": "checkout"}},
	}, nil))
	if found.Findings[0].Cause != "image nginx:missing failed to pull (tag not found); manifest unknown" {
		t.Fatal(found.Findings[0].Cause)
	}
}

func TestDiagnoseConfigSecretKey(t *testing.T) {
	found := mustDiagnose(t, DiagnosePod(checkout(map[string]any{
		"phase": "Pending",
		"containerStatuses": []any{
			map[string]any{
				"name": "app",
				"state": map[string]any{
					"waiting": map[string]any{
						"reason":  "CreateContainerConfigError",
						"message": "couldn't find key password in Secret default/db",
					},
				},
			},
		},
	}), nil, nil))
	if found.Findings[0].Cause != "missing Secret db key password" {
		t.Fatal(found.Findings[0].Cause)
	}
}

func TestDiagnosePendingCPU(t *testing.T) {
	found := mustDiagnose(t, DiagnosePod(checkout(map[string]any{
		"phase": "Pending",
		"conditions": []any{
			map[string]any{"type": "PodScheduled", "status": "False", "message": "0/3 nodes are available: 3 Insufficient cpu."},
		},
		"containerStatuses": []any{
			map[string]any{"name": "app", "ready": false, "state": map[string]any{"waiting": map[string]any{"reason": "ContainerCreating"}}},
		},
	}), []any{
		map[string]any{"reason": "FailedScheduling", "message": "0/3 nodes are available: 3 Insufficient cpu.", "involvedObject": map[string]any{"name": "checkout"}},
	}, nil))
	if found.Findings[0].Cause != "Pending: insufficient cpu" {
		t.Fatal(found.Findings[0].Cause)
	}
}

func TestDiagnoseProbe(t *testing.T) {
	events := make([]any, 3)
	for i := range events {
		events[i] = map[string]any{
			"type": "Warning", "reason": "Unhealthy",
			"message":        "Readiness probe failed: HTTP probe failed with statuscode: 500",
			"involvedObject": map[string]any{"name": "checkout"},
		}
	}
	found := mustDiagnose(t, DiagnosePod(checkout(map[string]any{
		"phase": "Running",
		"containerStatuses": []any{
			map[string]any{"name": "app", "ready": false, "state": map[string]any{"running": map[string]any{"startedAt": "t"}}},
		},
	}, map[string]any{
		"containers": []any{
			map[string]any{
				"name":           "app",
				"image":          "app:1",
				"readinessProbe": map[string]any{"httpGet": map[string]any{"path": "/health", "port": float64(8080)}},
			},
		},
	}), events, nil))
	if found.Findings[0].Cause != "readiness probe http://:8080/health failed (3 Unhealthy warnings)" {
		t.Fatal(found.Findings[0].Cause)
	}
}

func TestInspectPodsSkipsGeneralQuestion(t *testing.T) {
	found := InspectPods(InspectPodsInput{
		Question:  "how do I port-forward",
		Namespace: "default",
		FetchJSON: func(args []string) (bool, any) {
			t.Fatal("fetchJSON")
			return false, nil
		},
		FetchText: func(args []string) string {
			t.Fatal("fetchText")
			return ""
		},
	})
	if found != nil {
		t.Fatal(found)
	}
}

func TestInspectPodsFindsEleventhUnhealthy(t *testing.T) {
	calls := 0
	healthy := func(name string) map[string]any {
		return map[string]any{
			"metadata": map[string]any{"name": name, "namespace": "default"},
			"spec":     map[string]any{"containers": []any{map[string]any{"name": "app", "image": "app:1"}}},
			"status": map[string]any{
				"phase": "Running",
				"containerStatuses": []any{
					map[string]any{"name": "app", "ready": true, "state": map[string]any{"running": map[string]any{}}},
				},
			},
		}
	}
	items := make([]any, 0, 11)
	for i := 0; i < 10; i++ {
		items = append(items, healthy("p"+strconv.Itoa(i)))
	}
	items = append(items, map[string]any{
		"metadata": map[string]any{"name": "eleventh", "namespace": "default"},
		"spec": map[string]any{
			"containers": []any{
				map[string]any{"name": "app", "image": "app:1", "resources": map[string]any{"limits": map[string]any{"memory": "64Mi"}}},
			},
		},
		"status": map[string]any{
			"phase": "Running",
			"containerStatuses": []any{
				map[string]any{
					"name": "app", "ready": false,
					"lastState": map[string]any{"terminated": map[string]any{"reason": "OOMKilled", "exitCode": float64(137)}},
				},
			},
		},
	})
	found := InspectPods(InspectPodsInput{
		Question:  "why are pods crashing",
		Namespace: "default",
		FetchJSON: func(args []string) (bool, any) {
			calls++
			if len(args) > 1 && args[1] == "events" {
				return true, map[string]any{"items": []any{}}
			}
			return true, map[string]any{"items": items}
		},
		FetchText: func(args []string) string {
			t.Fatal("fetchText")
			return ""
		},
	})
	if found == nil || found.Pod != "eleventh" || found.Findings[0].Rule != FindingOOM {
		t.Fatalf("expected eleventh OOM pod, got found=%#v calls=%d", found, calls)
	}
}

func TestInspectPodsNamedCrash(t *testing.T) {
	var calls [][]string
	found := InspectPods(InspectPodsInput{
		Question:  "why is pod/checkout crashing",
		Namespace: "payments",
		FetchJSON: func(args []string) (bool, any) {
			calls = append(calls, append([]string{}, args...))
			if len(args) > 1 && args[1] == "pod" {
				return true, checkout(map[string]any{
					"phase": "Running",
					"containerStatuses": []any{
						map[string]any{
							"name":  "app",
							"ready": false,
							"state": map[string]any{"waiting": map[string]any{"reason": "CrashLoopBackOff"}},
							"lastState": map[string]any{
								"terminated": map[string]any{"reason": "Error", "exitCode": float64(1)},
							},
						},
					},
				})
			}
			return true, map[string]any{"items": []any{}}
		},
		FetchText: func(args []string) string {
			calls = append(calls, append([]string{}, args...))
			return "password=secret"
		},
	})
	if found == nil || found.Findings[0].Cause != "container app exited 1; password=***" {
		t.Fatalf("%#v", found)
	}
	if len(calls) < 3 || calls[0][1] != "pod" || calls[1][1] != "events" || calls[2][0] != "logs" {
		t.Fatalf("%#v", calls)
	}
}

func TestHealthyPodNoFindings(t *testing.T) {
	found := DiagnosePod(checkout(map[string]any{
		"phase": "Running",
		"containerStatuses": []any{
			map[string]any{"name": "app", "ready": true, "state": map[string]any{"running": map[string]any{}}},
		},
	}), nil, nil)
	if found != nil {
		t.Fatal(found)
	}
}

func TestTraceServicePortAndNetworkPolicyAndIngress(t *testing.T) {
	svc := map[string]any{
		"metadata": map[string]any{"name": "checkout", "namespace": "default"},
		"spec": map[string]any{
			"selector": map[string]any{"app": "checkout"},
			"ports":    []any{map[string]any{"name": "http", "port": float64(80), "targetPort": float64(8080)}},
		},
	}
	pod := map[string]any{
		"metadata": map[string]any{"name": "checkout-0", "namespace": "default", "labels": map[string]any{"app": "checkout"}},
		"spec": map[string]any{
			"containers": []any{map[string]any{
				"name":  "app",
				"ports": []any{map[string]any{"containerPort": float64(3000)}},
			}},
		},
		"status": map[string]any{
			"phase":      "Running",
			"conditions": []any{map[string]any{"type": "Ready", "status": "True"}},
		},
	}
	ing := map[string]any{
		"metadata": map[string]any{"name": "web-ing", "namespace": "default"},
		"spec": map[string]any{
			"rules": []any{map[string]any{
				"http": map[string]any{"paths": []any{map[string]any{
					"backend": map[string]any{"service": map[string]any{
						"name": "checkout",
						"port": map[string]any{"number": float64(8088)},
					}},
				}}},
			}},
		},
	}
	np := map[string]any{
		"metadata": map[string]any{"name": "deny-all", "namespace": "default"},
		"spec": map[string]any{
			"podSelector": map[string]any{"matchLabels": map[string]any{"app": "checkout"}},
			"policyTypes": []any{"Ingress"},
		},
	}
	diag := traceServiceFromObjects(svc, "default", []dict{pod}, []dict{ing}, []dict{np}, nil, nil)
	if diag == nil || len(diag.Findings) != 3 {
		t.Fatalf("expected 3 findings (port, netpol, ingress), got %#v", diag)
	}
}

func TestTraceRolloutQuotaAndPDB(t *testing.T) {
	dep := map[string]any{
		"metadata": map[string]any{"name": "web", "namespace": "default"},
		"spec":     map[string]any{"selector": map[string]any{"matchLabels": map[string]any{"app": "web"}}},
		"status": map[string]any{
			"conditions": []any{map[string]any{
				"type": "Progressing", "status": "False", "message": "ReplicaSet timed out progressing",
			}},
		},
	}
	quota := map[string]any{
		"metadata": map[string]any{"name": "ns-quota"},
		"status": map[string]any{
			"hard": map[string]any{"pods": "10"},
			"used": map[string]any{"pods": "10"},
		},
	}
	pdb := map[string]any{
		"metadata": map[string]any{"name": "web-pdb"},
		"spec":     map[string]any{"selector": map[string]any{"matchLabels": map[string]any{"app": "web"}}},
		"status":   map[string]any{"disruptionsAllowed": float64(0)},
	}
	diag := traceRolloutFromObjects(dep, "default", nil, nil, []dict{quota}, []dict{pdb}, nil, nil)
	if diag == nil || len(diag.Findings) != 3 {
		t.Fatalf("expected rollout+quota+pdb findings, got %#v", diag)
	}
}

func TestTraceHTTPRoutePortAndDiagnoseNamespace(t *testing.T) {
	svc := map[string]any{
		"metadata": map[string]any{"name": "checkout", "namespace": "default"},
		"spec": map[string]any{
			"selector": map[string]any{"app": "checkout"},
			"ports":    []any{map[string]any{"port": float64(80)}},
		},
	}
	route := map[string]any{
		"metadata": map[string]any{"name": "gw", "namespace": "default"},
		"spec": map[string]any{
			"rules": []any{map[string]any{
				"backendRefs": []any{map[string]any{"kind": "Service", "name": "checkout", "port": float64(9090)}},
			}},
		},
	}
	diag := traceServiceWithRoutes(svc, "default", nil, nil, []dict{route}, nil, nil, nil)
	if diag == nil || len(diag.Findings) < 2 {
		t.Fatalf("expected service + httproute port findings, got %#v", diag)
	}
	hasRoute := false
	for _, f := range diag.Findings {
		if f.Rule == FindingHTTPRoute {
			hasRoute = true
		}
	}
	if !hasRoute {
		t.Fatalf("expected FindingHTTPRoute in %#v", diag.Findings)
	}

	health := DiagnoseNamespace("default", func(args []string) (bool, any) {
		switch args[1] {
		case "pods":
			return true, map[string]any{"items": []any{
				checkout(map[string]any{
					"phase": "Running",
					"containerStatuses": []any{map[string]any{
						"name": "app", "ready": false,
						"lastState": map[string]any{"terminated": map[string]any{"reason": "OOMKilled", "exitCode": float64(137)}},
					}},
				}),
			}}
		case "services":
			return true, map[string]any{"items": []any{svc}}
		case "resourcequotas":
			return true, map[string]any{"items": []any{}}
		case "deployments":
			return true, map[string]any{"items": []any{}}
		default:
			return true, map[string]any{"items": []any{}}
		}
	}, nil)
	if len(health.FailingPods) != 1 || len(health.UnhealthySvcs) != 1 {
		t.Fatalf("unexpected health sweep: %#v", health)
	}
}

func TestAskNamespaceHealthQuestion(t *testing.T) {
	for _, positive := range []string{
		"health",
		"health check",
		"namespace health",
		"check namespace",
		"check ns payments",
		"namespace health in prod",
		"-n payments health",
	} {
		if !isNamespaceHealthQuestion(positive) {
			t.Fatalf("expected %q to match namespace health routing", positive)
		}
	}
	for _, negative := range []string{
		"why are pods crashing",
		"why is pod/api unhealthy",
		"which namespace",
		"how do I port-forward",
		"why is svc/checkout 503",
	} {
		if isNamespaceHealthQuestion(negative) {
			t.Fatalf("did not expect %q to match namespace health routing", negative)
		}
	}

	var seenNS string
	cluster := kube.Cluster{}
	input := InspectPodsInput{
		Namespace: "default",
		FetchJSON: func(args []string) (bool, any) {
			for i := 0; i+1 < len(args); i++ {
				if args[i] == "-n" {
					seenNS = args[i+1]
				}
			}
			switch args[1] {
			case "pods":
				return true, map[string]any{"items": []any{
					checkout(map[string]any{
						"phase": "Running",
						"containerStatuses": []any{map[string]any{
							"name": "app", "ready": false,
							"lastState": map[string]any{"terminated": map[string]any{"reason": "OOMKilled", "exitCode": float64(137)}},
						}},
					}),
				}}
			case "resourcequotas":
				return true, map[string]any{"items": []any{
					map[string]any{
						"metadata": map[string]any{"name": "compute-quota", "namespace": "payments"},
						"status": map[string]any{
							"hard": map[string]any{"pods": "10"},
							"used": map[string]any{"pods": "10"},
						},
					},
				}}
			default:
				return true, map[string]any{"items": []any{}}
			}
		},
	}
	advice := askAdviceWithInput("check namespace payments", cluster, "", input)
	if seenNS != "payments" {
		t.Fatalf("expected namespace payments to be queried, got %q", seenNS)
	}
	if !strings.Contains(advice.Why, "pod/checkout") || !strings.Contains(advice.Why, "OOMKilled") || !strings.Contains(advice.Why, "resourcequota") {
		t.Fatalf("unexpected health advice: %#v", advice)
	}

	healthy := askAdviceWithInput("health", kube.Cluster{}, "", InspectPodsInput{
		Namespace: "staging",
		FetchJSON: func([]string) (bool, any) { return true, map[string]any{"items": []any{}} },
	})
	if !strings.Contains(healthy.Why, "staging looks healthy") {
		t.Fatalf("expected healthy summary for staging, got %#v", healthy)
	}

	unreachable := askAdviceWithInput("health", cluster, "", InspectPodsInput{
		Namespace: "default",
		FetchJSON: func([]string) (bool, any) { return false, nil },
	})
	if strings.Contains(unreachable.Why, "looks healthy") {
		t.Fatalf("must not claim healthy when cluster is unreachable: %#v", unreachable)
	}
}
