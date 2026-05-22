package session

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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
// (ADR 0026) + dominator Phase 2c (ADR 0003) + paintress Phase 3
// real impl (= 83cb3ca) WithContinent pattern.
//
// configPath is the phonewave config.yaml path used by real-impl
// MCP tools to resolve outbox / inbox dirs across all configured
// repositories. When empty, real-impl tools return uninitialized.
type MCPServer struct {
	in         io.Reader
	out        io.Writer
	logger     domain.Logger
	configPath string
}

// NewMCPServer wires explicit I/O so tests can drive the server
// without subprocess overhead. Passing nil for logger uses NopLogger.
func NewMCPServer(in io.Reader, out io.Writer, logger domain.Logger) *MCPServer {
	if logger == nil {
		logger = &domain.NopLogger{}
	}
	return &MCPServer{in: in, out: out, logger: logger}
}

// WithConfigPath sets the phonewave config.yaml path used by
// real-impl MCP tools. Returns s for chaining (= 5-tool symmetric
// builder option: paintress.WithContinent / sightjack.WithBaseDir /
// amadeus.WithGateDir / dominator.WithPassDir / phonewave.WithConfigPath).
func (s *MCPServer) WithConfigPath(configPath string) *MCPServer {
	s.configPath = configPath
	return s
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
		result = realOutboxStatus(ctx, s.configPath, call.Arguments)
		status = "deprecated"
	case "phonewave.inbox_status":
		result = realInboxStatus(s.configPath, call.Arguments)
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
			"description": "Return outbox queue depth + dead-letter count (from the SQLite delivery store) + oldest age in seconds for the given source tool.",
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
			"description": "Return inbox queue depth + oldest age in seconds for the given target tool. Inbox has no dead-letter count (that is an outbox-side concept).",
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

// scanDirDepth counts non-directory entries in dir and returns
// (count, oldest_mtime). Missing dir returns (0, zero time). The
// oldest mtime helps the session estimate queue age — useful for
// detecting a stuck courier daemon.
func scanDirDepth(dir string) (int, time.Time) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, time.Time{}
	}
	depth := 0
	var oldest time.Time
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		depth++
		info, err := e.Info()
		if err != nil {
			continue
		}
		mt := info.ModTime()
		if oldest.IsZero() || mt.Before(oldest) {
			oldest = mt
		}
	}
	return depth, oldest
}

// loadDeadLetterCount opens the phonewave delivery store at
// stateDir/.run/delivery.db and returns the dead-letter count
// (= staged_delivery rows with retry_count >= maxDeliveryRetryCount).
// Returns 0 when the DB does not exist (= phonewave hasn't run yet).
func loadDeadLetterCount(ctx context.Context, stateDir string) (int, error) {
	dbPath := filepath.Join(stateDir, ".run", "delivery.db")
	if _, err := os.Stat(dbPath); errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	store, err := NewSQLiteDeliveryStore(stateDir)
	if err != nil {
		return 0, fmt.Errorf("open delivery store: %w", err)
	}
	defer store.Close()
	count, err := store.DeadLetterCount(ctx)
	if err != nil {
		return 0, fmt.Errorf("delivery store dead-letter count: %w", err)
	}
	return count, nil
}

// realOutboxStatus loads the phonewave config and aggregates outbox
// file counts across all configured repos' endpoints. Phase 4 adds
// dead_letter_count (= SQLite delivery store query) +
// oldest_age_seconds (= oldest outbox file mtime).
//
// Pattern: paintress.next_issue (= 83cb3ca) symmetric copy +
// Phase 4 follow-up dead-letter telemetry (refs/issues/0027).
func realOutboxStatus(ctx context.Context, configPath string, args json.RawMessage) map[string]any {
	var payload struct {
		Tool string `json:"tool"`
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &payload)
	}
	if configPath == "" {
		return jsonResult(map[string]any{
			"initialized": false,
			"reason":      "phonewave mcp config path not configured",
			"tool":        payload.Tool,
		})
	}
	cfg, err := LoadConfig(configPath)
	if err != nil {
		return jsonResult(map[string]any{
			"initialized": false,
			"reason":      fmt.Sprintf("config load failed: %v", err),
			"tool":        payload.Tool,
		})
	}
	totalDepth := 0
	endpoints := 0
	var globalOldest time.Time
	for _, repo := range cfg.Repositories {
		if payload.Tool != "" && !strings.Contains(repo.Path, payload.Tool) {
			continue
		}
		for _, ep := range repo.Endpoints {
			if len(ep.Produces) == 0 {
				continue
			}
			endpoints++
			depth, oldest := scanDirDepth(filepath.Join(repo.Path, ep.Dir, "outbox"))
			totalDepth += depth
			if !oldest.IsZero() && (globalOldest.IsZero() || oldest.Before(globalOldest)) {
				globalOldest = oldest
			}
		}
	}

	stateDir := filepath.Dir(configPath)
	deadLetterCount, deadLetterErr := loadDeadLetterCount(ctx, stateDir)
	oldestAgeSec := 0
	if !globalOldest.IsZero() {
		oldestAgeSec = int(time.Since(globalOldest).Seconds())
	}
	result := map[string]any{
		"initialized":        true,
		"tool":               payload.Tool,
		"endpoint_count":     endpoints,
		"total_depth":        totalDepth,
		"dead_letter_count":  deadLetterCount,
		"oldest_age_seconds": oldestAgeSec,
	}
	if deadLetterErr != nil {
		result["dead_letter_error"] = deadLetterErr.Error()
	}
	return jsonResult(result)
}

// realInboxStatus loads the phonewave config and aggregates inbox
// file counts across all configured repos' endpoints. Phase 4 adds
// oldest_age_seconds (= oldest inbox file mtime). Inbox does not
// have a SQLite-tracked dead-letter notion (= dead letters are an
// outbox-side concept), so dead_letter_count is not included.
func realInboxStatus(configPath string, args json.RawMessage) map[string]any {
	var payload struct {
		Tool string `json:"tool"`
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &payload)
	}
	if configPath == "" {
		return jsonResult(map[string]any{
			"initialized": false,
			"reason":      "phonewave mcp config path not configured",
			"tool":        payload.Tool,
		})
	}
	cfg, err := LoadConfig(configPath)
	if err != nil {
		return jsonResult(map[string]any{
			"initialized": false,
			"reason":      fmt.Sprintf("config load failed: %v", err),
			"tool":        payload.Tool,
		})
	}
	totalDepth := 0
	endpoints := 0
	var globalOldest time.Time
	for _, repo := range cfg.Repositories {
		if payload.Tool != "" && !strings.Contains(repo.Path, payload.Tool) {
			continue
		}
		for _, ep := range repo.Endpoints {
			if len(ep.Consumes) == 0 {
				continue
			}
			endpoints++
			depth, oldest := scanDirDepth(filepath.Join(repo.Path, ep.Dir, "inbox"))
			totalDepth += depth
			if !oldest.IsZero() && (globalOldest.IsZero() || oldest.Before(globalOldest)) {
				globalOldest = oldest
			}
		}
	}
	oldestAgeSec := 0
	if !globalOldest.IsZero() {
		oldestAgeSec = int(time.Since(globalOldest).Seconds())
	}
	return jsonResult(map[string]any{
		"initialized":        true,
		"tool":               payload.Tool,
		"endpoint_count":     endpoints,
		"total_depth":        totalDepth,
		"oldest_age_seconds": oldestAgeSec,
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
