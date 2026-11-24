import fs from "node:fs";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const covDir = path.join(root, "coverage");
const logPath = path.join(covDir, "run.log");

fs.mkdirSync(covDir, { recursive: true });

const run = spawnSync(process.execPath, [path.join(root, "scripts", "test-kube.mjs")], {
  cwd: root,
  env: { ...process.env, KUBE_TEST_COVERAGE: "1" },
  encoding: "utf8",
});

const combined = `${run.stdout ?? ""}${run.stderr ?? ""}`;
fs.writeFileSync(logPath, combined, "utf8");
process.stdout.write(combined);

const check = spawnSync(
  process.execPath,
  [path.join(root, "scripts", "coverage-check.mjs"), logPath],
  {
    cwd: root,
    stdio: "inherit",
  },
);

process.exit(run.status !== 0 ? (run.status ?? 1) : (check.status ?? 1));
