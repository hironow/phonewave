//go:build scenario

package scenario_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// MCPToolCall spawns `<tool> mcp` inside the workspace repo, performs
// the initialize handshake, invokes one tool, and returns the decoded
// JSON body from result.content[0].text. This is the scripted (non-LLM)
// stand-in for a human-initiated Claude Code session driving the MCP
// data planes (refs issue 0032: the L1.5 closed-loop proof).
//
// Scenario-layer classification: real binaries + real filesystem +
// real courier; no LLM is involved anywhere on this path.
func (w *Workspace) MCPToolCall(t *testing.T, ctx context.Context, tool, name string, args map[string]any) map[string]any {
	t.Helper()

	cmd := exec.CommandContext(ctx, tool, "mcp")
	cmd.Dir = w.RepoPath
	cmd.Env = w.Env
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("%s mcp stdin: %v", tool, err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("%s mcp stdout: %v", tool, err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start %s mcp: %v", tool, err)
	}
	defer func() {
		_ = stdin.Close()
		_ = cmd.Wait()
	}()

	argsJSON, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}
	lines := []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"scenario-mcp-client","version":"0"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		fmt.Sprintf(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":%q,"arguments":%s}}`, name, argsJSON),
	}
	for _, line := range lines {
		if _, err := stdin.Write([]byte(line + "\n")); err != nil {
			t.Fatalf("write to %s mcp: %v", tool, err)
		}
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	deadline := time.After(20 * time.Second)
	respCh := make(chan map[string]any, 1)
	go func() {
		for scanner.Scan() {
			var resp struct {
				ID     json.RawMessage `json:"id"`
				Result map[string]any  `json:"result"`
				Error  map[string]any  `json:"error"`
			}
			if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
				continue
			}
			if string(resp.ID) != "2" {
				continue
			}
			if resp.Error != nil {
				respCh <- map[string]any{"__rpc_error": resp.Error}
				return
			}
			respCh <- resp.Result
			return
		}
	}()

	var result map[string]any
	select {
	case result = <-respCh:
	case <-deadline:
		t.Fatalf("%s mcp %s: timed out waiting for response", tool, name)
	}
	if rpcErr, ok := result["__rpc_error"]; ok {
		t.Fatalf("%s mcp %s: rpc error: %v", tool, name, rpcErr)
	}
	content, ok := result["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("%s mcp %s: content missing: %v", tool, name, result)
	}
	first, _ := content[0].(map[string]any)
	text, _ := first["text"].(string)
	var body map[string]any
	if err := json.Unmarshal([]byte(text), &body); err != nil {
		t.Fatalf("%s mcp %s: decode body: %v (text=%q)", tool, name, err, text)
	}
	return body
}

// TestScenario_MCPClosedLoop_L1_5 proves the post-pivot ecosystem is
// alive end-to-end WITHOUT any legacy pipeline command: a scripted MCP
// client (the non-LLM stand-in for a Claude Code session) drives the
// restored write tools — sightjack registers waves + emits a
// specification, phonewave delivers it, paintress emits a report,
// amadeus emits implementation-feedback, and the fan-out lands back at
// the producers. GREEN here is the mechanical definition of "ecosystem
// restored" (refs issue 0032 acceptance criteria).
func TestScenario_MCPClosedLoop_L1_5(t *testing.T) {
	if testing.Short() {
		t.Skip("scenario tests are not short")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	ws := NewWorkspace(t, "minimal")
	obs := NewObserver(ws, t)

	pw := ws.StartPhonewave(t, ctx)
	defer ws.StopPhonewave(t, pw)
	defer ws.DumpPhonewaveLog(t, pw)

	// Step 1: designer write path — register waves, verify next_wave
	// serves them, then emit the specification d-mail.
	reg := ws.MCPToolCall(t, ctx, "sightjack", "register_waves", map[string]any{
		"session_id":   "mcp-loop-s1",
		"cluster_name": "auth",
		"waves": []map[string]any{{
			"id": "w1", "title": "Fix token refresh", "status": "available",
			"actions": []map[string]any{{"type": "edit", "description": "x"}},
		}},
	})
	if reg["registered"] != true {
		t.Fatalf("register_waves failed: %v", reg)
	}
	nw := ws.MCPToolCall(t, ctx, "sightjack", "next_wave", map[string]any{"session_id": "mcp-loop-s1"})
	wave, _ := nw["next_wave"].(map[string]any)
	if wave == nil || wave["id"] != "w1" {
		t.Fatalf("next_wave did not serve the registered wave: %v", nw)
	}
	spec := ws.MCPToolCall(t, ctx, "sightjack", "dmail", map[string]any{
		"kind": "specification", "name": "sj-spec-w1",
		"description": "wave w1 spec", "body": "# Spec\n\nimplement w1", "issues": []string{"X-1"},
	})
	if spec["sent"] != true {
		t.Fatalf("sightjack dmail failed: %v", spec)
	}
	specPath := ws.WaitForDMail(t, ".expedition", "inbox", 30*time.Second)
	ws.WaitForAbsent(t, ".siren", "outbox", 10*time.Second)
	obs.AssertDMailKind(specPath, "specification")
	t.Log("step 1: sightjack MCP write path → specification delivered to .expedition/inbox")

	// Step 2: implementer write path — emit the expedition report.
	rep := ws.MCPToolCall(t, ctx, "paintress", "dmail", map[string]any{
		"kind": "report", "name": "pt-report-x1-001",
		"description": "expedition 1 completed X-1", "body": "# Report\n\nPR opened", "issues": []string{"X-1"},
	})
	if rep["sent"] != true {
		t.Fatalf("paintress dmail failed: %v", rep)
	}
	reportPath := ws.WaitForDMail(t, ".gate", "inbox", 30*time.Second)
	ws.WaitForAbsent(t, ".expedition", "outbox", 10*time.Second)
	obs.AssertDMailKind(reportPath, "report")
	t.Log("step 2: paintress MCP dmail → report delivered to .gate/inbox")

	// Step 3: verifier write path — emit implementation-feedback and
	// verify the fan-out reaches the implementer.
	fb := ws.MCPToolCall(t, ctx, "amadeus", "dmail", map[string]any{
		"kind": "implementation-feedback", "name": "am-implfb-x1",
		"description": "PR 1 axis findings", "body": "# Findings\n\nfix dependency direction", "issues": []string{"X-1"},
	})
	if fb["sent"] != true {
		t.Fatalf("amadeus dmail failed: %v", fb)
	}
	// The spec from step 1 is still in .expedition/inbox (no consumer
	// ran), so wait for the second file and assert the feedback by name.
	ws.WaitForDMailCount(t, ".expedition", "inbox", 2, 30*time.Second)
	feedbackPath := filepath.Join(ws.RepoPath, ".expedition", "inbox", "am-implfb-x1.md")
	obs.AssertDMailKind(feedbackPath, "implementation-feedback")
	ws.WaitForAbsent(t, ".gate", "outbox", 10*time.Second)
	t.Log("step 3: amadeus MCP dmail → implementation-feedback delivered to .expedition/inbox")

	obs.AssertAllOutboxEmpty()
	t.Log("MCP closed loop complete: spec → report → feedback via MCP write tools only")
}
