package platform

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// RecordMCPInvocation increments mcp.tool.invocations counter and
// records mcp.tool.duration histogram for a `tools/call` invocation
// on the phonewave MCP server.
//
// Phase 3 (refs/issues/0027) cost monitoring (a): every MCP tool call
// is counted with (tool.name, result.status) attrs so credit-pool 0
// consumption can be verified post 2026-06-15 via OTel + Anthropic
// dashboard cross-check. paintress mcp_metrics.go is the reference
// impl; this file is a symmetric copy adapted for phonewave.
//
// Note: phonewave is the courier daemon and never invoked `claude -p`
// in production code. Its MCP server is purely additive (= visibility
// tools: outbox_status / inbox_status). Those two tools tag their
// invocations with status="visibility" as a cost-monitoring marker for
// the post-2026-06-15 credit-pool split; ping returns "ok".
//
// status values: "ok" (= JSON-RPC result returned)、 "error" (= JSON-RPC
// error returned)、 "visibility" (= cost-monitoring marker on the
// visibility tools; no stub flag is emitted).
// duration is measured from request decode to response write.
func RecordMCPInvocation(ctx context.Context, toolName, status string, duration time.Duration) {
	attrs := metric.WithAttributes(
		attribute.String("tool.name", SanitizeUTF8(toolName)),
		attribute.String("result.status", SanitizeUTF8(status)),
	)

	counter, err := Meter.Int64Counter("mcp.tool.invocations",
		metric.WithDescription("Total MCP tools/call invocations on phonewave MCP server"),
	)
	if err == nil {
		counter.Add(ctx, 1, attrs)
	}

	histogram, err := Meter.Float64Histogram("mcp.tool.duration",
		metric.WithDescription("Duration (seconds) of MCP tools/call invocations on phonewave MCP server"),
		metric.WithUnit("s"),
	)
	if err == nil {
		histogram.Record(ctx, duration.Seconds(), attrs)
	}
}
