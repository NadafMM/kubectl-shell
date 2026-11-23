import fs from "node:fs";

const logPath = process.argv[2] || "coverage/run.log";
if (!fs.existsSync(logPath)) {
  console.error(`missing ${logPath}; run KUBE_TEST_COVERAGE=1 npm test 2>&1 | tee ${logPath}`);
  process.exit(1);
}

const log = fs.readFileSync(logPath, "utf8");
const match = log.match(/all files\s*\|\s*([\d.]+)\s*\|\s*([\d.]+)\s*\|\s*([\d.]+)/);
if (!match) {
  console.error("could not parse Node test coverage summary from log");
  process.exit(1);
}

const lines = Number(match[1]);
const branches = Number(match[2]);
const funcs = Number(match[3]);

const mins = {
  lines: Number(process.env.COVERAGE_MIN_LINES || "42"),
  branches: Number(process.env.COVERAGE_MIN_BRANCHES || "35"),
  functions: Number(process.env.COVERAGE_MIN_FUNCTIONS || "48"),
};

if (lines < mins.lines || branches < mins.branches || funcs < mins.functions) {
  console.error(
    `coverage below floor: lines ${lines}% (min ${mins.lines}), branches ${branches}% (min ${mins.branches}), funcs ${funcs}% (min ${mins.functions})`,
  );
  process.exit(1);
}

console.log(`coverage-check: lines ${lines}% branches ${branches}% functions ${funcs}%`);
