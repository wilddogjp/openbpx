# Single-Binary Distribution Build Guide

This document describes how to build `bpx` as standalone executables for Windows, macOS, and Linux (`CGO_ENABLED=0` cross-builds).

## Target Artifacts

- `bpx_linux_amd64`
- `bpx_linux_arm64`
- `bpx_darwin_amd64`
- `bpx_darwin_arm64`
- `bpx_windows_amd64.exe`
- `bpx_windows_arm64.exe`

## Local Build (Manual)

```bash
cd bpx
mkdir -p dist/single

CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o dist/single/bpx_linux_amd64 ./cmd/bpx
CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o dist/single/bpx_linux_arm64 ./cmd/bpx
CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o dist/single/bpx_darwin_amd64 ./cmd/bpx
CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o dist/single/bpx_darwin_arm64 ./cmd/bpx
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o dist/single/bpx_windows_amd64.exe ./cmd/bpx
CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o dist/single/bpx_windows_arm64.exe ./cmd/bpx
```

If needed, generate checksums:

```bash
cd bpx
sha256sum dist/single/* > dist/single/checksums.txt
```

## GitHub Actions (Automatic on `main` push)

`.github/workflows/build-artifacts-main.yml` performs:

1. Trigger on `main` push
2. `go build` across the OS/arch matrix
3. Upload generated binaries and `.sha256` files as artifacts

Notes:

- Artifacts are uploaded as raw binaries (`bpx_*`), not pre-zipped by this workflow.
- Artifact storage format follows GitHub Actions behavior.
