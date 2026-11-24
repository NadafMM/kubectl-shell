import assert from "node:assert/strict";
import test from "node:test";
import { initialCluster } from "./fixture.ts";
import { flagValue, positionalWords, quoteArg, scopeOf, tokenize, unquote } from "./parse.ts";

test("tokenize keeps quotes on the token", () => {
  const tokens = tokenize(`kubectl get pods -l "app=web"`);
  assert.deepEqual(
    tokens.map((token) => token.text),
    ["kubectl", "get", "pods", "-l", `"app=web"`],
  );
});

test("positionalWords drops -n and its value", () => {
  const parsed = positionalWords(["get", "-n", "app", "pods"]);
  assert.deepEqual(parsed.words, ["get", "pods"]);
});

test("scopeOf reads -n app on the minikube fixture", () => {
  const scope = scopeOf(["kubectl", "get", "pods", "-n", "app"], initialCluster());
  assert.equal(scope.ns, "app");
  assert.equal(scope.context, "minikube");
  assert.equal(scope.all, false);
});

test("flagValue reads --replicas=0", () => {
  assert.equal(flagValue(["scale", "deploy", "api", "--replicas=0"], ["--replicas"]), "0");
});

test("unquote strips one matching pair", () => {
  assert.equal(unquote(`"app=web"`), "app=web");
  assert.equal(unquote("app=web"), "app=web");
  assert.equal(unquote('"app'), '"app');
});

test("quoteArg roundtrips an argument that contains a space", () => {
  assert.equal(quoteArg("pods"), "pods");
  assert.equal(quoteArg("my file.yaml"), `"my file.yaml"`);
  assert.equal(quoteArg('say "hi"'), `'say "hi"'`);
  assert.equal(quoteArg(`a "b" 'c'`), null);
  const args = ["apply", "-f", "my file.yaml"];
  const line = args.map((arg) => quoteArg(arg)).join(" ");
  assert.deepEqual(
    tokenize(line).map((token) => unquote(token.text)),
    args,
  );
});
