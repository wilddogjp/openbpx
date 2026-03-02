# Contributing to BPX

Thanks for your interest in contributing to `bpx`.

## Before You Start

- For large changes, open an Issue first and align scope.
- Keep one concern per PR.
- Treat Unreal Engine source as behavioral reference only.
- Do not copy Unreal Engine source text into this repository.

## Development Setup

```bash
git clone https://github.com/wilddogjp/bpx.git
cd bpx
go mod download
```

## Required Local Checks

Run these before opening or updating a PR:

```bash
gofmt -l .
go vet ./...
staticcheck ./...
go test ./...
```

Notes:

- `gofmt -l .` must produce no output.
- If `staticcheck` is not installed, install it with:
  - `go install honnef.co/go/tools/cmd/staticcheck@latest`

## PR Requirements

Each PR description should include:

- What changed
- Why it changed
- How it was tested

When changing CLI behavior (flags/output/contract):

- Update [docs/commands.md](docs/commands.md)
- State compatibility impact clearly
- Use `BREAKING CHANGE` in the PR title when applicable

## Commit Conventions

Use Conventional Commit prefixes:

- `feat:`
- `fix:`
- `docs:`
- `test:`
- `refactor:`
- `ci:`
- `chore:`

## Testing and Fixtures

- Use existing fixtures under `testdata/`.
- For parser/rewrite safety changes, add focused tests near the modified package.
- If test coverage is not feasible, document the gap and manual validation performed.

## Security

Please do not report vulnerabilities in public issues.
See [SECURITY.md](SECURITY.md) for private reporting instructions.
