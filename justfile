# phonewave — task runner
# https://just.systems

set shell := ["bash", "-eu", "-o", "pipefail", "-c"]

# Default: show help
default: help

# Help: list available recipes
help:
    @just --list --unsorted

# Define specific commands
MARKDOWNLINT := "bunx markdownlint-cli2"

# Install prek hooks (pre-commit + pre-push) with quiet mode
prek-install:
    prek install -t pre-commit -t pre-push --overwrite
    @sed 's/-- "\$@"/--quiet -- "\$@"/' .git/hooks/pre-commit > .git/hooks/pre-commit.tmp && mv .git/hooks/pre-commit.tmp .git/hooks/pre-commit && chmod +x .git/hooks/pre-commit
    @sed 's/-- "\$@"/--quiet -- "\$@"/' .git/hooks/pre-push > .git/hooks/pre-push.tmp && mv .git/hooks/pre-push.tmp .git/hooks/pre-push && chmod +x .git/hooks/pre-push
    @echo "prek hooks installed (quiet mode)"

# Run all prek hooks on all files
prek-run:
    prek run --all-files

# Lint markdown files
lint-md:
    @{{MARKDOWNLINT}} --fix "*.md" "docs/**/*.md"

# Version from git tags
VERSION := `git describe --tags --always --dirty 2>/dev/null || echo "dev"`

# Build the binary with version info
build:
    go build -ldflags "-s -w -X github.com/hironow/phonewave/internal/cmd.Version={{VERSION}}" -o phonewave ./cmd/phonewave/

# Build and install to /usr/local/bin
install: build
    mv phonewave /usr/local/bin/

# Run all tests
test:
    go test ./... -count=1 -timeout=300s

# Run tests with verbose output
test-v:
    go test ./... -count=1 -timeout=300s -v

# Run tests with race detector
test-race:
    go test ./... -race -count=1 -timeout=300s

# Run tests with coverage report
cover:
    go test ./... -coverprofile=coverage.out -count=1 -timeout=300s
    go tool cover -func=coverage.out

# Open coverage in browser
cover-html: cover
    go tool cover -html=coverage.out

# Format code
fmt:
    gofmt -w .

# Run go vet
vet:
    go vet ./...

# Run semgrep rules
semgrep:
    semgrep scan --config .semgrep/ --error --severity ERROR .

# Lint (fmt check + vet + markdown lint)
lint: vet lint-md
    @gofmt -l . | grep . && echo "gofmt: files need formatting" && exit 1 || true

# Format, vet, test — full check before commit
check: fmt vet test

# Run phonewave doctor (quick smoke test after build)
doctor: build
    ./phonewave doctor

# Run Docker lifecycle tests (requires Docker)
test-docker:
    go test ./... -tags=docker -count=1 -timeout=600s -v -run TestLifecycleDocker

# Run all tests including Docker tests
test-all: test test-docker

# Start Jaeger v2 (OTel trace viewer + MCP) on http://localhost:16686
jaeger:
    docker compose -f docker/compose.yaml up -d
    @echo "Jaeger UI:      http://localhost:16686"
    @echo "OTLP endpoint:  http://localhost:4318"
    @echo "MCP endpoint:   http://localhost:16687/mcp"
    @echo ""
    @echo "Run phonewave with tracing:"
    @echo "  OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318 phonewave ./your-repo"

# Stop Jaeger
jaeger-down:
    docker compose -f docker/compose.yaml down

# Generate CLI markdown docs (for LLM consumption)
docgen:
    go run ./internal/tools/docgen/

# Snapshot GoReleaser build (no publish)
release-snapshot:
    goreleaser release --snapshot --clean

# Clean build artifacts
clean:
    rm -f phonewave coverage.out
    rm -rf dist/
    go clean
