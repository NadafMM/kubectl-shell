/** One-shot --yes only; do not pre-confirm an interactive shell. */
export function allowYes(yes: boolean, argc: number, tty: boolean): boolean {
  if (!yes) return false;
  if (argc === 0 && tty) return false;
  return true;
}

/** Confirm when the first character is y or Y, and drop that line. */
export function readAnswer(chunks: string[]): boolean {
  let answer = "";
  while (chunks.length > 0) {
    const next = chunks.shift() ?? "";
    const newline = next.search(/[\r\n]/);
    if (newline >= 0) {
      answer += next.slice(0, newline);
      const rest = next.slice(newline + 1);
      if (rest) chunks.unshift(rest);
      break;
    }
    answer += next;
  }
  const first = answer.trimStart()[0];
  return first === "y" || first === "Y";
}

/** Read one answer line byte by byte; a terminal that cannot be read counts as no. */
export function readConfirm(
  fd: number,
  read: (fd: number, buf: Buffer) => number,
  warn: (text: string) => void,
): boolean {
  const chunks: string[] = [];
  const buf = Buffer.alloc(1);
  try {
    for (;;) {
      const n = read(fd, buf);
      if (n <= 0) break;
      const text = buf.toString("utf8", 0, n);
      chunks.push(text);
      if (text === "\n" || text === "\r") break;
    }
  } catch (error) {
    const err = error as NodeJS.ErrnoException;
    warn(`\nkube: could not read the answer (${err.code ?? err.message}); not running it.\n`);
    return false;
  }
  return readAnswer(chunks);
}
