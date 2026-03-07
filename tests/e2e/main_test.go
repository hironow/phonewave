//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// sharedImage holds the Docker image name built once by TestMain.
// Docker-based tests use this instead of rebuilding per-test.
var sharedImage string

func TestMain(m *testing.M) {
	// Build the Docker image once before all tests.
	// This avoids the ~20-30s rebuild per test that causes CI timeouts.
	ctx := context.Background()

	req := testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    repoRoot(),
				Dockerfile: "tests/e2e/testdata/Dockerfile.test",
				PrintBuildLog: true,
			},
			WaitingFor: wait.ForExec([]string{"phonewave", "--version"}).
				WithStartupTimeout(120 * time.Second),
		},
		Started: true,
	}

	// Build and start a throwaway container to cache the image.
	c, err := testcontainers.GenericContainer(ctx, req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: failed to build shared image: %v\n", err)
		os.Exit(1)
	}

	// Get the image name from the container for reuse.
	inspect, err := c.Inspect(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: failed to inspect container: %v\n", err)
		c.Terminate(ctx)
		os.Exit(1)
	}
	sharedImage = inspect.Image

	// Terminate the bootstrap container — tests create their own.
	c.Terminate(ctx)

	os.Exit(m.Run())
}

// repoRoot returns the phonewave repository root relative to the test working directory.
// go test runs with CWD = package dir (tests/e2e/), so repo root is "../..".
func repoRoot() string {
	return "../.."
}

// phonewave returns the path to the phonewave binary.
// In CI, it's installed to /usr/local/bin; locally, use `which`.
func phonewaveBin() string {
	if p, err := exec.LookPath("phonewave"); err == nil {
		return p
	}
	return "phonewave"
}
