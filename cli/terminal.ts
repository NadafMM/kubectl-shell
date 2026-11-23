const SLEEP = new Int32Array(new SharedArrayBuffer(4));

/** Synchronous pause used when stdin is non-blocking. One buffer for the process. */
export function sleepMs(ms: number) {
  Atomics.wait(SLEEP, 0, 0, ms);
}

export function refreshLine(rl: object) {
  const refresh = (rl as { _refreshLine?: () => void })._refreshLine;
  refresh?.();
}

export function plainText(value: string): string {
  let out = "";
  for (let i = 0; i < value.length; i += 1) {
    const code = value.charCodeAt(i);
    if (code === 0x1b) {
      const next = value[i + 1];
      if (next === "]") {
        i += 2;
        while (
          i < value.length &&
          value.charCodeAt(i) !== 0x07 &&
          value.slice(i, i + 2) !== "\u001b\\"
        )
          i += 1;
        if (value.slice(i, i + 2) === "\u001b\\") i += 1;
      } else if (next === "[") {
        i += 2;
        while (i < value.length && (value.charCodeAt(i) < 0x40 || value.charCodeAt(i) > 0x7e))
          i += 1;
      }
      continue;
    }
    if (code < 0x20 || (code >= 0x7f && code <= 0x9f)) continue;
    out += value[i];
  }
  return out;
}

export function completionValue(value: string): string | null {
  if (plainText(value) !== value) return null;
  if (/[`$;|\n]/.test(value)) return null;
  return value;
}
