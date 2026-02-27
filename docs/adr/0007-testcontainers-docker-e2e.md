# 0007. testcontainers-go Docker E2E Testing Strategy

**Date:** 2026-02-23
**Status:** Accepted

## Context

phonewave's core functionality (file watching, D-Mail delivery, multi-endpoint
routing) depends on file system events and process lifecycle management. Unit
tests with temporary directories verify logic but cannot reproduce the full
daemon lifecycle: startup scan, runtime delivery via fsnotify, PID file
management, and graceful shutdown.

Additionally, multi-container scenarios (shared Docker volumes between daemon
and writer processes) are essential for validating cross-process D-Mail delivery
— the primary production use case.

## Decision

Use testcontainers-go for Docker-based E2E tests with the following strategy:

1. **Build tag isolation**: Docker tests use `//go:build docker` to prevent
   execution in standard `go test` runs. Activated via `just test-docker`.
2. **Multi-stage Dockerfile**: `testdata/Dockerfile.test` uses a builder stage
   (golang:bookworm) and a slim runtime stage (debian:bookworm-slim) with only
   the phonewave binary.
3. **Single-container test**: Validates the full daemon lifecycle within one
   container — init, startup scan, runtime delivery, multi-target routing,
   delivery log, and graceful shutdown.
4. **Multi-container test**: Uses a shared Docker volume between a daemon
   container and a writer container (alpine) to verify cross-process D-Mail
   delivery via shared file system.
5. **OTel tracing test**: Uses `docker/compose-e2e.yaml` with Jaeger v2 to
   verify end-to-end trace propagation from daemon operations to the
   telemetry backend.
6. **Helper functions**: Shared test helpers (`execInContainer`,
   `waitForFileInContainer`, `heredocWrite`, etc.) provide a consistent API
   for container interactions across all Docker tests.

## Consequences

### Positive

- Full daemon lifecycle validation in an isolated, reproducible environment
- Cross-process delivery verification via shared Docker volumes
- OTel integration testing with real Jaeger backend
- Build tag isolation prevents Docker tests from slowing standard test runs

### Negative

- Docker must be available on the test machine (CI and local)
- Container image build adds latency to first test run (~120s timeout)
- testcontainers-go dependency is test-only but adds to go.sum
