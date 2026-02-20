package phonewave

import (
	"context"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// tracer is the package-level tracer used by all instrumented code.
// Initialized to a noop tracer so library consumers can use phonewave
// without calling InitTracer. When InitTracer is called with a valid
// OTLP endpoint, this is replaced with a recording tracer.
var tracer trace.Tracer = noop.NewTracerProvider().Tracer("phonewave")

// InitTracer sets up the OpenTelemetry TracerProvider.
// If OTEL_EXPORTER_OTLP_ENDPOINT or OTEL_EXPORTER_OTLP_TRACES_ENDPOINT is
// set, it creates an OTLP HTTP exporter with a BatchSpanProcessor.
// Otherwise, it keeps the noop TracerProvider.
// Returns a shutdown function that flushes and closes the exporter.
func InitTracer(serviceName, ver string) func(context.Context) error {
	// Respect both the generic and trace-specific OTLP endpoint variables.
	// otlptracehttp.New() honours both internally, but we need to gate on
	// them here to decide whether to create a real provider or stay noop.
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" && os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT") == "" {
		return func(context.Context) error { return nil }
	}

	exp, err := otlptracehttp.New(context.Background())
	if err != nil {
		LogWarn("Failed to create OTLP exporter, tracing disabled: %v", err)
		return func(context.Context) error { return nil }
	}

	res, _ := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(ver),
		),
	)

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	tracer = tp.Tracer(serviceName)

	return func(ctx context.Context) error {
		return tp.Shutdown(ctx)
	}
}
