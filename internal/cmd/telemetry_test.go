package cmd

import (
	"context"
	"testing"
)

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
