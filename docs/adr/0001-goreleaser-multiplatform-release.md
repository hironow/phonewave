# 0001. goreleaser Multiplatform Release Strategy

**Date:** 2026-02-23
**Status:** Accepted

## Context

phonewave must be distributed as a standalone binary for multiple platforms
(darwin/linux/windows x amd64/arm64). Manual cross-compilation and release
artifact management is error-prone and difficult to reproduce. The release
process also needs to include checksums, cosign signatures, and Homebrew tap
updates.

## Decision

Use goreleaser v2 for automated multiplatform release with the following
configuration:

1. **Target matrix**: darwin, linux, windows x amd64, arm64. CGO disabled
   (`CGO_ENABLED=0`) for static binaries.
2. **Version injection**: `ldflags` injects `Version`, `Commit`, and `Date`
   into `internal/cmd` package variables at build time.
3. **Pre-release hooks**: `go mod tidy` and `go test ./... -count=1 -short`
   run before builds to catch issues early.
4. **Archive format**: `.tar.gz` for Unix platforms, `.zip` for Windows.
5. **Integrity**: SHA-256 checksums (`checksums.txt`) and cosign blob
   signatures for supply chain security.
6. **Homebrew distribution**: Automatic Homebrew tap update via
   `hironow/homebrew-tap` repository with token-based authentication.
7. **Changelog filtering**: `docs:`, `test:`, `ci:`, `chore:` prefixes are
   excluded from generated changelogs to keep release notes focused.

## Consequences

### Positive

- Reproducible, automated releases across 6 platform combinations
- Supply chain security via cosign signatures and SHA-256 checksums
- Users can install via `brew install hironow/tap/phonewave`
- Version information is embedded at build time, not hardcoded

### Negative

- goreleaser v2 configuration must be maintained alongside Go version upgrades
- cosign signing requires key management in CI environment
- Homebrew tap token must be kept in sync with CI secrets
