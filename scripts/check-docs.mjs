import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");

const required = [
  "README.md",
  "LICENSE",
  "CONTRIBUTING.md",
  "SECURITY.md",
  "CODE_OF_CONDUCT.md",
  "docs/security/threat-model.md",
  "docs/ci.md",
  "docs/packaging.md",
  "VERSION",
  "Makefile",
];

for (const rel of required) {
  if (!fs.existsSync(path.join(root, rel))) throw new Error(`missing ${rel}`);
}

const readme = fs.readFileSync(path.join(root, "README.md"), "utf8");
for (const needle of ["CONTRIBUTING.md", "docs/security/threat-model.md", "LICENSE"]) {
  if (!readme.includes(needle)) throw new Error(`README must mention ${needle}`);
}

const localLink = /]\((docs\/[^)#]+\.md)\)/g;
for (const match of readme.matchAll(localLink)) {
  const target = path.join(root, match[1]);
  if (!fs.existsSync(target)) throw new Error(`broken README link: ${match[1]}`);
}

const pkg = JSON.parse(fs.readFileSync(path.join(root, "package.json"), "utf8"));
const scriptNames = new Set(Object.keys(pkg.scripts ?? {}));

function listDocFiles(dir, acc = []) {
  for (const ent of fs.readdirSync(dir, { withFileTypes: true })) {
    const rel = path
      .join(dir, ent.name)
      .slice(root.length + 1)
      .replace(/\\/g, "/");
    if (ent.isDirectory()) {
      if (rel === "docs/superpowers") continue;
      listDocFiles(path.join(dir, ent.name), acc);
    } else if (ent.name.endsWith(".md")) {
      acc.push(rel);
    }
  }
  return acc;
}

const docFiles = ["README.md", "CONTRIBUTING.md", ...listDocFiles(path.join(root, "docs"))];
const npmRun = /npm run ([\w:-]+)/g;

for (const rel of docFiles) {
  const text = fs.readFileSync(path.join(root, rel), "utf8");
  for (const match of text.matchAll(npmRun)) {
    const name = match[1];
    if (!scriptNames.has(name)) {
      throw new Error(`unknown npm script "${name}" in ${rel}`);
    }
  }
}

console.log("check-docs: ok");
