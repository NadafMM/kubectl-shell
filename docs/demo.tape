# Demo GIF for README — recorded against a kind cluster (Task 11).
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
Sleep 1.2s

Type "kube get pods"
Enter
Sleep 2.5s

Type "echo n | kube plan delete pod nginx-5fdc696c4c-vjm4w"
Enter
Sleep 2.8s

Type "printf q | kube view pods"
Enter
Sleep 2.5s

Type "kube ask how do I list deployments"
Enter
Sleep 3s
