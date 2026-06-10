package session_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hironow/phonewave/internal/session"
)

func TestMCPServer_ListsAllPhase2dTools(t *testing.T) {
	// given
	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n")
	var out bytes.Buffer
	srv := session.NewMCPServer(in, &out, nil)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then: all 3 Phase 2d tools advertised, with stable names
	var resp map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &resp); err != nil {
		t.Fatalf("decode response: %v (raw=%q)", err, out.String())
	}
	if resp["jsonrpc"] != "2.0" {
		t.Errorf("jsonrpc = %v, want 2.0", resp["jsonrpc"])
	}
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("missing result: %v", resp)
	}
	tools, ok := result["tools"].([]any)
	if !ok {
		t.Fatalf("tools list missing: %v", result["tools"])
	}
	want := map[string]bool{
		"ping":          false,
		"outbox_status": false,
		"inbox_status":  false,
	}
	for _, t0 := range tools {
		entry, _ := t0.(map[string]any)
		if name, _ := entry["name"].(string); name != "" {
			want[name] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("missing Phase 2d tool: %s", name)
		}
	}
}

func TestMCPServer_CallsPingTool(t *testing.T) {
	// given
	in := strings.NewReader(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"ping","arguments":{}}}` + "\n")
	var out bytes.Buffer
	srv := session.NewMCPServer(in, &out, nil)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then
	var resp map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &resp); err != nil {
		t.Fatalf("decode response: %v (raw=%q)", err, out.String())
	}
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("missing result: %v", resp)
	}
	content, ok := result["content"].([]any)
	if !ok || len(content) != 1 {
		t.Fatalf("content list mismatch: %v", result["content"])
	}
	first, _ := content[0].(map[string]any)
	if first["text"] != "pong" {
		t.Errorf("text = %v, want pong", first["text"])
	}
}

func TestMCPServer_RejectsUnknownTool(t *testing.T) {
	// given
	in := strings.NewReader(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"phonewave.does_not_exist","arguments":{}}}` + "\n")
	var out bytes.Buffer
	srv := session.NewMCPServer(in, &out, nil)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then
	var resp map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &resp); err != nil {
		t.Fatalf("decode response: %v (raw=%q)", err, out.String())
	}
	rpcErr, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error, got %v", resp)
	}
	if code, _ := rpcErr["code"].(float64); int(code) != -32601 {
		t.Errorf("error code = %v, want -32601", rpcErr["code"])
	}
}

func TestMCPServer_OutboxStatus_UninitializedConfigPath(t *testing.T) {
	// given: NewMCPServer without WithConfigPath.
	in := strings.NewReader(`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"outbox_status","arguments":{"tool":"paintress"}}}` + "\n")
	var out bytes.Buffer
	srv := session.NewMCPServer(in, &out, nil)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then
	body := decodeFirstText(t, &out)
	if body["initialized"] != false {
		t.Errorf("initialized = %v, want false (empty configPath)", body["initialized"])
	}
}

