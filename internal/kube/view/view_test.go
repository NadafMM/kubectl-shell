package view

import (
	"bytes"
	"github.com/NadafMM/kubectl-shell/internal/kube"
	"github.com/NadafMM/kubectl-shell/internal/kube/plan"
	"strings"
	"testing"
)

func testViewCluster() kube.Cluster {
	return kube.Cluster{
		Current: "minikube",
		Contexts: []kube.Context{
			{Name: "minikube", Cluster: "minikube", User: "minikube", Namespace: "default"},
		},
		Namespaces: []kube.Namespace{
			{Name: "default", Status: "Active", Age: "1d"},
			{Name: "app", Status: "Active", Age: "1d"},
		},
		Nodes: []kube.Node{
			{Name: "node-1", Status: "Ready", Roles: "worker", Age: "1d", Version: "v1.31.1", Schedulable: true},
		},
		Objects: []kube.KObject{
			{Kind: "pod", Name: "api", NS: "default", Age: "1d", Labels: map[string]string{}, Status: "Running"},
			{Kind: "pod", Name: "worker-0", NS: "default", Age: "6h", Labels: map[string]string{}, Status: "CrashLoopBackOff"},
			{Kind: "pod", Name: "checkout", NS: "app", Age: "2h", Labels: map[string]string{}, Status: "Pending"},
			{Kind: "deployment", Name: "web", NS: "default", Age: "2d", Labels: map[string]string{}, Status: "1/1"},
		},
	}
}

func TestProblemPodsSortAboveRunning(t *testing.T) {
	rows := RowsOf(testViewCluster(), BoardPod, "default", false, "")
	if len(rows) < 2 || rows[0].Name != "worker-0" || rows[1].Name != "api" {
		t.Fatalf("%v", rows)
	}
}

func TestAllNamespacesTable(t *testing.T) {
	text := TableText(testViewCluster(), BoardPod, true)
	if !strings.Contains(text, "app  checkout  Pending") {
		t.Fatal(text)
	}
}

func TestDeployBoardListsWeb(t *testing.T) {
	text := TableText(testViewCluster(), BoardDeployment, false)
	if !strings.Contains(text, "web") {
		t.Fatal(text)
	}
}

func TestDeletePlanForPodIsHighRisk(t *testing.T) {
	plan := plan.PlanCommand(DeleteEngineLine(Row{Kind: "pod", Name: "worker-0", NS: "default"}), testViewCluster())
	if plan.Risk != "high" || !strings.Contains(plan.Command, "delete pod worker-0") {
		t.Fatalf("%+v", plan)
	}
}

func TestReduceDeleteConfirm(t *testing.T) {
	rows := RowsOf(testViewCluster(), BoardPod, "default", false, "")
	armed, effect := Reduce(NewViewState(BoardPod, false, "default", false), "d", rows, []string{"default"})
	if effect.Type != "arm-delete" || armed.Mode != ViewModeConfirm {
		t.Fatalf("%+v %+v", armed, effect)
	}
	confirmed, effect := Reduce(armed, "y", rows, []string{"default"})
	if effect.Type != "delete" {
		t.Fatalf("%+v %+v", confirmed, effect)
	}
	cancelled, effect := Reduce(armed, "n", rows, []string{"default"})
	if effect.Type != "none" || cancelled.Message != "not deleted" {
		t.Fatalf("%+v %+v", cancelled, effect)
	}
}

func TestReduceQuitLeavesSubmode(t *testing.T) {
	help := NewViewState(BoardPod, false, "default", false)
	help.Mode = ViewModeHelp
	next, effect := Reduce(help, "q", nil, nil)
	if next.Mode != ViewModeList || effect.Type != "none" {
		t.Fatalf("%+v %+v", next, effect)
	}
	_, effect = Reduce(next, "q", nil, nil)
	if effect.Type != "quit" {
		t.Fatalf("%+v", effect)
	}
}

func TestFilterKeepsMatchingName(t *testing.T) {
	rows := RowsOf(testViewCluster(), BoardPod, "default", true, "check")
	if len(rows) != 1 || rows[0].Name != "checkout" {
		t.Fatalf("%v", rows)
	}
}

func TestBoardFromAliases(t *testing.T) {
	if BoardFrom("deploy") != BoardDeployment || BoardFrom("") != BoardPod {
		t.Fatal(BoardFrom("deploy"), BoardFrom(""))
	}
}

func TestOpenViewPlainTable(t *testing.T) {
	var out bytes.Buffer
	cluster := OpenView(testViewCluster(), ViewOptions{
		Board:      BoardPod,
		All:        false,
		ForcePlain: true,
		Stdout:     &out,
	})
	if cluster.Current != "minikube" {
		t.Fatal(cluster.Current)
	}
	if !strings.Contains(out.String(), "worker-0  CrashLoopBackOff") {
		t.Fatal(out.String())
	}
}

func TestPaintHonorsNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if paint("x", "bad") != "x" || paint("y", "selected") != "y" {
		t.Fatal(paint("x", "bad"), paint("y", "selected"))
	}
}
