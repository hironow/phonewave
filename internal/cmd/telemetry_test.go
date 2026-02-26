package cmd

import (
	"context"
	"testing"

	phonewave "github.com/hironow/phonewave"
)

func TestInitTracer_NoopWhenEndpointUnset(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "")

	shutdown := initTracer("test-svc", "0.0.1")
	defer shutdown(context.Background())

	_, span := phonewave.Tracer.Start(context.Background(), "test-span")
	defer span.End()

	if span.IsRecording() {
		t.Error("span should NOT be recording when endpoint is unset (noop provider)")
	}
}
