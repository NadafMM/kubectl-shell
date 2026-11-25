package plan

import (
	"github.com/NadafMM/kubectl-shell/internal/kube"
	"strings"
	"testing"
)

func TestPlanDeletePod(t *testing.T) {
	plan := PlanCommand("kubectl delete pod checkout", kube.InitialCluster())
	if plan.Risk != "high" || plan.Verb != "delete" || plan.Namespace != "default" || plan.Context != "minikube" || !plan.Mutating {
		t.Fatalf("%#v", plan)
	}
	if len(plan.Targets) != 1 || plan.Targets[0] != "pod/checkout" {
		t.Fatal(plan.Targets)
	}
}

func TestPlanRisks(t *testing.T) {
	cluster := kube.InitialCluster()
	high := []string{
		"kubectl drain node-1",
		"kubectl rollout undo deployment/api",
		"kubectl rollout restart deployment api",
		"kubectl scale deploy api --replicas=0",
		"kubectl scale deploy api --replicas=3",
		"kubectl set image deployment/api nginx=nginx:1.2",
		"kubectl label pod api env=prod",
		"kubectl annotate pod api note=x",
		"kubectl edit deploy api",
		"kubectl autoscale deploy api --max=5",
		"kubectl uncordon node-1",
		"kubectl exec pod/api -- date",
		"kubectl debug pod/api",
		"kubectl cp pod:/tmp/a ./a",
		"kubectl attach pod/api",
		"kubectl apply -f deploy.yaml",
		"kubectl create -f deploy.yaml",
		"kubectl patch deploy api -p '{}'",
		"kubectl port-forward pod/api 8080:80",
		"kubectl run temp --image=nginx",
		"kubectl expose deploy api",
		"kubectl proxy",
		"kubectl certificate approve csr-1",
		"kubectl config set-credentials user --token=abc",
	}
	for _, line := range high {
		if plan := PlanCommand(line, cluster); plan.Risk != "high" {
			t.Fatalf("%s risk %s", line, plan.Risk)
		}
	}
	if strings.Contains(PlanCommand("kubectl cp pod:/tmp/a ./a", cluster).Summary, "Nothing in the cluster changes") {
		t.Fatal("cp summary")
	}
	if PlanCommand("kubectl apply --dry-run=client -f deploy.yaml", cluster).Risk != "low" {
		t.Fatal("dry-run apply")
	}
	if PlanCommand("kubectl run temp --image=nginx --dry-run=client", cluster).Risk != "low" {
		t.Fatal("dry-run run")
	}
	if PlanCommand("kubectl label pod api env=prod --dry-run=client", cluster).Risk != "low" {
		t.Fatal("dry-run label")
	}
	read := PlanCommand("kubectl get pods", cluster)
	if read.Risk != "read" || read.Mutating {
		t.Fatal(read)
	}
	if PlanCommand("kubectl get secret token", cluster).Risk != "read" {
		t.Fatal("get secret")
	}
	if PlanCommand("kubectl my-plugin foo", cluster).Risk != "read" {
		t.Fatal("unknown verb should stay read")
	}
}

func TestPlanRevealingSummaries(t *testing.T) {
	cluster := kube.InitialCluster()
	if !strings.Contains(PlanCommand("kubectl get secret token -o yaml", cluster).Summary, "may print Secret data") {
		t.Fatal("yaml secret")
	}
	if !strings.Contains(PlanCommand("kubectl get secret token -o json", cluster).Summary, "may print Secret data") {
		t.Fatal("json secret")
	}
	if strings.Contains(PlanCommand("kubectl get secret token", cluster).Summary, "may print Secret data") {
		t.Fatal("plain secret")
	}
	if strings.Contains(PlanCommand("kubectl get pods", cluster).Summary, "may print Secret data") {
		t.Fatal("pods")
	}
	if !strings.Contains(PlanCommand("kubectl get configmap app -o yaml", cluster).Summary, "may print ConfigMap data") {
		t.Fatal("configmap yaml")
	}
	if strings.Contains(PlanCommand("kubectl get configmap app", cluster).Summary, "may print ConfigMap data") {
		t.Fatal("plain configmap")
	}
	if !strings.Contains(strings.ToLower(PlanCommand("kubectl describe secret token", cluster).Summary), "secret data") {
		t.Fatal("describe secret")
	}
	if !strings.Contains(strings.ToLower(PlanCommand("kubectl config view --raw", cluster).Summary), "credentials") {
		t.Fatal("config view")
	}
	if !strings.Contains(strings.ToLower(PlanCommand("kubectl port-forward pod/api 8080:80", cluster).Summary), "local port") {
		t.Fatal("port-forward")
	}
	if !strings.Contains(strings.ToLower(PlanCommand("kubectl proxy", cluster).Summary), "proxy") {
		t.Fatal("proxy")
	}
}

func TestAdviseNamesCrashingPod(t *testing.T) {
	advice := Advise("why is worker-0 crashing", kube.InitialCluster())
	if !strings.Contains(advice.Why, "worker-0 is CrashLoopBackOff in default") {
		t.Fatal(advice.Why)
	}
	if len(advice.Commands) == 0 || advice.Commands[0] != "describe pod worker-0" {
		t.Fatal(advice.Commands)
	}
}
