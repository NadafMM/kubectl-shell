import assert from "node:assert/strict";
import test from "node:test";
import { allowYes, readAnswer, readConfirm } from "./confirm.ts";

test("y confirms and the rest of the line is consumed", () => {
  const chunks = ["y", "e", "s", " ", "l", "e", "f", "t", "o", "v", "e", "r", "\n", "g", "e", "t"];
  assert.equal(readAnswer(chunks), true);
  assert.deepEqual(chunks, ["g", "e", "t"]);
});

test("n does not confirm", () => {
  const chunks = ["n", "\n"];
  assert.equal(readAnswer(chunks), false);
  assert.deepEqual(chunks, []);
});

test("yes does not arm an empty tty shell", () => {
  assert.equal(allowYes(true, 0, true), false);
  assert.equal(allowYes(true, 0, false), true);
  assert.equal(allowYes(true, 2, true), true);
  assert.equal(allowYes(false, 0, true), false);
});

const reader = (bytes: string) => {
  let at = 0;
  return (_fd: number, buf: Buffer) => {
    if (at >= bytes.length) return 0;
    buf.write(bytes[at++]!);
    return 1;
  };
};

test("readConfirm stops at the end of the answer line", () => {
  assert.equal(
    readConfirm(0, reader("y\rdelete"), () => {}),
    true,
  );
  assert.equal(
    readConfirm(0, reader("\r"), () => {}),
    false,
  );
});

test("readConfirm says no and warns once when the terminal cannot be read", () => {
  const warnings: string[] = [];
  const answer = readConfirm(
    0,
    () => {
      throw Object.assign(new Error("i/o error"), { code: "EIO" });
    },
    (text) => warnings.push(text),
  );
  assert.equal(answer, false);
  assert.equal(warnings.length, 1);
  assert.match(warnings[0]!, /could not read the answer \(EIO\); not running it\./);
});
