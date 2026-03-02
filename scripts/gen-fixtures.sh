#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<USAGE
Usage: $(basename "$0") --lyra-root <path> --bpx-repo-root <path> [--scope <csv>] [--include <csv>] [--force] [--editor-cmd-path <path>] [--skip-editor-build]
USAGE
}

lyra_root=""
bpx_repo_root=""
scope="1,2"
include=""
force="0"
editor_cmd_path=""
skip_editor_build="0"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --lyra-root)
      lyra_root="${2:-}"
      shift 2
      ;;
    --bpx-repo-root)
      bpx_repo_root="${2:-}"
      shift 2
      ;;
    --scope)
      scope="${2:-}"
      shift 2
      ;;
    --include)
      include="${2:-}"
      shift 2
      ;;
    --force)
      force="1"
      shift
      ;;
    --editor-cmd-path)
      editor_cmd_path="${2:-}"
      shift 2
      ;;
    --skip-editor-build)
      skip_editor_build="1"
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ -z "$lyra_root" || -z "$bpx_repo_root" ]]; then
  echo "--lyra-root and --bpx-repo-root are required." >&2
  usage >&2
  exit 2
fi

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ps_script="$script_dir/gen-fixtures.ps1"

if [[ ! -f "$ps_script" ]]; then
  echo "PowerShell generation script not found: $ps_script" >&2
  exit 1
fi

lyra_root_win="$(wslpath -w "$lyra_root")"
bpx_repo_root_win="$(wslpath -w "$bpx_repo_root")"
ps_script_win="$(wslpath -w "$ps_script")"

cmd=(
  powershell.exe
  -NoProfile
  -ExecutionPolicy Bypass
  -File "$ps_script_win"
  -LyraRoot "$lyra_root_win"
  -BpxRepoRoot "$bpx_repo_root_win"
  -Scope "$scope"
)

if [[ -n "$include" ]]; then
  cmd+=(-Include "$include")
fi

if [[ "$force" == "1" ]]; then
  cmd+=(-Force)
fi

if [[ -n "$editor_cmd_path" ]]; then
  cmd+=(-EditorCmdPath "$(wslpath -w "$editor_cmd_path")")
fi

if [[ "$skip_editor_build" == "1" ]]; then
  cmd+=(-SkipEditorBuild)
fi

"${cmd[@]}"