func TestMCPServer_OutboxStatus_RealImpl_EmptyConfig(t *testing.T) {
	// given: temp dir with empty config.yaml.
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("repositories: []\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	in := strings.NewReader(`{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"outbox_status","arguments":{"tool":"paintress"}}}` + "\n")
	var out bytes.Buffer
	srv := session.NewMCPServer(in, &out, nil).WithConfigPath(configPath)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then: Phase 4 fields present + zero values for empty config.
	body := decodeFirstText(t, &out)
	if body["initialized"] != true {
		t.Errorf("initialized = %v, want true (body=%v)", body["initialized"], body)
	}
	if body["tool"] != "paintress" {
		t.Errorf("tool = %v, want paintress", body["tool"])
	}
	if got, _ := body["total_depth"].(float64); int(got) != 0 {
		t.Errorf("total_depth = %v, want 0 (empty config)", body["total_depth"])
	}
	if got, _ := body["dead_letter_count"].(float64); int(got) != 0 {
		t.Errorf("dead_letter_count = %v, want 0 (no delivery.db yet)", body["dead_letter_count"])
	}
	if got, _ := body["oldest_age_seconds"].(float64); int(got) != 0 {
		t.Errorf("oldest_age_seconds = %v, want 0 (empty outbox)", body["oldest_age_seconds"])
	}
}

func TestMCPServer_OutboxStatus_Phase4_OldestAgeFromFile(t *testing.T) {
	// given: temp dir with config + one repo with an outbox file (mtime
	// adjusted to 60 seconds ago).
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	repoDir := filepath.Join(tmpDir, "repo")
	endpointDir := filepath.Join(repoDir, "ep1")
	outboxDir := filepath.Join(endpointDir, "outbox")
	if err := os.MkdirAll(outboxDir, 0o755); err != nil {
		t.Fatalf("mkdir outbox: %v", err)
	}
	msg := filepath.Join(outboxDir, "msg-1.yaml")
	if err := os.WriteFile(msg, []byte("message_id: m1\n"), 0o644); err != nil {
		t.Fatalf("write msg: %v", err)
	}
	pastTime := time.Now().Add(-60 * time.Second)
	if err := os.Chtimes(msg, pastTime, pastTime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	cfgYAML := "repositories:\n  - path: " + repoDir + "\n    endpoints:\n      - dir: ep1\n        produces: [test]\n"
	if err := os.WriteFile(configPath, []byte(cfgYAML), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	in := strings.NewReader(`{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"outbox_status","arguments":{}}}` + "\n")
	var out bytes.Buffer
	srv := session.NewMCPServer(in, &out, nil).WithConfigPath(configPath)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then
	body := decodeFirstText(t, &out)
	if got, _ := body["total_depth"].(float64); int(got) != 1 {
		t.Errorf("total_depth = %v, want 1 (one msg-1.yaml)", body["total_depth"])
	}
	// oldest_age_seconds should be >= 60 (allow drift for slow test runs).
	if got, _ := body["oldest_age_seconds"].(float64); int(got) < 60 {
		t.Errorf("oldest_age_seconds = %v, want >= 60", body["oldest_age_seconds"])
	}
}

func TestMCPServer_InboxStatus_RealImpl_EmptyConfig(t *testing.T) {
	// given: temp dir with empty config.yaml.
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("repositories: []\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	in := strings.NewReader(`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"inbox_status","arguments":{"tool":"sightjack"}}}` + "\n")
	var out bytes.Buffer
	srv := session.NewMCPServer(in, &out, nil).WithConfigPath(configPath)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then
	body := decodeFirstText(t, &out)
	if body["initialized"] != true {
		t.Errorf("initialized = %v, want true", body["initialized"])
	}
	if body["tool"] != "sightjack" {
		t.Errorf("tool = %v, want sightjack", body["tool"])
	}
	if got, _ := body["total_depth"].(float64); int(got) != 0 {
		t.Errorf("total_depth = %v, want 0", body["total_depth"])
	}
}

// decodeFirstText extracts the JSON payload from the first content
// item of the MCP tools/call response. Stub responses ship a single
// JSON-string text entry so the body is a JSON object inside a string.
func decodeFirstText(t *testing.T, out *bytes.Buffer) map[string]any {
	t.Helper()
	var resp map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &resp); err != nil {
		t.Fatalf("decode response: %v (raw=%q)", err, out.String())
	}
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("missing result: %v", resp)
	}
	content, ok := result["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("missing content: %v", result)
	}
	first, _ := content[0].(map[string]any)
	text, _ := first["text"].(string)
	var body map[string]any
	if err := json.Unmarshal([]byte(text), &body); err != nil {
		t.Fatalf("decode inner JSON: %v (raw=%q)", err, text)
	}
	return body
}

func TestMCPServer_RejectsUnknownMethod(t *testing.T) {
	// given
	in := strings.NewReader(`{"jsonrpc":"2.0","id":4,"method":"completion/complete"}` + "\n")
	var out bytes.Buffer
	srv := session.NewMCPServer(in, &out, nil)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then
	var resp map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &resp); err != nil {
		t.Fatalf("decode response: %v (raw=%q)", err, out.String())
	}
	rpcErr, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error, got %v", resp)
	}
	if code, _ := rpcErr["code"].(float64); int(code) != -32601 {
		t.Errorf("error code = %v, want -32601", rpcErr["code"])
	}
}

