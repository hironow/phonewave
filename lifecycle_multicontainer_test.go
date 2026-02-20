//go:build docker

package phonewave

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestLifecycleDocker_MultiContainer(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker multi-container test in short mode")
	}

	ctx := context.Background()
	repoPath := "/shared/repo"
	volumeName := fmt.Sprintf("phonewave-test-%d", time.Now().UnixNano())

	// =====================================================================
	// Phase 1: Create shared Docker volume
	// =====================================================================
	dockerCli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatalf("docker client: %v", err)
	}
	defer dockerCli.Close()

	vol, err := dockerCli.VolumeCreate(ctx, volume.CreateOptions{Name: volumeName})
	if err != nil {
		t.Fatalf("create volume: %v", err)
	}
	defer func() {
		if err := dockerCli.VolumeRemove(ctx, vol.Name, true); err != nil {
			t.Errorf("remove volume: %v", err)
		}
	}()

	// =====================================================================
	// Phase 2: Start daemon container (phonewave binary + shared volume)
	// =====================================================================
	daemonReq := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    ".",
			Dockerfile: "testdata/Dockerfile.test",
		},
		HostConfigModifier: func(hc *container.HostConfig) {
			hc.Mounts = []mount.Mount{
				{
					Type:   mount.TypeVolume,
					Source: volumeName,
					Target: "/shared",
				},
			}
		},
		WaitingFor: wait.ForExec([]string{"phonewave", "--version"}).
			WithStartupTimeout(120 * time.Second),
	}

	daemonContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: daemonReq,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start daemon container: %v", err)
	}
	defer func() {
		if err := daemonContainer.Terminate(ctx); err != nil {
			t.Errorf("terminate daemon container: %v", err)
		}
	}()

	// =====================================================================
	// Phase 3: Start writer container (alpine + shared volume)
	// =====================================================================
	writerReq := testcontainers.ContainerRequest{
		Image: "alpine:latest",
		Cmd:   []string{"sleep", "infinity"},
		HostConfigModifier: func(hc *container.HostConfig) {
			hc.Mounts = []mount.Mount{
				{
					Type:   mount.TypeVolume,
					Source: volumeName,
					Target: "/shared",
				},
			}
		},
		WaitingFor: wait.ForExec([]string{"true"}).
			WithStartupTimeout(30 * time.Second),
	}

	writerContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: writerReq,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start writer container: %v", err)
	}
	defer func() {
		if err := writerContainer.Terminate(ctx); err != nil {
			t.Errorf("terminate writer container: %v", err)
		}
	}()

	// =====================================================================
	// Phase 4: Setup ecosystem on shared volume (via daemon container)
	// =====================================================================
	setupEcosystemInContainer(t, ctx, daemonContainer, repoPath)

	// =====================================================================
	// Phase 5: Run phonewave init + start daemon
	// =====================================================================
	execInContainer(t, ctx, daemonContainer, []string{
		"sh", "-c", fmt.Sprintf("cd /shared && phonewave init %s", repoPath),
	})

	if !fileExistsInContainer(t, ctx, daemonContainer, "/shared/phonewave.yaml") {
		t.Fatal("phonewave.yaml not created")
	}

	// Start daemon
	execInContainer(t, ctx, daemonContainer, []string{
		"sh", "-c", "cd /shared && nohup phonewave run --verbose > /tmp/phonewave.log 2>&1 &",
	})
	waitForFileInContainer(t, ctx, daemonContainer, "/shared/.phonewave/watch.pid", 15*time.Second)

	// =====================================================================
	// Phase 6: Writer container writes D-Mail to shared volume
	// =====================================================================
	specContent := "---\nname: spec-cross\nkind: specification\ndescription: Cross-container spec\n---\n\n# Cross-container\n"
	heredocWrite(t, ctx, writerContainer, repoPath+"/.siren/outbox/spec-cross.md", specContent)

	// =====================================================================
	// Phase 7: Verify daemon container detected and delivered the file
	// =====================================================================
	waitForFileInContainer(t, ctx, daemonContainer,
		repoPath+"/.expedition/inbox/spec-cross.md", 15*time.Second)
	waitForFileAbsentInContainer(t, ctx, daemonContainer,
		repoPath+"/.siren/outbox/spec-cross.md", 15*time.Second)

	// Writer can also see the delivered file on the shared volume
	waitForFileInContainer(t, ctx, writerContainer,
		repoPath+"/.expedition/inbox/spec-cross.md", 5*time.Second)

	// =====================================================================
	// Phase 8: Writer sends feedback (multi-target: siren + expedition)
	// =====================================================================
	fbContent := "---\nname: fb-cross\nkind: feedback\ndescription: Cross-container feedback\n---\n\n# Feedback\n"
	heredocWrite(t, ctx, writerContainer, repoPath+"/.gate/outbox/fb-cross.md", fbContent)

	waitForFileInContainer(t, ctx, daemonContainer,
		repoPath+"/.siren/inbox/fb-cross.md", 15*time.Second)
	waitForFileInContainer(t, ctx, daemonContainer,
		repoPath+"/.expedition/inbox/fb-cross.md", 15*time.Second)

	// =====================================================================
	// Phase 9: Verify delivery log from daemon container
	// =====================================================================
	_, logOutput := execInContainerNoFail(t, ctx, daemonContainer,
		[]string{"cat", "/shared/.phonewave/delivery.log"})

	if !strings.Contains(logOutput, "DELIVERED") {
		t.Error("delivery log missing DELIVERED entries")
	}
	if !strings.Contains(logOutput, "kind=specification") {
		t.Error("delivery log missing kind=specification")
	}
	if !strings.Contains(logOutput, "kind=feedback") {
		t.Error("delivery log missing kind=feedback")
	}

	// =====================================================================
	// Phase 10: Stop daemon
	// =====================================================================
	execInContainer(t, ctx, daemonContainer, []string{
		"sh", "-c", "kill $(cat /shared/.phonewave/watch.pid)",
	})
	waitForFileAbsentInContainer(t, ctx, daemonContainer,
		"/shared/.phonewave/watch.pid", 10*time.Second)
}
