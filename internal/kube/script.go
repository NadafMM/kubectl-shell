package kube

func CompletionScript(shell string) string {
	switch shell {
	case "bash":
		return `# bash completion for kube
_kube_completion() {
  local IFS=$'\n'
  COMPREPLY=()
  mapfile -t COMPREPLY < <(kube __complete --cursor "$COMP_POINT" --line "$COMP_LINE" 2>/dev/null)
  if [[ ${#COMPREPLY[@]} -eq 1 ]]; then
    local one="${COMPREPLY[0]}"
    if [[ "$one" != */ && "$one" != *= ]]; then
      COMPREPLY[0]="$one "
    fi
  fi
}
complete -o nospace -F _kube_completion kube
`
	case "zsh":
		return `#compdef kube
_kube() {
  local -a vals
  vals=("${(@f)$(kube __complete --cursor "$CURSOR" --line "$BUFFER" 2>/dev/null)}")
  (( ${#vals} )) || return
  if (( ${#vals} == 1 )); then
    local suf=' '
    [[ ${vals[1]} == */ || ${vals[1]} == *= ]] && suf=''
    compadd -S "$suf" -- "${vals[1]}"
  else
    compadd -- "${vals[@]}"
  fi
}
if (( ${+functions[compdef]} )); then
  compdef _kube kube
fi
`
	case "fish":
		return `function __kube_complete
  kube __complete --cursor (commandline -C) --line (commandline -b)
end
complete -c kube -f -a '(__kube_complete)'
`
	case "powershell", "pwsh":
		return `Register-ArgumentCompleter -Native -CommandName kube -ScriptBlock {
  param($wordToComplete, $commandAst, $cursorPosition)
  $line = $commandAst.ToString()
  if ($null -eq $line) { $line = '' }
  $cursor = $cursorPosition
  if ($null -eq $cursor) { $cursor = $line.Length }
  $out = & kube __complete --cursor $cursor --line $line 2>$null
  if (-not $out) { return }
  foreach ($item in $out) {
    if ([string]::IsNullOrWhiteSpace($item)) { continue }
    $value = $item
    [System.Management.Automation.CompletionResult]::new($value, $value, 'ParameterValue', $value)
  }
}
`
	default:
		return ""
	}
}
