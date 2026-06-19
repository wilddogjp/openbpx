#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Install openbpx (.deb) from GitHub Releases.

Usage:
  bash ./scripts/install-bpx-from-release.sh

Environment overrides:
  OPENBPX_RELEASE_REPO  GitHub repo in owner/name form (default: wilddogjp/openbpx)
  OPENBPX_TAG           Release tag to install (for example: v0.2.0)
  OPENBPX_ARCH          Target architecture (amd64 or arm64)
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

if [[ $# -ne 0 ]]; then
  echo "error: this script does not accept positional arguments" >&2
  usage >&2
  exit 2
fi

need_cmd() {
  local cmd="$1"
  if ! command -v "${cmd}" >/dev/null 2>&1; then
    echo "error: required command not found: ${cmd}" >&2
    exit 1
  fi
}

normalize_arch() {
  local raw="$1"
  case "${raw}" in
    amd64|x86_64)
      echo "amd64"
      ;;
    arm64|aarch64)
      echo "arm64"
      ;;
    *)
      echo "error: unsupported architecture: ${raw}" >&2
      echo "supported values: amd64, arm64" >&2
      exit 1
      ;;
  esac
}

detect_arch() {
  if [[ -n "${OPENBPX_ARCH:-}" ]]; then
    normalize_arch "${OPENBPX_ARCH}"
    return
  fi

  if command -v dpkg >/dev/null 2>&1; then
    normalize_arch "$(dpkg --print-architecture)"
    return
  fi

  if command -v uname >/dev/null 2>&1; then
    normalize_arch "$(uname -m)"
    return
  fi

  echo "error: failed to detect architecture" >&2
  exit 1
}

resolve_tag() {
  local repo="$1"
  local tag="${OPENBPX_TAG:-}"

  if [[ -n "${tag}" ]]; then
    if [[ "${tag}" != v* ]]; then
      echo "v${tag}"
    else
      echo "${tag}"
    fi
    return
  fi

  basename "$(curl -fsSL -o /dev/null -w '%{url_effective}' "https://github.com/${repo}/releases/latest")"
}

install_deb() {
  local deb_path="$1"

  if [[ "$(id -u)" -eq 0 ]]; then
    dpkg -i "${deb_path}"
    return
  fi

  if command -v sudo >/dev/null 2>&1; then
    sudo dpkg -i "${deb_path}"
    return
  fi

  echo "error: root privileges are required to install the package" >&2
  echo "run as root or install sudo" >&2
  exit 1
}

need_cmd curl
need_cmd dpkg
need_cmd sha256sum
need_cmd awk
need_cmd mktemp

REPO="${OPENBPX_RELEASE_REPO:-wilddogjp/openbpx}"
ARCH="$(detect_arch)"
TAG="$(resolve_tag "${REPO}")"
VER="${TAG#v}"
PKG_FILE="openbpx_${VER}_${ARCH}.deb"
BASE_URL="https://github.com/${REPO}/releases/download/${TAG}"
TMP_DIR="$(mktemp -d)"
CHECKSUMS_FILE="${TMP_DIR}/checksums.txt"
PKG_PATH="${TMP_DIR}/${PKG_FILE}"

cleanup() {
  rm -rf "${TMP_DIR}"
}
trap cleanup EXIT

echo "Installing openbpx"
echo "  repo: ${REPO}"
echo "  tag:  ${TAG}"
echo "  arch: ${ARCH}"

curl -fsSL -o "${CHECKSUMS_FILE}" "${BASE_URL}/checksums.txt"
curl -fsSL -o "${PKG_PATH}" "${BASE_URL}/${PKG_FILE}"

expected_sha="$(
  awk -v file="${PKG_FILE}" '$2 == file || $2 == ("*" file) { print $1; exit }' "${CHECKSUMS_FILE}"
)"

if [[ -z "${expected_sha}" ]]; then
  echo "error: checksum entry not found for ${PKG_FILE}" >&2
  exit 1
fi

actual_sha="$(sha256sum "${PKG_PATH}" | awk '{ print $1 }')"
if [[ "${actual_sha}" != "${expected_sha}" ]]; then
  echo "error: checksum mismatch for ${PKG_FILE}" >&2
  echo "expected: ${expected_sha}" >&2
  echo "actual:   ${actual_sha}" >&2
  exit 1
fi

install_deb "${PKG_PATH}"

if command -v bpx >/dev/null 2>&1; then
  echo "Installed: $(bpx version)"
else
  echo "openbpx installed, but bpx was not found on PATH" >&2
fi
