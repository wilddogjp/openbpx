#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Install BPX CLI from GitHub Releases with SHA-256 verification.

Usage:
  install-bpx-from-release.sh [--version <vX.Y.Z>] [--install-dir <dir>] [--repo <owner/name>]

Options:
  --version      Release tag (default: latest)
  --install-dir  Installation directory for bpx (default: ~/.local/bin)
  --repo         GitHub repository (default: wilddogjp/openbpx)
  -h, --help     Show help
EOF
}

repo="wilddogjp/openbpx"
version=""
install_dir="${HOME}/.local/bin"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version)
      version="$2"
      shift 2
      ;;
    --install-dir)
      install_dir="$2"
      shift 2
      ;;
    --repo)
      repo="$2"
      shift 2
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

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "required command not found: $1" >&2
    exit 1
  fi
}

sha256_file() {
  local target="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$target" | awk '{print $1}'
    return
  fi
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$target" | awk '{print $1}'
    return
  fi
  if command -v openssl >/dev/null 2>&1; then
    openssl dgst -sha256 "$target" | awk '{print $NF}'
    return
  fi
  echo "no SHA-256 tool found (need sha256sum, shasum, or openssl)" >&2
  exit 1
}

normalize_version() {
  local raw="$1"
  if [[ -z "$raw" ]]; then
    echo ""
    return
  fi
  if [[ "$raw" == v* ]]; then
    echo "$raw"
  else
    echo "v$raw"
  fi
}

detect_latest_version() {
  require_cmd curl
  local api_url="https://api.github.com/repos/${repo}/releases/latest"
  local tag
  tag="$(curl -fsSL "$api_url" | awk -F '"' '/"tag_name"/ {print $4; exit}')"
  if [[ -z "$tag" ]]; then
    echo "failed to resolve latest release tag from ${api_url}" >&2
    exit 1
  fi
  echo "$tag"
}

detect_os() {
  case "$(uname -s | tr '[:upper:]' '[:lower:]')" in
    linux) echo "linux" ;;
    darwin) echo "darwin" ;;
    *)
      echo "unsupported OS for this script: $(uname -s)" >&2
      echo "use the PowerShell installer for Windows" >&2
      exit 1
      ;;
  esac
}

detect_arch() {
  case "$(uname -m | tr '[:upper:]' '[:lower:]')" in
    x86_64|amd64) echo "amd64" ;;
    arm64|aarch64) echo "arm64" ;;
    *)
      echo "unsupported architecture: $(uname -m)" >&2
      exit 1
      ;;
  esac
}

verify_checksum() {
  local checksum_file="$1"
  local asset_file="$2"
  local asset_name
  asset_name="$(basename "$asset_file")"

  local expected
  expected="$(awk -v n="$asset_name" '$2==n {print $1; exit}' "$checksum_file")"
  if [[ -z "$expected" ]]; then
    echo "checksum entry not found for ${asset_name}" >&2
    exit 1
  fi

  local actual
  actual="$(sha256_file "$asset_file")"

  if [[ "${expected,,}" != "${actual,,}" ]]; then
    echo "checksum verification failed for ${asset_name}" >&2
    echo "expected: ${expected}" >&2
    echo "actual:   ${actual}" >&2
    exit 1
  fi
}

require_cmd curl
require_cmd tar
require_cmd awk

version="$(normalize_version "$version")"
if [[ -z "$version" ]]; then
  version="$(detect_latest_version)"
fi

os="$(detect_os)"
arch="$(detect_arch)"
version_nov="${version#v}"
asset_name="bpx_${version_nov}_${os}_${arch}.tar.gz"
base_url="https://github.com/${repo}/releases/download/${version}"

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

checksum_path="${tmp_dir}/checksums.txt"
asset_path="${tmp_dir}/${asset_name}"
extract_dir="${tmp_dir}/extract"

curl -fsSL "${base_url}/checksums.txt" -o "$checksum_path"
curl -fsSL "${base_url}/${asset_name}" -o "$asset_path"

verify_checksum "$checksum_path" "$asset_path"

mkdir -p "$extract_dir"
tar -xzf "$asset_path" -C "$extract_dir"

binary_path="$(find "$extract_dir" -type f -name bpx | head -n 1 || true)"
if [[ -z "$binary_path" ]]; then
  echo "failed to find bpx binary in ${asset_name}" >&2
  exit 1
fi

mkdir -p "$install_dir"
cp "$binary_path" "${install_dir}/bpx"
chmod 755 "${install_dir}/bpx"

echo "Installed: ${install_dir}/bpx"
"${install_dir}/bpx" version || true

case ":$PATH:" in
  *":${install_dir}:"*)
    ;;
  *)
    echo "Add to PATH if needed: export PATH=\"${install_dir}:\$PATH\""
    ;;
esac
