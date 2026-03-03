package cmd

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/hironow/phonewave/internal/platform"
)

// setupTestTracer installs an InMemoryExporter with a synchronous span processor
// so spans are immediately available for inspection. It restores the global
// TracerProvider after the test.
func setupTestTracer(t *testing.T) *tracetest.InMemoryExporter {
	t.Helper()
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	oldTracer := platform.Tracer
	platform.Tracer = tp.Tracer("phonewave-test")
	t.Cleanup(func() {
		tp.Shutdown(context.Background())
		otel.SetTracerProvider(prev)
		platform.Tracer = oldTracer
	})
	return exp
}

func TestInitTracer_NoopWhenEndpointUnset(t *testing.T) {
	// given — no OTLP endpoint configured (neither generic nor trace-specific)
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "")

	// when
	shutdown := initTracer("test-svc", "0.0.1")
	defer shutdown(context.Background())

	// After initTracer with no endpoint, tracer is noop. We can only verify
	// that the function returns without error and shutdown is callable.
}

func TestParseExtraEndpoints_CommaSeparated(t *testing.T) {
	eps := parseExtraEndpoints("localhost:4318,weave.local:4318")
	if len(eps) != 2 {
		t.Fatalf("got %d endpoints, want 2", len(eps))
	}
	if eps[0] != "localhost:4318" {
		t.Errorf("eps[0] = %q, want %q", eps[0], "localhost:4318")
	}
}

func TestParseExtraEndpoints_Empty(t *testing.T) {
	eps := parseExtraEndpoints("")
	if len(eps) != 0 {
		t.Errorf("got %d endpoints, want 0", len(eps))
	}
}

func TestStartRootSpan_CreatesNamedSpan(t *testing.T) {
	// given
	exp := setupTestTracer(t)

	// when
	_ = startRootSpan(context.Background(), "run")
	endRootSpan()

	// then
	spans := exp.GetSpans()
	if len(spans) == 0 {
		t.Fatal("expected at least 1 span")
	}
	if spans[0].Name != "phonewave.run" {
		t.Errorf("span name = %q, want %q", spans[0].Name, "phonewave.run")
	}
	var found bool
	for _, attr := range spans[0].Attributes {
		if string(attr.Key) == "phonewave.command" && attr.Value.AsString() == "run" {
			found = true
		}
	}
	if !found {
		t.Error("expected phonewave.command=run attribute on root span")
	}
}

func TestEndRootSpan_NilSafe(t *testing.T) {
	// given — rootSpan is nil (no startRootSpan called)
	rootSpan = nil

	// when / then — must not panic
	endRootSpan()
}
