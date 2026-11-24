import { spawnSync } from "node:child_process";
import crypto from "node:crypto";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const pkg = JSON.parse(fs.readFileSync(path.join(root, "package.json"), "utf8"));
const version = pkg.version;

const refName = process.env.GITHUB_REF_NAME;
if (refName !== undefined && refName !== "" && refName !== `v${version}`) {
  console.error(`tag ${refName} does not match package.json version ${version}`);
  process.exit(1);
}

const bundleName = `kubectl-shell-${version}`;
const distDir = path.join(root, "dist");
const tarball = path.join(distDir, `${bundleName}.tar.gz`);

const stageRoot = fs.mkdtempSync(path.join(os.tmpdir(), "kubectl-shell-release-"));
const bundleDir = path.join(stageRoot, bundleName);
fs.mkdirSync(bundleDir, { recursive: true });

const staged = [
  { src: path.join(root, "cli", "kube.cjs"), rel: "cli/kube.cjs" },
  { src: path.join(root, "LICENSE"), rel: "LICENSE" },
  { src: path.join(root, "README.md"), rel: "README.md" },
];

for (const { src, rel } of staged) {
  const dest = path.join(bundleDir, rel);
  fs.mkdirSync(path.dirname(dest), { recursive: true });
  fs.copyFileSync(src, dest);
}

fs.mkdirSync(distDir, { recursive: true });
const tar = spawnSync("tar", ["-czf", tarball, "-C", stageRoot, bundleName], {
  stdio: "inherit",
});
fs.rmSync(stageRoot, { recursive: true, force: true });

if (tar.status !== 0) {
  process.exit(tar.status ?? 1);
}

const sha256 = crypto.createHash("sha256").update(fs.readFileSync(tarball)).digest("hex");

const absPath = path.resolve(tarball);
console.log(`path=${absPath}`);
console.log(`sha256=${sha256}`);
