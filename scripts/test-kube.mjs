import * as esbuild from "esbuild";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");

function collect(dir) {
  if (!fs.existsSync(dir)) return [];
  const found = [];
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) found.push(...collect(full));
    else if (entry.name.endsWith(".test.ts")) found.push(full);
  }
  return found;
}

const files = [
  ...collect(path.join(root, "src", "lib", "kube")),
  ...collect(path.join(root, "cli")),
];
if (files.length === 0) process.exit(0);

const outDir = fs.mkdtempSync(path.join(os.tmpdir(), "kube-tests-"));
let status;
try {
  await esbuild.build({
    entryPoints: files,
    outdir: outDir,
    outbase: root,
    bundle: true,
    platform: "node",
    format: "cjs",
    outExtension: { ".js": ".cjs" },
    packages: "external",
    logLevel: "warning",
    absWorkingDir: root,
  });

  function bundled(dir) {
    const found = [];
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
      const full = path.join(dir, entry.name);
      if (entry.isDirectory()) found.push(...bundled(full));
      else if (entry.name.endsWith(".cjs")) found.push(full);
    }
    return found;
  }

  const nodeArgs = ["--test"];
  if (process.env.KUBE_TEST_COVERAGE === "1") {
    nodeArgs.unshift("--experimental-test-coverage");
  }
  const tests = spawnSync(process.execPath, [...nodeArgs, ...bundled(outDir)], {
    cwd: root,
    stdio: "inherit",
    env: {
      ...process.env,
      NODE_V8_COVERAGE:
        process.env.KUBE_TEST_COVERAGE === "1"
          ? path.join(root, "coverage")
          : process.env.NODE_V8_COVERAGE,
    },
  });
  status = tests.status ?? 1;
} finally {
  try {
    fs.rmSync(outDir, { recursive: true, force: true });
  } catch {
    /* ignore */
  }
}
process.exit(status ?? 1);
