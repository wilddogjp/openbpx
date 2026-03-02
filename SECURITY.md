# Security Policy

If you believe you found a security issue in BPX, please report it privately.

## Reporting a Vulnerability

Use GitHub's private vulnerability reporting for this repository:

- Go to the repository's `Security` tab
- Click `Report a vulnerability`
- Submit a private advisory with reproduction details

Do not open public issues for unpatched vulnerabilities.

## What to Include

Please include:

1. Affected version or commit SHA
2. Impact summary
3. Reproduction steps (minimal PoC)
4. Expected vs actual behavior
5. Suggested mitigation (if available)

Reports with clear reproduction and impact details are triaged first.

## Scope Notes

BPX parses and rewrites binary asset data. Security-sensitive areas include:

- untrusted input parsing boundaries
- offset/size arithmetic safety
- path handling and file overwrite behavior
- malformed input that could trigger crashes or corruption

## Supported Versions

Current support focus is `main` and the latest release line (`v0.x`).
When possible, fixes are applied to `main` first and included in the next release.
