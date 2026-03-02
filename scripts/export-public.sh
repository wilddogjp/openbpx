#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Export files intended for the public bpx repository.

Usage:
  scripts/export-public.sh --output-dir <path> [--repo-root <path>] [--dry-run]

Options:
  --repo-root   Source repository root (default: current directory)
  --output-dir  Destination directory for exported content (required)
  --dry-run     Print planned copy list without writing files

Notes:
  - Only files tracked by git are exported.
  - If --output-dir already contains files, it must be a directory previously
    created by this script (marker file: .bpx-export-root).
EOF
}

REPO_ROOT="."
OUTPUT_DIR=""
DRY_RUN="false"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo-root)
      REPO_ROOT="$2"
      shift 2
      ;;
    --output-dir)
      OUTPUT_DIR="$2"
      shift 2
      ;;
    --dry-run)
      DRY_RUN="true"
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown option: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [[ -z "${OUTPUT_DIR}" ]]; then
  echo "--output-dir is required" >&2
  usage >&2
  exit 1
fi

REPO_ROOT="$(cd "${REPO_ROOT}" && pwd)"
OUTPUT_DIR="$(mkdir -p "${OUTPUT_DIR}" && cd "${OUTPUT_DIR}" && pwd)"

if ! git -C "${REPO_ROOT}" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  echo "repo root is not a git worktree: ${REPO_ROOT}" >&2
  exit 1
fi

if [[ "${OUTPUT_DIR}" == "/" || "${OUTPUT_DIR}" == "${HOME}" || "${OUTPUT_DIR}" == "${REPO_ROOT}" ]]; then
  echo "refusing dangerous output-dir: ${OUTPUT_DIR}" >&2
  exit 1
fi

case "${OUTPUT_DIR}" in
  "${REPO_ROOT}"/*)
    echo "output-dir must be outside repo root: ${OUTPUT_DIR}" >&2
    exit 1
    ;;
esac

INCLUDE_PATHS=(
  ".agents"
  ".claude-plugin"
  "CHANGELOG.md"
  "CONTRIBUTING.md"
  ".github/workflows/build-artifacts-main.yml"
  ".github/workflows/fuzz-weekly.yml"
  ".github/workflows/release.yml"
  ".github/workflows/test.yml"
  ".gitignore"
  ".goreleaser.yaml"
  "ISSUE.md"
  "LICENSE"
  "README.md"
  "SECURITY.md"
  "cmd"
  "docs/build-distribution.md"
  "docs/assets"
  "docs/commands.md"
  "docs/dev/ue56-notes.md"
  "docs/disasm-script-extraction.md"
  "docs/test-fixtures.md"
  "go.mod"
  "go.sum"
  "internal"
  "pkg"
  "skills"
  "scripts"
  "testdata"
)

if [[ "${DRY_RUN}" == "false" ]]; then
  marker="${OUTPUT_DIR}/.bpx-export-root"
  if [[ -n "$(find "${OUTPUT_DIR}" -mindepth 1 -maxdepth 1 ! -name ".bpx-export-root" -print -quit)" && ! -f "${marker}" ]]; then
    echo "refusing to clean non-export directory: ${OUTPUT_DIR}" >&2
    echo "use an empty directory or a directory previously created by this script" >&2
    exit 1
  fi
  find "${OUTPUT_DIR}" -mindepth 1 -maxdepth 1 ! -name ".bpx-export-root" -exec rm -rf {} +
  : > "${marker}"
fi

echo "Export source: ${REPO_ROOT}"
echo "Export destination: ${OUTPUT_DIR}"
echo "Dry run: ${DRY_RUN}"
echo ""
echo "Paths:"

declare -A tracked_file_set=()

for rel in "${INCLUDE_PATHS[@]}"; do
  mapfile -t tracked_paths < <(git -C "${REPO_ROOT}" ls-files -- "${rel}")

  if [[ ${#tracked_paths[@]} -eq 0 ]]; then
    echo "  - [skip] ${rel} (no tracked files)"
    continue
  fi

  echo "  - [copy] ${rel} (${#tracked_paths[@]} tracked files)"
  for tracked_path in "${tracked_paths[@]}"; do
    if [[ ! -e "${REPO_ROOT}/${tracked_path}" ]]; then
      echo "    - [skip] ${tracked_path} (missing in working tree)"
      continue
    fi
    tracked_file_set["${tracked_path}"]=1
  done
done

mapfile -t tracked_files < <(printf '%s\n' "${!tracked_file_set[@]}" | sort)

if [[ "${DRY_RUN}" == "false" ]]; then
  copied_count=0
  for rel in "${tracked_files[@]}"; do
    src="${REPO_ROOT}/${rel}"
    dst="${OUTPUT_DIR}/${rel}"
    mkdir -p "$(dirname "${dst}")"
    cp -a "${src}" "${dst}"
    copied_count=$((copied_count + 1))
  done
fi

if [[ "${DRY_RUN}" == "false" ]]; then
  echo ""
  echo "Export completed. Copied tracked files: ${copied_count}"
  find "${OUTPUT_DIR}" -type f ! -name ".bpx-export-root" | sed "s#^${OUTPUT_DIR}/#  - #g" | sort
fi
