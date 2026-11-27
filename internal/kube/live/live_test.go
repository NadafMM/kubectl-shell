package live

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestMapListItemKeepsNetworkPolicy(t *testing.T) {
	mapped, ok := MapListItem(kItem{Kind: "NetworkPolicy", Metadata: &struct {
		Name              string            `json:"name"`
		Namespace         string            `json:"namespace"`
		CreationTimestamp string            `json:"creationTimestamp"`
		Labels            map[string]string `json:"labels"`
	}{Name: "allow-web", Namespace: "app"}})
	if !ok || mapped.Kind != "networkpolicy" || mapped.Name != "allow-web" {
		t.Fatalf("%#v", mapped)
	}
}

func TestMapNodeReadyFalse(t *testing.T) {
	node, ok := MapNode(kItem{
		Kind: "Node",
		Metadata: &struct {
			Name              string            `json:"name"`
			Namespace         string            `json:"namespace"`
			CreationTimestamp string            `json:"creationTimestamp"`
			Labels            map[string]string `json:"labels"`
		}{Name: "worker-a"},
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
		}{Unschedulable: true},
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
		}{Conditions: []struct {
			Type   string `json:"type"`
			Status string `json:"status"`
		}{{Type: "Ready", Status: "False"}}},
	})
	if !ok || node.Status != "NotReady" || node.Schedulable {
		t.Fatalf("%#v", node)
	}
}

func TestProjectedJSONPathOmitsSecretData(t *testing.T) {
	if strings.Contains(ProjectedJSONPath, ".data") || strings.Contains(ProjectedJSONPath, "stringData") {
		t.Fatal(ProjectedJSONPath)
	}
}

func TestParseProjected(t *testing.T) {
	line := "Pod\tkept\tdefault\t\t\tRunning\t\t\t\t\t\t\t\t\t\t\t\t\tweb\tapp=kept"
	parsed := ParseProjected(line)
	if len(parsed) != 1 || parsed[0].Metadata == nil || parsed[0].Metadata.Name != "kept" {
		t.Fatalf("%#v", parsed)
	}
	if parsed[0].Metadata.Labels["app"] != "kept" {
		t.Fatal(parsed[0].Metadata.Labels)
	}
	if len(parsed[0].Spec.Containers) != 1 || parsed[0].Spec.Containers[0].Name != "web" {
		t.Fatal(parsed[0].Spec)
	}
	if len(ParseProjected(line+"\textra")) != 0 {
		t.Fatal("shifted row should drop")
	}
	tabInMessage := "Pod\tkept\tdefault\t\t\tRunning\t\t\t\t\t\t\tmsg\twith\ta\ttab\t\t\t\t\tapp=kept"
	if len(ParseProjected(tabInMessage)) != 0 {
		t.Fatal("a tab inside a field should drop the row")
	}
}

func TestInvalidatesLiveCache(t *testing.T) {
	if InvalidatesLiveCache([]string{"get", "pods"}) {
		t.Fatal("get should keep the cache")
	}
	if InvalidatesLiveCache([]string{"delete", "pod", "x", "--dry-run=server"}) {
		t.Fatal("dry-run should keep the cache")
	}
	if !InvalidatesLiveCache([]string{"delete", "pod", "x"}) {
		t.Fatal("delete should drop the cache")
	}
	if InvalidatesLiveCache([]string{"rollout", "status", "deploy/web"}) {
		t.Fatal("rollout status is a read")
	}
	if !InvalidatesLiveCache([]string{"rollout", "restart", "deploy/web"}) {
		t.Fatal("rollout restart changes pods")
	}
}

func withFake(t *testing.T, run func(log string)) {
	t.Helper()
	home := t.TempDir()
	log := filepath.Join(home, "kubectl.log")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("KUBE_SHELL_KUBECTL", "node")
	t.Setenv("KUBE_SHELL_NODE_SCRIPT", filepath.Join("..", "..", "..", "scripts", "fake-kubectl.mjs"))
	t.Setenv("KUBE_FAKE_LOG", log)
	t.Setenv("KUBE_FAKE_FIXTURE", "up")
	os.Unsetenv("KUBE_FAKE_FAIL")
	InvalidateLive()
	t.Cleanup(InvalidateLive)
	run(log)
}

