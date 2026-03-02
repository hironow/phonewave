//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// buildCrossToolContainer creates and starts a container with all 4 tool binaries.
// Context is tap/ (3 levels up from tests/e2e/ working dir).
// Dockerfile path is relative to the context root.
func buildCrossToolContainer(t *testing.T, ctx context.Context) testcontainers.Container {
	t.Helper()
	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    "../../..",
			Dockerfile: "phonewave/tests/e2e/Dockerfile.cross-e2e",
		},
		WaitingFor: wait.ForExec([]string{"phonewave", "--version"}).
			WithStartupTimeout(180 * time.Second),
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start cross-tool container: %v", err)
	}
	t.Cleanup(func() {
		if err := c.Terminate(ctx); err != nil {
			t.Errorf("terminate container: %v", err)
		}
	})
	return c
}

// loadGoldenFile reads a golden D-Mail file from testdata/cross-tool/.
func loadGoldenFile(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "cross-tool", name))
	if err != nil {
		t.Fatalf("read golden file %s: %v", name, err)
	}
	return string(data)
}

func TestCrossTool_AllBinariesExist(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker test in short mode")
	}
	ctx := context.Background()
	c := buildCrossToolContainer(t, ctx)

	for _, tool := range []string{"phonewave", "sightjack", "paintress", "amadeus"} {
		output := execInContainer(t, ctx, c, []string{tool, "--version"})
		if !strings.Contains(output, tool) {
			t.Errorf("%s --version: unexpected output: %s", tool, output)
		}
	}
}

func TestCrossTool_RouteAllKinds(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker test in short mode")
	}
	ctx := context.Background()
	c := buildCrossToolContainer(t, ctx)
	repoPath := "/workspace/repo"

	// Setup 3-tool ecosystem
	setupEcosystemInContainer(t, ctx, c, repoPath)

	// Init phonewave
	execInContainer(t, ctx, c, []string{
		"sh", "-c", fmt.Sprintf("cd /workspace && phonewave init %s", repoPath),
	})

	// Start daemon
	startDaemonInContainer(t, ctx, c, "/workspace", "--verbose")
	defer stopDaemonInContainer(t, ctx, c, "/workspace")

	// Route table:
	// specification: .siren/outbox -> .expedition/inbox
	// report:        .expedition/outbox -> .gate/inbox
	// feedback:      .gate/outbox -> .siren/inbox + .expedition/inbox

	tests := []struct {
		name       string
		goldenFile string
		outbox     string
		inboxes    []string
	}{
		{
			name:       "specification routes siren to expedition",
			goldenFile: "sj-specification.md",
			outbox:     repoPath + "/.siren/outbox",
			inboxes:    []string{repoPath + "/.expedition/inbox"},
		},
		{
			name:       "report routes expedition to gate",
			goldenFile: "pt-report.md",
			outbox:     repoPath + "/.expedition/outbox",
			inboxes:    []string{repoPath + "/.gate/inbox"},
		},
		{
			name:       "feedback routes gate to siren and expedition",
			goldenFile: "am-feedback.md",
			outbox:     repoPath + "/.gate/outbox",
			inboxes: []string{
				repoPath + "/.siren/inbox",
				repoPath + "/.expedition/inbox",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := loadGoldenFile(t, tt.goldenFile)
			fileName := tt.goldenFile
			heredocWrite(t, ctx, c, tt.outbox+"/"+fileName, content)

			// Verify delivery to all expected inboxes
			for _, inbox := range tt.inboxes {
				waitForFileInContainer(t, ctx, c, inbox+"/"+fileName, 15*time.Second)
				// Verify content integrity
				delivered := readFileInContainer(t, ctx, c, inbox+"/"+fileName)
				if !strings.Contains(delivered, "dmail-schema-version") {
					t.Errorf("delivered file missing frontmatter in %s", inbox)
				}
			}

			// Verify source removed from outbox
			waitForFileAbsentInContainer(t, ctx, c, tt.outbox+"/"+fileName, 10*time.Second)
		})
	}
}

func TestCrossTool_Idempotency(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker test in short mode")
	}
	ctx := context.Background()
	c := buildCrossToolContainer(t, ctx)
	repoPath := "/workspace/repo"

	setupEcosystemInContainer(t, ctx, c, repoPath)
	execInContainer(t, ctx, c, []string{
		"sh", "-c", fmt.Sprintf("cd /workspace && phonewave init %s", repoPath),
	})
	startDaemonInContainer(t, ctx, c, "/workspace", "--verbose")
	defer stopDaemonInContainer(t, ctx, c, "/workspace")

	content := loadGoldenFile(t, "sj-specification.md")

	// First delivery
	heredocWrite(t, ctx, c, repoPath+"/.siren/outbox/idem-test.md", content)
	waitForFileInContainer(t, ctx, c, repoPath+"/.expedition/inbox/idem-test.md", 15*time.Second)
	waitForFileAbsentInContainer(t, ctx, c, repoPath+"/.siren/outbox/idem-test.md", 10*time.Second)

	// Second delivery of same content (different file name to trigger processing)
	heredocWrite(t, ctx, c, repoPath+"/.siren/outbox/idem-test-2.md", content)
	waitForFileInContainer(t, ctx, c, repoPath+"/.expedition/inbox/idem-test-2.md", 15*time.Second)

	// Both files should exist in inbox (phonewave delivers by filename, not content)
	if !fileExistsInContainer(t, ctx, c, repoPath+"/.expedition/inbox/idem-test.md") {
		t.Error("first delivery should still be in inbox")
	}
	if !fileExistsInContainer(t, ctx, c, repoPath+"/.expedition/inbox/idem-test-2.md") {
		t.Error("second delivery should be in inbox")
	}
}

func TestCrossTool_DeliveryLog(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker test in short mode")
	}
	ctx := context.Background()
	c := buildCrossToolContainer(t, ctx)
	repoPath := "/workspace/repo"

	setupEcosystemInContainer(t, ctx, c, repoPath)
	execInContainer(t, ctx, c, []string{
		"sh", "-c", fmt.Sprintf("cd /workspace && phonewave init %s", repoPath),
	})
	startDaemonInContainer(t, ctx, c, "/workspace", "--verbose")
	defer stopDaemonInContainer(t, ctx, c, "/workspace")

	// Deliver all golden files
	for _, gf := range []struct {
		file, outbox string
	}{
		{"sj-specification.md", repoPath + "/.siren/outbox"},
		{"pt-report.md", repoPath + "/.expedition/outbox"},
		{"am-feedback.md", repoPath + "/.gate/outbox"},
	} {
		content := loadGoldenFile(t, gf.file)
		heredocWrite(t, ctx, c, gf.outbox+"/"+gf.file, content)
	}

	// Wait for all deliveries
	time.Sleep(3 * time.Second)

	// Verify delivery log contains all kinds
	logContent := readFileInContainer(t, ctx, c, "/workspace/.phonewave/delivery.log")
	for _, kind := range []string{"specification", "report", "feedback"} {
		if !strings.Contains(logContent, "kind="+kind) {
			t.Errorf("delivery log missing kind=%s", kind)
		}
	}
	if !strings.Contains(logContent, "DELIVERED") {
		t.Error("delivery log missing DELIVERED entries")
	}
}
