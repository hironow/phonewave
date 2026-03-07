//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestLifecycleDocker_OTelTracing(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker OTel test in short mode")
	}

	ctx := context.Background()

	// Create shared network for phonewave <-> jaeger communication
	net, err := network.New(ctx)
	if err != nil {
		t.Fatalf("create network: %v", err)
	}
	t.Cleanup(func() {
		if err := net.Remove(ctx); err != nil {
			t.Errorf("remove network: %v", err)
		}
	})
	netName := net.Name

	// Start Jaeger container
	jaegerReq := testcontainers.ContainerRequest{
		Image: "jaegertracing/jaeger:2.15.1",
		ExposedPorts: []string{
			"16686/tcp", // Jaeger UI + API
			"4318/tcp",  // OTLP HTTP
		},
		Files: []testcontainers.ContainerFile{
			{
				HostFilePath:      "../../docker/jaeger-v2-config.yaml",
				ContainerFilePath: "/etc/jaeger/config.yaml",
				FileMode:          0644,
			},
		},
		Cmd:      []string{"--config", "/etc/jaeger/config.yaml"},
		Networks: []string{netName},
		NetworkAliases: map[string][]string{
			netName: {"jaeger"},
		},
		WaitingFor: wait.ForHTTP("/").
			WithPort("16686/tcp").
			WithStartupTimeout(60 * time.Second),
	}

	jaeger, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: jaegerReq,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start jaeger: %v", err)
	}
	t.Cleanup(func() {
		if err := jaeger.Terminate(ctx); err != nil {
			t.Errorf("terminate jaeger: %v", err)
		}
	})

	// Start phonewave container on same network
	var phonewaveReq testcontainers.ContainerRequest
	if sharedImage != "" {
		phonewaveReq = testcontainers.ContainerRequest{
			Image: sharedImage,
			Cmd:   []string{"sleep", "infinity"},
			Env: map[string]string{
				"OTEL_EXPORTER_OTLP_ENDPOINT": "http://jaeger:4318",
			},
			Networks: []string{netName},
			WaitingFor: wait.ForExec([]string{"phonewave", "--version"}).
				WithStartupTimeout(30 * time.Second),
		}
	} else {
		phonewaveReq = testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    repoRoot(),
				Dockerfile: "tests/e2e/testdata/Dockerfile.test",
			},
			Env: map[string]string{
				"OTEL_EXPORTER_OTLP_ENDPOINT": "http://jaeger:4318",
			},
			Networks: []string{netName},
			WaitingFor: wait.ForExec([]string{"phonewave", "--version"}).
				WithStartupTimeout(120 * time.Second),
		}
	}

	pw, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: phonewaveReq,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start phonewave: %v", err)
	}
	t.Cleanup(func() {
		if err := pw.Terminate(ctx); err != nil {
			t.Errorf("terminate phonewave: %v", err)
		}
	})

	// Setup ecosystem and init
	repoPath := "/workspace/repo"
	setupEcosystemInContainer(t, ctx, pw, repoPath)
	execInContainer(t, ctx, pw, []string{
		"sh", "-c", fmt.Sprintf("cd /workspace && phonewave init %s", repoPath),
	})

	// Start daemon with tracing
	startDaemonInContainer(t, ctx, pw, "/workspace", "--verbose")

	// Deliver a D-Mail to generate traces
	dmailContent := "---\ndmail-schema-version: \"1\"\nname: spec-otel\nkind: specification\ndescription: OTel test\n---\n\n# OTel\n"
	heredocWrite(t, ctx, pw, repoPath+"/.siren/outbox/spec-otel.md", dmailContent)
	waitForFileInContainer(t, ctx, pw, repoPath+"/.expedition/inbox/spec-otel.md", 10*time.Second)

	// Stop daemon gracefully to flush OTel batch processor.
	// The daemon receives SIGTERM via kill, but Go's default handler
	// doesn't run defers. However, we rely on the batch processor's
	// periodic export (every 5s). Stopping the daemon first ensures
	// all in-flight deliveries complete before we query Jaeger.
	stopDaemonInContainer(t, ctx, pw, "/workspace")

	// Wait for batch processor periodic export + Jaeger ingestion
	time.Sleep(10 * time.Second)

	// Query Jaeger API for phonewave traces
	jaegerHost, err := jaeger.Host(ctx)
	if err != nil {
		t.Fatalf("get jaeger host: %v", err)
	}
	jaegerPort, err := jaeger.MappedPort(ctx, "16686/tcp")
	if err != nil {
		t.Fatalf("get jaeger port: %v", err)
	}

	// Diagnostic: list known services in Jaeger
	client := &http.Client{Timeout: 5 * time.Second}
	servicesURL := fmt.Sprintf("http://%s:%s/api/services", jaegerHost, jaegerPort.Port())
	if resp, err := client.Get(servicesURL); err == nil {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Logf("Jaeger services: %s", string(body))
	}

	// Diagnostic: dump daemon log
	code, output, _ := pw.Exec(ctx, []string{"cat", "/tmp/phonewave.log"})
	if code == 0 {
		logBytes, _ := io.ReadAll(output)
		t.Logf("Daemon log (last 2000 chars):\n%s", tailBytes(logBytes, 2000))
	}

	// Diagnostic: test network connectivity from phonewave to jaeger
	code2, output2, _ := pw.Exec(ctx, []string{"sh", "-c", "wget -q -O- http://jaeger:16686/ 2>&1 | head -c 200 || echo 'CONNECTIVITY FAILED'"})
	if code2 == 0 {
		connBytes, _ := io.ReadAll(output2)
		t.Logf("Connectivity test: %s", string(connBytes))
	}

	apiURL := fmt.Sprintf("http://%s:%s/api/traces?service=phonewave&limit=10", jaegerHost, jaegerPort.Port())

	// Poll Jaeger API with retries
	var traceFound bool
	deadline := time.After(30 * time.Second)
	for !traceFound {
		select {
		case <-deadline:
			// Final diagnostic: dump raw API response
			if resp, err := client.Get(apiURL); err == nil {
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				t.Logf("Final Jaeger API response: %s", string(body))
			}
			t.Fatal("timeout waiting for traces in Jaeger")
		default:
			resp, err := client.Get(apiURL)
			if err != nil {
				time.Sleep(2 * time.Second)
				continue
			}
			func() {
				defer resp.Body.Close()
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return
				}
				var result struct {
					Data []json.RawMessage `json:"data"`
				}
				if err := json.Unmarshal(body, &result); err != nil {
					return
				}
				if len(result.Data) > 0 {
					traceFound = true
				}
			}()
			if !traceFound {
				time.Sleep(2 * time.Second)
			}
		}
	}
}

// tailBytes returns the last n bytes of b, or all of b if len(b) <= n.
func tailBytes(b []byte, n int) []byte {
	if len(b) <= n {
		return b
	}
	return b[len(b)-n:]
}
