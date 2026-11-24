import fs from "node:fs";
import { sleepMs } from "./terminal";

export type PromptInput = { fd: number; close: () => void };

const RETRY = new Set(["EAGAIN", "EWOULDBLOCK", "EINTR"]);

// Node switches fd 0 to non-blocking once process.stdin is touched, so on POSIX a fresh
// /dev/tty handle is the only way to get a read that actually waits for the user.
export function openPromptInput(
  platform: NodeJS.Platform = process.platform,
  open: (file: string) => number = (file) => fs.openSync(file, "r"),
): PromptInput {
  const stdin: PromptInput = { fd: 0, close: () => {} };
  if (platform === "win32") return stdin;
  let fd: number;
  try {
    fd = open("/dev/tty");
  } catch {
    return stdin;
  }
  return { fd, close: () => fs.closeSync(fd) };
}

export function readSyncRetry(
  fd: number,
  buf: Buffer,
  read: (fd: number, buf: Buffer) => number = (f, b) => fs.readSync(f, b, 0, b.length, null),
  sleep: (ms: number) => void = sleepMs,
): number {
  for (;;) {
    try {
      return read(fd, buf);
    } catch (error) {
      if (!RETRY.has((error as NodeJS.ErrnoException).code ?? "")) throw error;
      sleep(10);
    }
  }
}
