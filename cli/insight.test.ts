import assert from "node:assert/strict";
import test from "node:test";
import { initialCluster } from "../src/lib/kube/fixture.ts";
import { advise, planCommand } from "./insight.ts";

test("delete pod checkout is high risk on minikube/default", () => {
  const plan = planCommand("kubectl delete pod checkout", initialCluster());
  assert.equal(plan.risk, "high");
  assert.equal(plan.verb, "delete");
  assert.deepEqual(plan.targets, ["pod/checkout"]);
  assert.equal(plan.namespace, "default");
  assert.equal(plan.context, "minikube");
  assert.equal(plan.mutating, true);
});

test("drain is high risk", () => {
  const plan = planCommand("kubectl drain node-1", initialCluster());
  assert.equal(plan.risk, "high");
});

test("rollout undo is high risk", () => {
  const plan = planCommand("kubectl rollout undo deployment/api", initialCluster());
  assert.equal(plan.risk, "high");
});

test("rollout restart is high risk", () => {
  const plan = planCommand("kubectl rollout restart deployment api", initialCluster());
  assert.equal(plan.risk, "high");
});

test("scale to zero is high risk", () => {
  const plan = planCommand("kubectl scale deploy api --replicas=0", initialCluster());
  assert.equal(plan.risk, "high");
});

test("get pods is read", () => {
  const plan = planCommand("kubectl get pods", initialCluster());
  assert.equal(plan.risk, "read");
  assert.equal(plan.mutating, false);
});

test("exec debug cp and attach are high risk", () => {
  const cluster = initialCluster();
  for (const line of [
    "kubectl exec pod/api -- date",
    "kubectl debug pod/api",
    "kubectl cp pod:/tmp/a ./a",
    "kubectl attach pod/api",
  ]) {
    const plan = planCommand(line, cluster);
    assert.equal(plan.risk, "high", line);
  }
  assert.equal(
    planCommand("kubectl cp pod:/tmp/a ./a", cluster).summary.includes(
      "Nothing in the cluster changes",
    ),
    false,
  );
  assert.equal(planCommand("kubectl apply -f deploy.yaml", cluster).risk, "high");
  assert.equal(planCommand("kubectl apply --dry-run=client -f deploy.yaml", cluster).risk, "low");
  assert.equal(planCommand("kubectl create -f deploy.yaml", cluster).risk, "high");
  assert.equal(planCommand("kubectl patch deploy api -p '{}'", cluster).risk, "high");
  assert.equal(planCommand("kubectl get secret token", cluster).risk, "read");
});

test("get secret with revealing output warns in plan summary", () => {
  const cluster = initialCluster();
  const yaml = planCommand("kubectl get secret token -o yaml", cluster);
  assert.equal(yaml.risk, "read");
  assert.match(yaml.summary, /may print Secret data/);
  const json = planCommand("kubectl get secret token -o json", cluster);
  assert.match(json.summary, /may print Secret data/);
  assert.doesNotMatch(
    planCommand("kubectl get secret token", cluster).summary,
    /may print Secret data/,
  );
  assert.doesNotMatch(planCommand("kubectl get pods", cluster).summary, /may print Secret data/);
});

test("get configmap with revealing output warns in plan summary", () => {
  const cluster = initialCluster();
  assert.match(
    planCommand("kubectl get configmap app -o yaml", cluster).summary,
    /may print ConfigMap data/,
  );
  assert.doesNotMatch(
    planCommand("kubectl get configmap app", cluster).summary,
    /may print ConfigMap data/,
  );
});

test("port-forward run and expose are high risk", () => {
  const cluster = initialCluster();
  assert.equal(planCommand("kubectl port-forward pod/api 8080:80", cluster).risk, "high");
  assert.match(planCommand("kubectl port-forward pod/api 8080:80", cluster).summary, /local port/i);
  assert.equal(planCommand("kubectl run temp --image=nginx", cluster).risk, "high");
  assert.equal(planCommand("kubectl run temp --image=nginx --dry-run=client", cluster).risk, "low");
  assert.equal(planCommand("kubectl expose deploy api", cluster).risk, "high");
});

test("proxy certificate and set-credentials are high risk", () => {
  const cluster = initialCluster();
  assert.equal(planCommand("kubectl proxy", cluster).risk, "high");
  assert.match(planCommand("kubectl proxy", cluster).summary, /proxy/i);
  assert.equal(planCommand("kubectl certificate approve csr-1", cluster).risk, "high");
  assert.equal(
    planCommand("kubectl config set-credentials user --token=abc", cluster).risk,
    "high",
  );
});

test("describe secret warns in plan summary", () => {
  const cluster = initialCluster();
  assert.match(planCommand("kubectl describe secret token", cluster).summary, /Secret data/i);
});

test("config view --raw warns about credentials", () => {
  const cluster = initialCluster();
  assert.match(planCommand("kubectl config view --raw", cluster).summary, /credentials/i);
});

test("ask names the crashing fixture pod", () => {
  const advice = advise("why is worker-0 crashing", initialCluster());
  assert.match(advice.why, /worker-0 is CrashLoopBackOff in default/);
  assert.deepEqual(advice.commands[0], "describe pod worker-0");
});
