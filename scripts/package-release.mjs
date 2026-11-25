import { spawnSync } from "node:child_process";
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

const targets = [
  { goos: "linux", goarch: "amd64" },
  { goos: "linux", goarch: "arm64" },
  { goos: "darwin", goarch: "amd64" },
  { goos: "darwin", goarch: "arm64" },
  { goos: "windows", goarch: "amd64" },
];

const distDir = path.join(root, "dist");
fs.mkdirSync(distDir, { recursive: true });

for (const { goos, goarch } of targets) {
  const exe = goos === "windows" ? "kube.exe" : "kube";
  const stageRoot = fs.mkdtempSync(path.join(os.tmpdir(), "kubectl-shell-release-"));
  const bundleName = `kubectl-shell-${version}-${goos}-${goarch}`;
  const bundleDir = path.join(stageRoot, bundleName);
  fs.mkdirSync(bundleDir, { recursive: true });

  const outBin = path.join(bundleDir, exe);
  const build = spawnSync(
    "go",
    ["build", "-trimpath", "-ldflags=-s -w", "-o", outBin, "./cmd/kube"],
    {
      cwd: root,
      env: { ...process.env, GOOS: goos, GOARCH: goarch, CGO_ENABLED: "0" },
      stdio: "inherit",
    },
  );
  if (build.status !== 0) {
    fs.rmSync(stageRoot, { recursive: true, force: true });
    process.exit(build.status ?? 1);
  }

  fs.copyFileSync(path.join(root, "LICENSE"), path.join(bundleDir, "LICENSE"));
  fs.copyFileSync(path.join(root, "README.md"), path.join(bundleDir, "README.md"));

  const tarball = path.join(distDir, `${bundleName}.tar.gz`);
  const tar = spawnSync("tar", ["-czf", tarball, "-C", stageRoot, bundleName], {
    stdio: "inherit",
  });
  fs.rmSync(stageRoot, { recursive: true, force: true });
  if (tar.status !== 0) {
    process.exit(tar.status ?? 1);
  }
  console.error(`wrote ${tarball}`);
}
