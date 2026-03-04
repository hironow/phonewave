package platform

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// setupTestTracer installs an InMemoryExporter with a synchronous span
// processor so spans are immediately available for inspection. It restores
// the global TracerProvider and package-level tracer after the test.
func setupTestTracer(t *testing.T) *tracetest.InMemoryExporter {
	t.Helper()
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	Tracer = tp.Tracer("phonewave-test")
	t.Cleanup(func() {
		tp.Shutdown(context.Background())
		otel.SetTracerProvider(prev)
		Tracer = prev.Tracer("phonewave")
	})
	return exp
}

func TestMultiExporter_BothReceive(t *testing.T) {
	exp1 := tracetest.NewInMemoryExporter()
	exp2 := tracetest.NewInMemoryExporter()

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exp1),
		sdktrace.WithSyncer(exp2),
	)
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	oldTracer := Tracer
	Tracer = tp.Tracer("phonewave-test")
	t.Cleanup(func() {
		tp.Shutdown(context.Background())
		otel.SetTracerProvider(prev)
		Tracer = oldTracer
	})

	_, span := Tracer.Start(context.Background(), "multi-span") // nosemgrep: adr0003-otel-span-without-defer-end — test span, immediately ended
	span.End()

	if len(exp1.GetSpans()) == 0 {
		t.Error("exporter 1 received no spans")
	}
	if len(exp2.GetSpans()) == 0 {
		t.Error("exporter 2 received no spans")
	}
}

func TestSetupTestTracer_SpansAvailableImmediately(t *testing.T) {
	// given — test tracer with in-memory exporter (sync processor)
	exp := setupTestTracer(t)

	// when — create and end a span
	_, span := Tracer.Start(context.Background(), "sync-span") // nosemgrep: adr0003-otel-span-without-defer-end — test span, immediately ended
	span.End()

	// then — span should appear in exporter immediately (sync processor)
	spans := exp.GetSpans()
	if len(spans) == 0 {
		t.Fatal("expected at least 1 span in InMemoryExporter after span.End()")
	}
	if spans[0].Name != "sync-span" {
		t.Errorf("span name = %q, want %q", spans[0].Name, "sync-span")
	}
}
