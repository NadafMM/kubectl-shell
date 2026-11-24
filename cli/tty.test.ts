import assert from "node:assert/strict";
import test from "node:test";
import { openPromptInput, readSyncRetry } from "./tty.ts";

const errno = (code: string) => Object.assign(new Error(code), { code });

test("readSyncRetry keeps waiting through EAGAIN", () => {
  let calls = 0;
  let slept = 0;
  const n = readSyncRetry(
    0,
    Buffer.alloc(1),
    () => {
      calls += 1;
      if (calls < 60) throw errno("EAGAIN");
      return 1;
    },
    () => {
      slept += 1;
    },
  );
  assert.equal(n, 1);
  assert.equal(calls, 60);
  assert.equal(slept, 59);
});

test("readSyncRetry rethrows other errors", () => {
  assert.throws(
    () =>
      readSyncRetry(
        0,
        Buffer.alloc(1),
        () => {
          throw errno("EIO");
        },
        () => {},
      ),
    /EIO/,
  );
});

test("openPromptInput uses fd 0 on Windows", () => {
  assert.equal(
    openPromptInput("win32", () => {
      throw new Error("must not open");
    }).fd,
    0,
  );
});

test("openPromptInput opens /dev/tty on POSIX", () => {
  let opened = "";
  const input = openPromptInput("linux", (file) => {
    opened = file;
    return 42;
  });
  assert.equal(opened, "/dev/tty");
  assert.equal(input.fd, 42);
});

test("openPromptInput falls back to fd 0 when /dev/tty cannot open", () => {
  assert.equal(
    openPromptInput("darwin", () => {
      throw errno("ENXIO");
    }).fd,
    0,
  );
});
