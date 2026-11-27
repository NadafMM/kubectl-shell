package parse

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

// HookScript prints a shell function that keeps reads on native kubectl (or
// kubecolor when installed), routes mutating verbs through kube's blast-radius
// and policy gate, and supports per-shell ctx/ns isolation via `kubectl ctx`/`ns`.
func HookScript(shell string) string {
	switch shell {
	case "bash", "zsh":
		return `# kubectl-shell guard hook (` + shell + `)
kubectl() {
  if [[ "$1" == "ctx" && -n "$2" ]]; then
    eval "$(kube env --shell ` + shell + ` --session "tty-$$" --ctx "$2")"
    return $?
  fi
  if [[ "$1" == "ns" && -n "$2" ]]; then
    eval "$(kube env --shell ` + shell + ` --session "tty-$$" --ns "$2")"
    return $?
  fi
  local kbin="kubectl"
  if [[ "${KUBE_HOOK_COLOR:-auto}" != "never" ]] && command -v kubecolor >/dev/null 2>&1; then
    kbin="kubecolor"
  fi
  if [[ "${KUBE_GUARD_ALL:-}" == "1" ]]; then
    if [[ "$kbin" == "kubecolor" && -z "${KUBE_SHELL_KUBECTL:-}" && -t 1 ]]; then
      KUBE_SHELL_KUBECTL=kubecolor kube "$@"
    else
      kube "$@"
    fi
    return $?
  fi
  local first=""
  for arg in "$@"; do
    case "$arg" in
      -*) ;;
      *) first="$arg"; break ;;
    esac
  done
  case "$first" in
    delete|drain|cordon|uncordon|taint|apply|replace|patch|scale|rollout|set|label|annotate|exec|cp|attach|debug|port-forward|proxy|run|expose|create|edit)
      if [[ "$kbin" == "kubecolor" && -z "${KUBE_SHELL_KUBECTL:-}" && -t 1 ]]; then
        KUBE_SHELL_KUBECTL=kubecolor kube "$@"
      else
        kube "$@"
      fi
      ;;
    *)
      if [[ "$kbin" == "kubecolor" ]]; then
        command kubecolor "$@"
      else
        command kubectl "$@"
      fi
      ;;
  esac
}
`
	case "fish":
		return `# kubectl-shell guard hook (fish)
function kubectl --wraps kubectl
  if test (count $argv) -ge 2; and test "$argv[1]" = "ctx"
    kube env --shell fish --session "tty-$fish_pid" --ctx "$argv[2]" | source
    return $status
  end
  if test (count $argv) -ge 2; and test "$argv[1]" = "ns"
    kube env --shell fish --session "tty-$fish_pid" --ns "$argv[2]" | source
    return $status
  end
  set -l kbin "kubectl"
  if test "$KUBE_HOOK_COLOR" != "never"; and type -q kubecolor
    set kbin "kubecolor"
  end
  if test "$KUBE_GUARD_ALL" = "1"
    if test "$kbin" = "kubecolor"; and not set -q KUBE_SHELL_KUBECTL; and isatty stdout
      env KUBE_SHELL_KUBECTL=kubecolor kube $argv
    else
      kube $argv
    end
    return $status
  end
  set -l first ""
  for arg in $argv
    if not string match -q -- "-*" $arg
      set first $arg
      break
    end
  end
  switch $first
    case delete drain cordon uncordon taint apply replace patch scale rollout set label annotate exec cp attach debug port-forward proxy run expose create edit
      if test "$kbin" = "kubecolor"; and not set -q KUBE_SHELL_KUBECTL; and isatty stdout
        env KUBE_SHELL_KUBECTL=kubecolor kube $argv
      else
        kube $argv
      end
    case '*'
      if test "$kbin" = "kubecolor"
        command kubecolor $argv
      else
        command kubectl $argv
      end
  end
end
`
	case "powershell", "pwsh":
		return `# kubectl-shell guard hook (powershell)
function kubectl {
  if ($args.Count -ge 2 -and $args[0] -eq 'ctx') {
    kube env --shell powershell --session "tty-$PID" --ctx $args[1] | Out-String | Invoke-Expression
    return
  }
  if ($args.Count -ge 2 -and $args[0] -eq 'ns') {
    kube env --shell powershell --session "tty-$PID" --ns $args[1] | Out-String | Invoke-Expression
    return
  }
  $colorCmd = $null
  if ($env:KUBE_HOOK_COLOR -ne 'never') {
    $colorCmd = Get-Command kubecolor -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
  }
  $native = Get-Command kubectl -CommandType Application -ErrorAction Stop | Select-Object -First 1
  if ($env:KUBE_GUARD_ALL -eq '1') {
    & kube @args
    return
  }
  $first = ''
  foreach ($a in $args) {
    if ($a -notlike '-*') { $first = $a; break }
  }
  $guarded = @('delete','drain','cordon','uncordon','taint','apply','replace','patch','scale','rollout','set','label','annotate','exec','cp','attach','debug','port-forward','proxy','run','expose','create','edit')
  if ($guarded -contains $first) {
    & kube @args
  } elseif ($colorCmd) {
    & $colorCmd @args
  } else {
    & $native @args
  }
}
`
	default:
		return ""
	}
}
