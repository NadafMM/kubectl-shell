import assert from "node:assert/strict";
import test from "node:test";
import { sensitiveStdoutKind } from "./reveal-output.ts";

test("get secret with -o before resource is revealing", () => {
  assert.equal(
    sensitiveStdoutKind(["get", "-n", "app", "-o", "yaml", "secret", "token"]),
    "secret",
  );
});

test("get with jsonpath on secrets is revealing", () => {
  assert.equal(sensitiveStdoutKind(["get", "secret", "token", "-o", "jsonpath={.data}"]), "secret");
});

test("describe configmap is revealing", () => {
  assert.equal(sensitiveStdoutKind(["describe", "configmap", "app"]), "configmap");
});

test("config view --raw is kubeconfig", () => {
  assert.equal(sensitiveStdoutKind(["config", "view", "--raw"]), "kubeconfig");
});

test("plain get pods is not revealing", () => {
  assert.equal(sensitiveStdoutKind(["get", "pods"]), null);
});
