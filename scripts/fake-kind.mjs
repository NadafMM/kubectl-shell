import fs from "node:fs";
import path from "node:path";

const args = process.argv.slice(2);
const log = process.env.KUBE_FAKE_KIND_LOG;
if (log) fs.appendFileSync(log, `${JSON.stringify(args)}\n`);

if (args[0] === "create" && args[1] === "cluster") {
  const index = args.indexOf("--kubeconfig");
  const file = index >= 0 ? args[index + 1] : "";
  if (file) {
    fs.mkdirSync(path.dirname(file), { recursive: true });
    fs.writeFileSync(
      file,
      "apiVersion: v1\nkind: Config\ncurrent-context: kind-kube-shell-sandbox\n",
    );
  }
  process.exit(0);
}

if (args[0] === "delete" && args[1] === "cluster") process.exit(0);

process.stderr.write("fake kind: unexpected command\n");
process.exit(1);