func TestMCPServer_Initialize_Handshake(t *testing.T) {
	// given: client sends initialize with a different protocol version
	in := strings.NewReader(`{"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"claude-code","version":"1.0"}}}` + "\n")
	var out bytes.Buffer
	srv := session.NewMCPServer(in, &out, nil)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then: server returns ITS supported version (not an echo), + tools cap + serverInfo
	var resp struct {
		Result struct {
			ProtocolVersion string                     `json:"protocolVersion"`
			Capabilities    map[string]json.RawMessage `json:"capabilities"`
			ServerInfo      struct {
				Name string `json:"name"`
			} `json:"serverInfo"`
		} `json:"result"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &resp); err != nil {
		t.Fatalf("decode initialize response: %v (raw=%q)", err, out.String())
	}
	if resp.Result.ProtocolVersion != "2024-11-05" {
		t.Errorf("protocolVersion = %q, want 2024-11-05 (server supported, not echo of client 2025-06-18)", resp.Result.ProtocolVersion)
	}
	if _, ok := resp.Result.Capabilities["tools"]; !ok {
		t.Errorf("capabilities.tools missing: %v", resp.Result.Capabilities)
	}
	if resp.Result.ServerInfo.Name != "phonewave" {
		t.Errorf("serverInfo.name = %q, want phonewave", resp.Result.ServerInfo.Name)
	}
}

func TestMCPServer_NotificationsInitialized_NoResponse(t *testing.T) {
	// given: a JSON-RPC notification (no id)
	in := strings.NewReader(`{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n")
	var out bytes.Buffer
	srv := session.NewMCPServer(in, &out, nil)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then: notifications must not produce a response
	if strings.TrimSpace(out.String()) != "" {
		t.Errorf("notification must produce no response, got: %q", out.String())
	}
}

func TestMCPServer_OutboxStatus_Includes24hDeliveryStats(t *testing.T) {
	// given: config + delivery.log with one fresh DELIVERED and one
	// fresh FAILED line (refs issue 0034: "did my d-mail get
	// delivered?" must be answerable via MCP)
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("repositories: []\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	logBody := now + " DELIVERED a.md -> inbox\n" + now + " FAILED b.md route missing\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "delivery.log"), []byte(logBody), 0o644); err != nil {
		t.Fatalf("write delivery.log: %v", err)
	}

	in := strings.NewReader(`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"outbox_status","arguments":{"tool":"paintress"}}}` + "\n")
	var out bytes.Buffer
	srv := session.NewMCPServer(in, &out, nil).WithConfigPath(configPath)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then
	body := decodeFirstText(t, &out)
	if got, _ := body["delivered_24h"].(float64); int(got) != 1 {
		t.Errorf("delivered_24h = %v, want 1 (body=%v)", body["delivered_24h"], body)
	}
	if got, _ := body["failed_24h"].(float64); int(got) != 1 {
		t.Errorf("failed_24h = %v, want 1", body["failed_24h"])
	}
}

func TestMCPServer_Initialize_AdvertisesInstructions(t *testing.T) {
	// given
	in := strings.NewReader(`{"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"claude-code","version":"1.0"}}}` + "\n")
	var out bytes.Buffer
	srv := session.NewMCPServer(in, &out, nil)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then: Tool Search deferred loading reads instructions at startup
	var resp struct {
		Result struct {
			Instructions string `json:"instructions"`
		} `json:"result"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !strings.Contains(resp.Result.Instructions, "courier") {
		t.Errorf("instructions = %q, want a one-paragraph courier role summary", resp.Result.Instructions)
	}
}
