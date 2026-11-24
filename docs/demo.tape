# Demo GIF for README — recorded against a kind cluster.
# Tab uses bash completion from `kube completion` (same engine as the interactive shell).
Output docs/demo.gif

Set Shell bash
Set FontSize 16
Set Width 980
Set Height 480
Set TypingSpeed 40ms
Set Theme "Catppuccin Mocha"
Set Padding 14

Type "kube --version"
Enter
Sleep 1s

Type `eval "$(kube completion bash)"`
Enter
Sleep 700ms

# Unique prefix so Tab visibly expands (not a kubectl short name like "po")
Type "kube get depl"
Sleep 400ms
Tab
Sleep 1.8s
Enter
Sleep 2.2s

# Ambiguous prefix: Tab lists live matches
Type "kube get p"
Sleep 350ms
Tab
Sleep 400ms
Tab
Sleep 1.8s
Type "ods"
Sleep 400ms
Enter
Sleep 2.2s

Type "echo n | kube plan delete pod nginx-5fdc696c4c-6jtwm"
Enter
Sleep 2.6s
