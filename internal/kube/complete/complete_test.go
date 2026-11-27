package complete

import (
	"github.com/NadafMM/kubectl-shell/internal/kube"
	"github.com/NadafMM/kubectl-shell/internal/kube/parse"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func hasValue(items []kube.Candidate, value string) bool {
	for _, item := range items {
		if item.Value == value {
			return true
		}
	}
	return false
}

func TestEmptyLineOffersView(t *testing.T) {
	found := Complete("", 0, kube.InitialCluster())
	if !hasValue(found.Candidates, "view") {
		t.Fatal(found.Candidates)
	}
}

func TestViewCompletesDeploy(t *testing.T) {
	line := "view "
	found := Complete(line, len(line), kube.InitialCluster())
	if !hasValue(found.Candidates, "deploy") {
		t.Fatal(found.Candidates)
	}
}

func TestGetPodsCompletesWorker(t *testing.T) {
	line := "kubectl get pods "
	found := Complete(line, len(line), kube.InitialCluster())
	if !hasValue(found.Candidates, "worker-0") {
		t.Fatal(found.Candidates)
	}
}

func TestImageCompletionDoesNotInvent(t *testing.T) {
	line := "kubectl run web --image="
	found := Complete(line, len(line), kube.InitialCluster())
	for _, item := range found.Candidates {
		if strings.Contains(item.Value, "nginx") {
			t.Fatal(item)
		}
	}
}

func TestLabelCompletion(t *testing.T) {
	line := "kubectl get pods -l "
	found := Complete(line, len(line), kube.InitialCluster())
	if !hasValue(found.Candidates, "app=worker") {
		t.Fatal(found.Candidates)
	}
}

func TestResolveResourceKinds(t *testing.T) {
	checks := map[string]string{
		"pods": "pod", "endpoints": "endpoints", "poddisruptionbudgets": "poddisruptionbudget",
		"networkpolicies": "networkpolicy", "resourcequotas": "resourcequota", "limitranges": "limitrange",
		"storageclasses": "storageclass", "priorityclasses": "priorityclass",
		"customresourcedefinitions": "customresourcedefinition",
	}
	for name, kind := range checks {
		got, ok := parse.ResolveResource(name)
		if !ok || got.Kind != kind {
			t.Fatalf("%s -> %#v", name, got)
		}
	}
}

func TestCatalogKindsDoNotBorrowPodNames(t *testing.T) {
	for _, name := range []string{
		"networkpolicies", "poddisruptionbudgets", "resourcequotas", "limitranges",
		"priorityclasses", "customresourcedefinitions", "endpoints", "storageclasses",
	} {
		line := "kubectl get " + name + " "
		found := Complete(line, len(line), kube.InitialCluster())
		if hasValue(found.Candidates, "worker-0") {
			t.Fatal(name)
		}
		for _, item := range found.Candidates {
			if item.Kind != "flag" {
				t.Fatalf("%s produced %s %s", name, item.Kind, item.Value)
			}
		}
	}
}

func TestNetworkPolicyCompletesItsOwnName(t *testing.T) {
	cluster := kube.InitialCluster()
	cluster.Objects = append(cluster.Objects, kube.KObject{
		Kind: "networkpolicy", Name: "allow-web", NS: "default", Age: "1d", Labels: map[string]string{},
	})
	line := "kubectl get networkpolicies "
	found := Complete(line, len(line), cluster)
	if !hasValue(found.Candidates, "allow-web") || hasValue(found.Candidates, "worker-0") {
		t.Fatal(found.Candidates)
	}
}

func TestStorageClassOutsideNamespace(t *testing.T) {
	cluster := kube.InitialCluster()
	cluster.Objects = append(cluster.Objects, kube.KObject{
		Kind: "storageclass", Name: "standard", NS: "", Age: "1d", Labels: map[string]string{},
	})
	line := "kubectl get storageclasses "
	found := Complete(line, len(line), cluster)
	if !hasValue(found.Candidates, "standard") {
		t.Fatal(found.Candidates)
	}
}

func TestManifestCompletion(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "deploy.yaml"), []byte("kind: Pod\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "k8s"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "k8s", "web.yaml"), []byte("kind: Service\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	top := parse.ManifestCandidates("", root)
	if !hasValue(top, "deploy.yaml") || !hasValue(top, "k8s/") {
		t.Fatal(top)
	}
	nested := parse.ManifestCandidates("k8s/w", root)
	if len(nested) != 1 || nested[0].Value != "k8s/web.yaml" {
		t.Fatal(nested)
	}
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })
	line := "kubectl apply -f "
	found := Complete(line, len(line), kube.InitialCluster())
	if !hasValue(found.Candidates, "deploy.yaml") || hasValue(found.Candidates, "./deploy.yaml") {
		t.Fatal(found.Candidates)
	}
}

func TestSymlinkManifestIsSkipped(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "real.yaml")
	if err := os.WriteFile(target, []byte("kind: Pod\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, "deploy.yaml")); err != nil {
		t.Fatal(err)
	}
	if hasValue(parse.ManifestCandidates("", root), "deploy.yaml") {
		t.Fatal("symlink should not be a candidate")
	}
}

func TestCompletionValueDropsMetacharacters(t *testing.T) {
	if got, ok := parse.CompletionValue("deploy.yaml"); !ok || got != "deploy.yaml" {
		t.Fatal(got, ok)
	}
	for _, value := range []string{"foo$(id).yaml", "foo`id`.yaml", "foo;id.yaml", "foo|id.yaml", "foo\nid.yaml"} {
		if _, ok := parse.CompletionValue(value); ok {
			t.Fatal(value)
		}
	}
}

func TestCompleteIncludesDynamicCRDs(t *testing.T) {
	cluster := kube.InitialCluster()
	cluster.DynamicResources = []kube.Resource{
		{Name: "applications", Singular: "application", Kind: "application", Detail: "argoproj.io/v1alpha1", Weight: 14, Namespaced: true, Short: []string{"app"}},
	}
	line := "kubectl get app"
	found := Complete(line, len(line), cluster)
	if !hasValue(found.Candidates, "applications") {
		t.Fatalf("expected CRD applications in %+v", found.Candidates)
	}
}
