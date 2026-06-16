#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<USAGE
Usage: $(basename "$0") [--lyra-root <path>] [--bpx-repo-root <path>] [--scope <csv>] [--include <csv>] [--force] [--editor-cmd-path <path>] [--skip-editor-build] [--golden-root <path>] [--ue-engine-root <path>] [--config <path>]
USAGE
}

lyra_root=""
bpx_repo_root=""
scope="1,2"
include=""
force="0"
editor_cmd_path=""
skip_editor_build="0"
golden_root=""
ue_engine_root=""
config_path=""

to_windows_path() {
  local input="$1"
  if [[ "$input" =~ ^[A-Za-z]:[\\/].* ]] || [[ "$input" == \\\\* ]]; then
    printf '%s' "$input"
    return 0
  fi
  wslpath -w "$input"
}

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
    --golden-root)
      golden_root="${2:-}"
      shift 2
      ;;
    --ue-engine-root)
      ue_engine_root="${2:-}"
      shift 2
      ;;
    --config)
      config_path="${2:-}"
      shift 2
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

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ps_script="$script_dir/gen-fixtures.ps1"
sync_ps_script="$script_dir/sync-bpx-plugin.ps1"
default_config_path="$script_dir/local-fixtures.config.json"

if [[ ! -f "$ps_script" ]]; then
  echo "PowerShell generation script not found: $ps_script" >&2
  exit 1
fi

if [[ ! -f "$sync_ps_script" ]]; then
  echo "PowerShell sync script not found: $sync_ps_script" >&2
  exit 1
fi

ps_script_win="$(to_windows_path "$ps_script")"
sync_ps_script_win="$(to_windows_path "$sync_ps_script")"

effective_config_path="$config_path"
if [[ -z "$effective_config_path" && -f "$default_config_path" ]]; then
  effective_config_path="$default_config_path"
fi

emit_build_profiles() {
  python3 - "$effective_config_path" "$lyra_root" "$ue_engine_root" "$editor_cmd_path" <<'PY'
import json
import ntpath
import os
import sys

config_path, cli_lyra_root, cli_ue_engine_root, cli_editor_cmd_path = sys.argv[1:5]
config = None
if config_path:
    with open(config_path, encoding="utf-8-sig") as f:
        config = json.load(f)

def cfg_value(obj, key):
    if not isinstance(obj, dict):
        return ""
    value = obj.get(key, "")
    return value if isinstance(value, str) else ""

def derive_engine_root(lyra_root, ue_engine_root, editor_cmd_path):
    if ue_engine_root:
        return ue_engine_root
    if editor_cmd_path:
        return ntpath.normpath(ntpath.join(ntpath.dirname(editor_cmd_path), "..", "..", ".."))
    if lyra_root:
        return ntpath.normpath(ntpath.join(lyra_root, "..", "..", ".."))
    return ""

def emit_profile(name, lyra_root, ue_engine_root, editor_cmd_path):
    if not lyra_root:
        raise SystemExit(f"missing LyraRoot for profile {name}")
    resolved_engine_root = derive_engine_root(lyra_root, ue_engine_root, editor_cmd_path)
    if not resolved_engine_root:
        raise SystemExit(f"missing UEEngineRoot for profile {name}")
    build_bat = ntpath.join(resolved_engine_root, "Engine", "Build", "BatchFiles", "Build.bat")
    uproject = ntpath.join(lyra_root, "Lyra.uproject")
    print("\t".join([name, lyra_root, build_bat, uproject]))

if cli_lyra_root:
    emit_profile(
        "cli",
        cli_lyra_root,
        cli_ue_engine_root or cfg_value(config, "ueEngineRoot"),
        cli_editor_cmd_path or cfg_value(config, "editorCmdPath"),
    )
    raise SystemExit(0)

engines = config.get("engines") if isinstance(config, dict) else None
if isinstance(engines, dict) and engines:
    for name in sorted(engines):
        engine_cfg = engines.get(name) or {}
        emit_profile(
            name,
            cfg_value(engine_cfg, "lyraRoot") or cfg_value(config, "lyraRoot"),
            cli_ue_engine_root or cfg_value(engine_cfg, "ueEngineRoot") or cfg_value(config, "ueEngineRoot"),
            cli_editor_cmd_path or cfg_value(engine_cfg, "editorCmdPath") or cfg_value(config, "editorCmdPath"),
        )
    raise SystemExit(0)

flat_lyra_root = cfg_value(config, "lyraRoot")
if not flat_lyra_root:
    raise SystemExit("LyraRoot is required (pass --lyra-root or configure scripts/local-fixtures.config.json)")

emit_profile(
    "default",
    flat_lyra_root,
    cli_ue_engine_root or cfg_value(config, "ueEngineRoot"),
    cli_editor_cmd_path or cfg_value(config, "editorCmdPath"),
)
PY
}

prebuild_profiles() {
  local -a profiles=()
  local profile_name
  local profile_lyra_root
  local profile_build_bat
  local profile_uproject
  mapfile -t profiles < <(emit_build_profiles)

  for profile in "${profiles[@]}"; do
    IFS=$'\t' read -r profile_name profile_lyra_root profile_build_bat profile_uproject <<<"$profile"
    [[ -z "${profile_name:-}" ]] && continue

    echo
    echo "=== Prebuild BPX fixture plugin: $profile_name ==="
    echo "LyraRoot: $profile_lyra_root"

    sync_cmd=(
      powershell.exe
      -NoProfile
      -ExecutionPolicy Bypass
      -File "$sync_ps_script_win"
      -LyraRoot "$profile_lyra_root"
    )
    if [[ "$force" == "1" ]]; then
      sync_cmd+=(-Force)
    fi
    "${sync_cmd[@]}"

    powershell.exe \
      -NoProfile \
      -ExecutionPolicy Bypass \
      -Command "& '$profile_build_bat' LyraEditor Win64 Development '-Project=$profile_uproject' '-Module=BPXFixtureGenerator' -NoUBTMakefiles -WaitMutex -NoHotReloadFromIDE" \
      < /dev/null
  done
}

if [[ "$skip_editor_build" != "1" ]]; then
  prebuild_profiles
  skip_editor_build="1"
fi

cmd=(
  powershell.exe
  -NoProfile
  -ExecutionPolicy Bypass
  -File "$ps_script_win"
  -Scope "$scope"
)

if [[ -n "$lyra_root" ]]; then
  cmd+=(-LyraRoot "$(to_windows_path "$lyra_root")")
fi

if [[ -n "$bpx_repo_root" ]]; then
  cmd+=(-BpxRepoRoot "$(to_windows_path "$bpx_repo_root")")
fi

if [[ -n "$include" ]]; then
  cmd+=(-Include "$include")
fi

if [[ "$force" == "1" ]]; then
  cmd+=(-Force)
fi

if [[ -n "$editor_cmd_path" ]]; then
  cmd+=(-EditorCmdPath "$(to_windows_path "$editor_cmd_path")")
fi

if [[ "$skip_editor_build" == "1" ]]; then
  cmd+=(-SkipEditorBuild)
fi

if [[ -n "$golden_root" ]]; then
  cmd+=(-GoldenRoot "$(to_windows_path "$golden_root")")
fi

if [[ -n "$ue_engine_root" ]]; then
  cmd+=(-UEEngineRoot "$(to_windows_path "$ue_engine_root")")
fi

if [[ -n "$config_path" ]]; then
  cmd+=(-ConfigPath "$(to_windows_path "$config_path")")
fi

"${cmd[@]}"
