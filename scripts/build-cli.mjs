import * as esbuild from "esbuild";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const out = path.join(root, "cli", "kube.cjs");
fs.mkdirSync(path.dirname(out), { recursive: true });

const pkg = JSON.parse(fs.readFileSync(path.join(root, "package.json"), "utf8"));

await esbuild.build({
  entryPoints: [path.join(root, "cli", "bin.ts")],
  bundle: true,
  platform: "node",
  format: "cjs",
  outfile: out,
  banner: { js: "#!/usr/bin/env node" },
  logLevel: "warning",
  absWorkingDir: root,
  define: { __KUBE_VERSION__: JSON.stringify(pkg.version) },
});

fs.chmodSync(out, 0o755);
console.log(out);
