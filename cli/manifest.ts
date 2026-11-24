import fs from "node:fs";
import path from "node:path";

export type ManifestRef = { kind: string; name: string; namespace?: string };

export function manifestRefs(
  file: string,
): { ok: true; refs: ManifestRef[] } | { ok: false; error: string } {
  let stat: fs.Stats;
  try {
    stat = fs.statSync(file);
  } catch (error) {
    const err = error as NodeJS.ErrnoException;
    return { ok: false, error: err.code ?? err.message };
  }
  if (stat.isDirectory()) {
    const refs: ManifestRef[] = [];
    for (const name of fs.readdirSync(file)) {
      if (!/\.(ya?ml|json)$/i.test(name)) continue;
      const nested = manifestRefs(path.join(file, name));
      if (!nested.ok) return nested;
      refs.push(...nested.refs);
    }
    return { ok: true, refs };
  }
  try {
    return { ok: true, refs: parseManifest(fs.readFileSync(file, "utf8")) };
  } catch (error) {
    const err = error as NodeJS.ErrnoException;
    return { ok: false, error: err.code ?? err.message };
  }
}

export function parseManifest(text: string): ManifestRef[] {
  const trimmed = text.trim();
  if (!trimmed) return [];
  if (trimmed.startsWith("{") || trimmed.startsWith("[")) {
    try {
      return jsonRefs(JSON.parse(trimmed) as unknown);
    } catch {
      return [];
    }
  }
  const refs: ManifestRef[] = [];
  for (const doc of trimmed.split(/\r?\n---\r?\n/)) {
    const ref = yamlRef(doc);
    if (ref) refs.push(ref);
  }
  return refs;
}

function jsonRefs(value: unknown): ManifestRef[] {
  if (Array.isArray(value)) return value.flatMap(jsonRefs);
  if (!value || typeof value !== "object") return [];
  const doc = value as {
    kind?: string;
    items?: unknown;
    metadata?: { name?: string; namespace?: string };
  };
  if (Array.isArray(doc.items)) return jsonRefs(doc.items);
  if (!doc.kind || !doc.metadata?.name) return [];
  return [{ kind: doc.kind, name: doc.metadata.name, namespace: doc.metadata.namespace }];
}

function yamlRef(doc: string): ManifestRef | null {
  let kind = "";
  let name = "";
  let namespace: string | undefined;
  let inMeta = false;
  let metaIndent = 0;
  for (const raw of doc.split(/\r?\n/)) {
    if (!raw.trim() || raw.trimStart().startsWith("#")) continue;
    const indent = raw.length - raw.trimStart().length;
    const line = raw.trim();
    if (inMeta && indent <= metaIndent) inMeta = false;
    const match = /^([A-Za-z0-9_]+):\s*(.*)$/.exec(line);
    if (!match) continue;
    const key = match[1]!;
    const value = strip(match[2] ?? "");
    if (!inMeta && indent === 0 && key === "kind" && value) kind = value;
    if (!inMeta && indent === 0 && key === "metadata") {
      inMeta = true;
      metaIndent = indent;
      continue;
    }
    if (inMeta && indent > metaIndent) {
      if (key === "name" && value && !name) name = value;
      if (key === "namespace" && value && namespace === undefined) namespace = value;
    }
  }
  if (!kind || !name) return null;
  return { kind, name, namespace };
}

function strip(value: string): string {
  const cut = value.split(/\s+#/)[0]?.trim() ?? "";
  if (
    cut.length >= 2 &&
    ((cut.startsWith('"') && cut.endsWith('"')) || (cut.startsWith("'") && cut.endsWith("'")))
  ) {
    return cut.slice(1, -1);
  }
  return cut;
}
