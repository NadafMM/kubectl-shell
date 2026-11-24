import assert from "node:assert/strict";
import test from "node:test";
import { redactLine } from "./redact.ts";

test("redactLine leaves an ordinary command unchanged", () => {
  assert.equal(redactLine("get pods"), "get pods");
});

test("redactLine hides literal and token values", () => {
  assert.equal(
    redactLine("create secret generic db --from-literal=password=hunter2"),
    "create secret generic db --from-literal=password=***",
  );
  assert.equal(
    redactLine("create secret generic db --from-literal password=hunter2"),
    "create secret generic db --from-literal password=***",
  );
  assert.equal(
    redactLine("create secret generic db --from-file=key=./secret.txt"),
    "create secret generic db --from-file=key=***",
  );
  assert.equal(redactLine("create token --token abc"), "create token --token ***");
  assert.equal(redactLine("create token --token=abc"), "create token --token=***");
  assert.equal(redactLine("login --password=secret"), "login --password=***");
  assert.equal(redactLine("login --docker-password=secret"), "login --docker-password=***");
});

test("redactLine hides set env assignment", () => {
  assert.equal(redactLine("set env deploy/api TOKEN=sekret"), "set env deploy/api TOKEN=***");
});

test("redactLine hides client-secret flag", () => {
  assert.equal(
    redactLine("create secret docker-registry reg --client-secret=abc"),
    "create secret docker-registry reg --client-secret=***",
  );
});