func logLines(log string) []string {
	raw, err := os.ReadFile(log)
	if err != nil {
		return nil
	}
	var lines []string
	for _, line := range strings.Split(string(raw), "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func TestColdLiveClusterBoundsKubectl(t *testing.T) {
	withFake(t, func(log string) {
		cluster, ok := LiveCluster("kubectl")
		if !ok || cluster.Current != "minikube" {
			t.Fatalf("%v %#v", ok, cluster)
		}
		n := len(logLines(log))
		if n < 1 || n > 5 {
			t.Fatalf("calls %d\n%s", n, strings.Join(logLines(log), "\n"))
		}
		found := false
		for _, object := range cluster.Objects {
			if object.Name == "web" && object.Kind == "deployment" {
				found = true
			}
		}
		if !found {
			t.Fatalf("objects %#v", cluster.Objects)
		}
	})
}

func TestSecondLiveClusterHitsCache(t *testing.T) {
	withFake(t, func(log string) {
		if _, ok := LiveCluster("kubectl"); !ok {
			t.Fatal("cold")
		}
		first := len(logLines(log))
		if _, ok := LiveCluster("kubectl"); !ok {
			t.Fatal("warm")
		}
		if len(logLines(log)) != first {
			t.Fatalf("cache miss %d -> %d", first, len(logLines(log)))
		}
	})
}

func TestProbeReusesFreshDiskCache(t *testing.T) {
	withFake(t, func(log string) {
		if _, ok := LiveCluster("kubectl"); !ok {
			t.Fatal("cold")
		}
		saved, err := os.ReadFile(cachePath())
		if err != nil {
			t.Fatal(err)
		}
		InvalidateLive()
		if err := os.MkdirAll(filepath.Dir(cachePath()), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(cachePath(), saved, 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("KUBE_FAKE_FIXTURE", "down")
		before := len(logLines(log))
		if !ProbeLive() {
			t.Fatal("fresh disk cache should answer the probe")
		}
		if len(logLines(log)) != before {
			t.Fatal("probe called kubectl")
		}
	})
}

func TestFailedCombinedGetKeepsPartial(t *testing.T) {
	withFake(t, func(log string) {
		t.Setenv("KUBE_FAKE_FAIL", "namespaced")
		InvalidateLive()
		cluster, ok := LiveCluster("kubectl")
		if !ok {
			t.Fatal("live")
		}
		found := false
		for _, object := range cluster.Objects {
			if object.Name == "kept" && object.Kind == "pod" {
				found = true
			}
		}
		if !found {
			t.Fatalf("objects %#v", cluster.Objects)
		}
		if len(logLines(log)) <= 4 {
			t.Fatalf("calls %d", len(logLines(log)))
		}
	})
}

func TestProbeIgnoresStaleDiskCache(t *testing.T) {
	withFake(t, func(log string) {
		t.Setenv("KUBE_FAKE_FIXTURE", "down")
		InvalidateLive()
		if err := os.MkdirAll(filepath.Dir(cachePath()), 0o700); err != nil {
			t.Fatal(err)
		}
		stale := time.Now().Add(-time.Minute).UnixMilli()
		body := `{"at":` + strconv.FormatInt(stale, 10) + `,"key":"minikube|minikube|default","cluster":{"current":"minikube","objects":[]}}`
		if err := os.WriteFile(cachePath(), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if ProbeLive() {
			t.Fatal("a stale disk cache should not count as live")
		}
	})
}

func TestProbeIgnoresCacheWithoutObjects(t *testing.T) {
	withFake(t, func(log string) {
		t.Setenv("KUBE_FAKE_FIXTURE", "down")
		InvalidateLive()
		if err := os.MkdirAll(filepath.Dir(cachePath()), 0o700); err != nil {
			t.Fatal(err)
		}
		body := `{"at":` + strconvNow() + `,"key":"minikube|minikube|default","cluster":{"current":"minikube"}}`
		if err := os.WriteFile(cachePath(), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if ProbeLive() {
			t.Fatal("cache without objects is not a live cluster")
		}
	})
}

func strconvNow() string {
	return strconv.FormatInt(time.Now().UnixMilli(), 10)
}

func TestRunLiveKeepsCacheOnGet(t *testing.T) {
	withFake(t, func(log string) {
		if _, ok := LiveCluster("kubectl"); !ok {
			t.Fatal("cold")
		}
		if _, err := os.Stat(cachePath()); err != nil {
			t.Fatal(err)
		}
		code := RunLive([]string{"get", "pods"}, RunLiveOptions{})
		if code != 0 {
			t.Fatal(code)
		}
		if _, err := os.Stat(cachePath()); err != nil {
			t.Fatal("get should keep the cache")
		}
	})
}

func TestRunLiveClearsCacheOnDelete(t *testing.T) {
	withFake(t, func(log string) {
		if _, ok := LiveCluster("kubectl"); !ok {
			t.Fatal("cold")
		}
		code := RunLive([]string{"delete", "pod", "x"}, RunLiveOptions{})
		if code != 0 {
			t.Fatal(code)
		}
		if _, err := os.Stat(cachePath()); !os.IsNotExist(err) {
			t.Fatal("delete should clear the cache")
		}
	})
}

func TestRunLiveRedactsPipedSecretJSON(t *testing.T) {
	withFake(t, func(log string) {
		t.Setenv("KUBE_SHELL_REDACT_STDOUT", "1")
		// Temp file is not a char device, so redaction kicks in like a pipe.
		outFile, err := os.CreateTemp(t.TempDir(), "out")
		if err != nil {
			t.Fatal(err)
		}
		code := RunLive([]string{"get", "secret", "token", "-o", "json"}, RunLiveOptions{
			Stdout: outFile,
			Stderr: os.Stderr,
		})
		name := outFile.Name()
		_ = outFile.Close()
		if code != 0 {
			t.Fatal(code)
		}
		raw, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		text := string(raw)
		if !strings.Contains(text, "***") || strings.Contains(text, "cGFzcw==") {
			t.Fatalf("stdout=%q", text)
		}
	})
}

func TestContextSnapshotAvoidsObjectAPIs(t *testing.T) {
	withFake(t, func(log string) {
		cluster, ok := ContextSnapshot("kubectl get pods")
		if !ok || cluster.Current != "minikube" {
			t.Fatalf("ok=%v cluster=%+v", ok, cluster)
		}
		for _, line := range logLines(log) {
			if strings.Contains(line, `"get"`) {
				t.Fatalf("ContextSnapshot should not call kubectl get: %s", line)
			}
		}
	})
}
