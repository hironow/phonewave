package session

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/platform"
)

// MCPServer is a minimal stdio-based Model Context Protocol server
// scaffolded for the refs/issues/0027 jun15 MCP pivot (Phase 2d,
// optional visibility).
//
// Unlike the other four tools (paintress / sightjack / amadeus /
// dominator), phonewave is the courier daemon and never invokes
// `claude -p`. This MCP server therefore does NOT replace any
// deprecated LLM invocation path; instead it exposes phonewave's
// runtime data plane (outbox / inbox / dead-letter queue depth) as
// visibility tools that a claude code session can query while the
// other four tools' MCP servers handle their own data planes.
//
// Wire it into a claude code interactive session via --mcp-config so
// session-driven workflows can read phonewave queue depths without
// shelling out to the CLI.
//
// Protocol: JSON-RPC 2.0 over stdio, one envelope per line. Stderr
// carries human-readable diagnostics (per the project stdout/stderr
// separation invariant). Pattern follows paintress Phase 1
// (ADR 0017) + sightjack Phase 2a (ADR 0018) + amadeus Phase 2b
// (ADR 0026) + dominator Phase 2c (ADR 0003).
type MCPServer struct {
	in     io.Reader
	out    io.Writer
	logger domain.Logger
}

// NewMCPServer wires explicit I/O so tests can drive the server
// without subprocess overhead. Passing nil for logger uses NopLogger.
func NewMCPServer(in io.Reader, out io.Writer, logger domain.Logger) *MCPServer {
	if logger == nil {
		logger = &domain.NopLogger{}
	}
	return &MCPServer{in: in, out: out, logger: logger}
}

// jsonrpcMessage is the minimum JSON-RPC 2.0 envelope this skeleton
// understands. Method-specific params decode on demand from
// Params (json.RawMessage).
type jsonrpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *jsonrpcError   `json:"error,omitempty"`
}

type jsonrpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Serve reads messages from in line-by-line and writes responses to
// out until ctx cancels or stdin closes. Per-message decode errors
// surface as JSON-RPC error responses; only stream-level read errors
// abort Serve.
func (s *MCPServer) Serve(ctx context.Context) error {
	scanner := bufio.NewScanner(s.in)
	// 4 MiB buffer for symmetry with the other four tools.
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		if err := s.handle(ctx, line); err != nil {
			s.logger.Warn("mcp server: handle: %v", err)
		}
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("mcp server: read stdin: %w", err)
	}
	return nil
}

func (s *MCPServer) handle(ctx context.Context, line []byte) error {
	var msg jsonrpcMessage
	if err := json.Unmarshal(line, &msg); err != nil {
		return fmt.Errorf("decode request: %w", err)
	}
	switch msg.Method {
	case "tools/list":
		return s.respond(msg.ID, map[string]any{"tools": toolDescriptors()})
	case "tools/call":
		return s.handleToolsCall(ctx, msg)
	default:
		return s.respondError(msg.ID, -32601, fmt.Sprintf("method not implemented: %s", msg.Method))
	}
}

// handleToolsCall dispatches a single tools/call request and records
// MCP invocation metrics (mcp.tool.invocations counter +
// mcp.tool.duration histogram) for cost-monitoring verification post
// 2026-06-15 (refs/issues/0027 Phase 3 cost monitoring (a)).
//
// phonewave's tools are visibility-only (outbox / inbox queue depth
// reads) and never invoked claude -p in production, so status="ok"
// is the steady-state value. status="deprecated" is reserved for
// future stub responses that ship a stub:true flag.
func (s *MCPServer) handleToolsCall(ctx context.Context, msg jsonrpcMessage) error {
	start := time.Now()
	var call struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(msg.Params, &call); err != nil {
		platform.RecordMCPInvocation(ctx, "", "error", time.Since(start))
		return s.respondError(msg.ID, -32602, "invalid tools/call params")
	}

	status := "ok"
	var result map[string]any
	switch call.Name {
	case "phonewave.ping":
		result = textResult("pong")
	case "phonewave.outbox_status":
		result = stubOutboxStatus(call.Arguments)
		status = "deprecated"
	case "phonewave.inbox_status":
		result = stubInboxStatus(call.Arguments)
		status = "deprecated"
	default:
		platform.RecordMCPInvocation(ctx, call.Name, "error", time.Since(start))
		return s.respondError(msg.ID, -32601, fmt.Sprintf("unknown tool: %s", call.Name))
	}

	err := s.respond(msg.ID, result)
	if err != nil {
		status = "error"
	}
	platform.RecordMCPInvocation(ctx, call.Name, status, time.Since(start))
	return err
}

