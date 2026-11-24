import fs from "node:fs";
import os from "node:os";
import path from "node:path";

export function stateDir(): string {
  return path.join(os.homedir(), ".kube-shell");
}

export function writePrivate(file: string, data: string): void {
  const dir = path.dirname(file);
  fs.mkdirSync(dir, { recursive: true, mode: 0o700 });
  fs.chmodSync(dir, 0o700);
  const tmp = `${file}.${process.pid}.tmp`;
  fs.writeFileSync(tmp, data, { mode: 0o600 });
  fs.chmodSync(tmp, 0o600);
  fs.renameSync(tmp, file);
  fs.chmodSync(file, 0o600);
}

export function tryWritePrivate(file: string, data: string): boolean {
  try {
    writePrivate(file, data);
    return true;
  } catch {
    return false;
  }
}
