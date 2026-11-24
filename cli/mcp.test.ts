import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import test from "node:test";

const root = process.cwd();
const kube = path.join(root, "cli", "kube.cjs");
const fake = path.join(root, "scripts", "fake-kubectl.mjs");

function frame(message: unknown): Buffer {
  const body = Buffer.from(JSON.stringify(message));
  return Buffer.concat([Buffer.from(`Content-Length: ${body.length}\r\n\r\n`), body]);
}

function frames(buf: Buffer): { result?: { content?: { text?: string }[]; isError?: boolean } }[] {
  const out = [];
  let offset = 0;
  while (offset + 4 < buf.length) {
    const headerEnd = buf.indexOf("\r\n\r\n", offset);
    if (headerEnd < 0) break;
    const header = buf.subarray(offset, headerEnd).toString("utf8");
    const match = /Content-Length:\s*(\d+)/i.exec(header);
    if (!match) break;
    const length = Number(match[1]);
    const start = headerEnd + 4;
    out.push(JSON.parse(buf.subarray(start, start + length).toString("utf8")));
    offset = start + length;
  }
  return out;
}

test("mcp executes allowed reads, redacts secrets, and requires a plan id to delete", () => {
  const home = fs.mkdtempSync(path.join(os.tmpdir(), "kube-mcp-"));
  fs.mkdirSync(path.join(home, ".kube-shell"), { recursive: true });
  fs.writeFileSync(
    path.join(home, ".kube-shell", "policy.json"),
    JSON.stringify({
      agents: { allow: ["get", "describe", "logs", "plan"], write_requires: "approved-plan" },
    }),
  );
  const calls = [
    {
      jsonrpc: "2.0",
      id: 1,
      method: "tools/call",
      params: { name: "kube_execute", arguments: { command: "get pods" } },
    },
    {
      jsonrpc: "2.0",
      id: 2,
      method: "tools/call",
      params: { name: "kube_execute", arguments: { command: "get secret token -o json" } },
    },
    {
      jsonrpc: "2.0",
      id: 3,
      method: "tools/call",
      params: { name: "kube_execute", arguments: { command: "delete pod worker-0" } },
    },
    {
      jsonrpc: "2.0",
      id: 4,
      method: "tools/call",
      params: { name: "kube_plan", arguments: { command: "delete pod worker-0" } },
    },
  ];
  const first = spawnSync(process.execPath, [kube, "mcp"], {
    cwd: root,
    input: Buffer.concat(calls.map(frame)),
    env: {
      ...process.env,
      HOME: home,
      USERPROFILE: home,
      KUBE_SHELL_KUBECTL: process.execPath,
      KUBE_SHELL_NODE_SCRIPT: fake,
      KUBE_FAKE_FIXTURE: "up",
      KUBE_SHELL_POLICY: path.join(home, "ignored.json"),
    },
  });
  assert.equal(first.status, 0, first.stderr.toString());
  const replies = frames(first.stdout);
  assert.equal(replies[0]?.result?.isError, false);
  const secret = replies[1]?.result?.content?.[0]?.text ?? "";
  assert.match(secret, /\*\*\*/);
  assert.doesNotMatch(secret, /cGFzcw==/);
  assert.equal(replies[2]?.result?.isError, true);
  const planned = JSON.parse(replies[3]?.result?.content?.[0]?.text ?? "{}") as { planId?: string };
  assert.ok(planned.planId);
  const second = spawnSync(process.execPath, [kube, "mcp"], {
    cwd: root,
    input: frame({
      jsonrpc: "2.0",
      id: 5,
      method: "tools/call",
      params: {
        name: "kube_execute",
        arguments: { command: "delete pod worker-0", planId: planned.planId },
      },
    }),
    env: {
      ...process.env,
      HOME: home,
      USERPROFILE: home,
      KUBE_SHELL_KUBECTL: process.execPath,
      KUBE_SHELL_NODE_SCRIPT: fake,
      KUBE_FAKE_FIXTURE: "up",
    },
  });
  const executed = frames(second.stdout);
  assert.equal(executed[0]?.result?.isError, false);
  const audit = fs.readFileSync(path.join(home, ".kube-shell", "audit.jsonl"), "utf8");
  assert.match(audit, /"actor":"mcp"/);
});