// toolDescriptors returns the Phase 2d MVP tool set. Each entry pins
// the interface (name, description, inputSchema) so claude code
// clients see a stable contract. The handler bodies (stubOutboxStatus
// / stubInboxStatus) are placeholders that ship in subsequent
// commits with real domain wiring against the courier daemon state.
func toolDescriptors() []map[string]any {
	return []map[string]any{
		{
			"name":        "phonewave.ping",
			"description": "Health check. Returns 'pong'.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			"name":        "phonewave.outbox_status",
			"description": "Return outbox queue depth + dead-letter count for the given tool (Phase 2d: stub echoes the requested tool with a contract descriptor).",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"tool": map[string]any{"type": "string", "description": "source tool name (paintress / sightjack / amadeus / dominator)"},
				},
				"required": []any{"tool"},
			},
		},
		{
			"name":        "phonewave.inbox_status",
			"description": "Return inbox queue depth + seen/ack counts for the given target tool (Phase 2d: stub echoes the requested tool with a contract descriptor).",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"tool": map[string]any{"type": "string", "description": "target tool name (paintress / sightjack / amadeus / dominator)"},
				},
				"required": []any{"tool"},
			},
		},
	}
}

// textResult wraps a plain string into the MCP content envelope.
func textResult(text string) map[string]any {
	return map[string]any{"content": []map[string]any{{"type": "text", "text": text}}}
}

// jsonResult marshals data as JSON and returns an MCP content envelope.
func jsonResult(data any) map[string]any {
	body, err := json.Marshal(data)
	if err != nil {
		return textResult(fmt.Sprintf(`{"error":"marshal failed: %v"}`, err))
	}
	return map[string]any{"content": []map[string]any{{"type": "text", "text": string(body)}}}
}

// stubOutboxStatus echoes the requested tool with a placeholder
// outbox depth payload so claude code clients can exercise the
// contract end-to-end before the real courier daemon state lookup
// wiring lands.
func stubOutboxStatus(args json.RawMessage) map[string]any {
	var payload struct {
		Tool string `json:"tool"`
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &payload)
	}
	return jsonResult(map[string]any{
		"stub":     true,
		"tool":     payload.Tool,
		"status":   nil,
		"reason":   "phase-2d-mvp: real outbox status lookup lands when the courier daemon state is exposed",
		"contract": map[string]any{"tool": "string", "depth": "integer (pending messages)", "dead_letter_count": "integer", "oldest_age_seconds": "integer"},
	})
}

// stubInboxStatus echoes the requested tool with a placeholder
// inbox depth payload.
func stubInboxStatus(args json.RawMessage) map[string]any {
	var payload struct {
		Tool string `json:"tool"`
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &payload)
	}
	return jsonResult(map[string]any{
		"stub":     true,
		"tool":     payload.Tool,
		"status":   nil,
		"reason":   "phase-2d-mvp: real inbox status lookup lands when the courier daemon state is exposed",
		"contract": map[string]any{"tool": "string", "depth": "integer (unconsumed messages)", "seen_count": "integer", "ack_count": "integer"},
	})
}

func (s *MCPServer) respond(id json.RawMessage, result any) error {
	return s.writeMessage(jsonrpcMessage{JSONRPC: "2.0", ID: id, Result: result})
}

func (s *MCPServer) respondError(id json.RawMessage, code int, message string) error {
	return s.writeMessage(jsonrpcMessage{JSONRPC: "2.0", ID: id, Error: &jsonrpcError{Code: code, Message: message}})
}

func (s *MCPServer) writeMessage(msg jsonrpcMessage) error {
	out, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("encode response: %w", err)
	}
	if _, err := s.out.Write(append(out, '\n')); err != nil {
		return fmt.Errorf("write response: %w", err)
	}
	return nil
}
