#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<USAGE
Usage: $(basename "$0") --lyra-root <path> [--force]
USAGE
}

lyra_root=""
force="0"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --lyra-root)
      lyra_root="${2:-}"
      shift 2
      ;;
    --force)
      force="1"
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

if [[ -z "$lyra_root" ]]; then
  echo "--lyra-root is required." >&2
  usage >&2
  exit 2
fi

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ps_script="$script_dir/sync-bpx-plugin.ps1"

if [[ ! -f "$ps_script" ]]; then
  echo "PowerShell sync script not found: $ps_script" >&2
  exit 1
fi

lyra_root_win="$(wslpath -w "$lyra_root")"
ps_script_win="$(wslpath -w "$ps_script")"

cmd=(powershell.exe -NoProfile -ExecutionPolicy Bypass -File "$ps_script_win" -LyraRoot "$lyra_root_win")
if [[ "$force" == "1" ]]; then
  cmd+=(-Force)
fi

"${cmd[@]}"
