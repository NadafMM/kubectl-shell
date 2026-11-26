package kube

import "encoding/json"

func pod(ns, name, status, age string, containers []string, labels map[string]string) KObject {
	item := KObject{Kind: "pod", NS: ns, Name: name, Status: status, Age: age, Labels: labels}
	for _, container := range containers {
		item.Containers = append(item.Containers, Container{Name: container})
	}
	return item
}

func obj(kind, name, ns, age, status, svc string, labels map[string]string) KObject {
	return KObject{Kind: kind, Name: name, NS: ns, Age: age, Status: status, SvcType: svc, Labels: labels}
}

// template is the practice cluster the unit tests complete against.
// The Go CLI does not serve it; live names arrive with the kubectl runner.
var template = Cluster{
	Current: "minikube",
	Contexts: []Context{
		{Name: "minikube", Cluster: "minikube", User: "minikube", Namespace: "default"},
		{Name: "staging", Cluster: "staging-eu", User: "staging", Namespace: "app"},
		{Name: "prod-east", Cluster: "prod-east-1", User: "prod", Namespace: "app"},
	},
	Namespaces: []Namespace{
		{Name: "default", Status: "Active", Age: "90d"},
		{Name: "kube-system", Status: "Active", Age: "90d"},
		{Name: "app", Status: "Active", Age: "40d"},
		{Name: "monitoring", Status: "Active", Age: "21d"},
	},
	Nodes: []Node{
		{Name: "control-plane", Status: "Ready", Roles: "control-plane", Age: "90d", Version: "v1.31.4", IP: "192.168.49.2", Schedulable: true},
		{Name: "node-1", Status: "Ready", Roles: "worker", Age: "90d", Version: "v1.31.4", IP: "192.168.49.3", Schedulable: true},
		{Name: "node-2", Status: "Ready", Roles: "worker", Age: "88d", Version: "v1.31.4", IP: "192.168.49.4", Schedulable: true},
	},
	PVs: []PV{
		{Name: "pvc-9f2a", Capacity: "10Gi", Mode: "RWO", Status: "Bound", Claim: "monitoring/prometheus-db", Age: "21d"},
	},
	Events: []Event{
		{NS: "default", Last: "2m", Type: "Warning", Reason: "BackOff", Object: "pod/worker-0", Message: "Back-off restarting failed container worker"},
	},
	Objects: []KObject{
		pod("default", "api-7d9f8b-xk2l", "Running", "4d", []string{"api"}, map[string]string{"app": "api"}),
		pod("default", "web-6c4d9a-mq1p", "Running", "2d", []string{"web"}, map[string]string{"app": "web"}),
		pod("default", "worker-0", "CrashLoopBackOff", "6h", []string{"worker"}, map[string]string{"app": "worker"}),
		pod("app", "checkout-9a1c-222", "Pending", "6m", []string{"checkout"}, map[string]string{"app": "checkout"}),
		obj("deployment", "api", "default", "40d", "", "", map[string]string{"app": "api"}),
		obj("deployment", "web", "default", "30d", "", "", map[string]string{"app": "web"}),
		obj("service", "api", "default", "40d", "", "ClusterIP", map[string]string{"app": "api"}),
		obj("configmap", "api-config", "default", "20d", "", "", map[string]string{"app": "api"}),
		obj("secret", "api-tls", "default", "20d", "", "", map[string]string{"app": "api"}),
		obj("ingress", "web", "default", "18d", "", "", map[string]string{"app": "web"}),
		obj("job", "migrate", "default", "1d", "Complete", "", map[string]string{"job": "migrate"}),
	},
}

func InitialCluster() Cluster {
	raw, err := json.Marshal(template)
	if err != nil {
		panic(err)
	}
	var out Cluster
	if err := json.Unmarshal(raw, &out); err != nil {
		panic(err)
	}
	return out
}
