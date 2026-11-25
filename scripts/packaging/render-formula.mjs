const formulaTemplate =
  "class KubectlShell < Formula\n" +
  '  desc "kubectl shell with live Tab completion and a terminal resource view"\n' +
  '  homepage "https://github.com/NadafMM/kubectl-shell"\n' +
  '  url "https://github.com/NadafMM/kubectl-shell/archive/refs/tags/v__VERSION__.tar.gz"\n' +
  '  sha256 "__SHA256__"\n' +
  '  license "MIT"\n' +
  "\n" +
  '  depends_on "go" => :build\n' +
  "\n" +
  "  def install\n" +
  '    system "go", "build", *std_go_args(ldflags: "-s -w -X main.version=#{version}", output: bin/"kube"), "./cmd/kube"\n' +
  '    generate_completions_from_executable(bin/"kube", "completion", shells: [:bash, :zsh, :fish])\n' +
  "  end\n" +
  "\n" +
  "  test do\n" +
  '    assert_match version.to_s, shell_output("#{bin}/kube --version")\n' +
  "  end\n" +
  "end\n";

const version = process.argv[2];
const sha256 = process.argv[3];

if (!version || !sha256) {
  console.error("Usage: node scripts/packaging/render-formula.mjs <version> <sha256>");
  process.exit(1);
}

const formula = formulaTemplate.replaceAll("__VERSION__", version).replaceAll("__SHA256__", sha256);

process.stdout.write(formula);
