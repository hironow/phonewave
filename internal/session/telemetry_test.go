package session

import (
	"context"
	phonewave "github.com/hironow/phonewave"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/fsnotify/fsnotify"
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
	phonewave.Tracer = tp.Tracer("phonewave-test")
	t.Cleanup(func() {
		tp.Shutdown(context.Background())
		otel.SetTracerProvider(prev)
		phonewave.Tracer = prev.Tracer("phonewave")
	})
	return exp
}

func TestSetupTestTracer_SpansAvailableImmediately(t *testing.T) {
	// given — test tracer with in-memory exporter (sync processor)
	exp := setupTestTracer(t)

	// when — create and end a span
	_, span := phonewave.Tracer.Start(context.Background(), "sync-span")
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

// findSpanByName returns the first span with the given name, or nil.
func findSpanByName(spans tracetest.SpanStubs, name string) *tracetest.SpanStub {
	for i := range spans {
		if spans[i].Name == name {
			return &spans[i]
		}
	}
	return nil
}

// spanNames returns all span names for diagnostic output.
func spanNames(spans tracetest.SpanStubs) []string {
	names := make([]string, len(spans))
	for i, s := range spans {
		names[i] = s.Name
	}
	return names
}

func TestDaemon_Run_CreatesStartupScanSpan(t *testing.T) {
	exp := setupTestTracer(t)

	// given — minimal daemon setup
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".siren", "outbox")
	stateDir := filepath.Join(repoDir, ".phonewave")
	for _, dir := range []string{outbox, stateDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	d, err := NewDaemon(DaemonOptions{
		Routes:     []phonewave.ResolvedRoute{},
		OutboxDirs: []string{outbox},
		StateDir:   stateDir,
	}, phonewave.NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}

	// when — start and immediately stop the daemon
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- d.Run(ctx) }()
	cancel()
	<-errCh

	// then — should have "daemon.startup_scan" as an independent root span (no parent)
	spans := exp.GetSpans()
	s := findSpanByName(spans, "daemon.startup_scan")
	if s == nil {
		t.Fatalf("expected 'daemon.startup_scan' span, got: %v", spanNames(spans))
	}
	// Verify it is a root span (no parent)
	if s.Parent.IsValid() {
		t.Error("startup_scan span should be a root span (no parent), but has a parent")
	}
	// Verify no long-lived daemon.run span exists
	if findSpanByName(spans, "daemon.run") != nil {
		t.Error("daemon.run span should not exist (anti-pattern: long-lived root span)")
	}
}

func TestDaemon_HandleEvent_CreatesSpan(t *testing.T) {
	exp := setupTestTracer(t)

	// given — daemon with a valid route
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".siren", "outbox")
	inbox := filepath.Join(repoDir, ".expedition", "inbox")
	stateDir := filepath.Join(repoDir, ".phonewave")
	for _, dir := range []string{outbox, inbox, stateDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	dmailContent := "---\ndmail-schema-version: \"1\"\nname: spec-span\nkind: specification\ndescription: \"Span test\"\n---\n"
	dmailPath := filepath.Join(outbox, "spec-span.md")
	if err := os.WriteFile(dmailPath, []byte(dmailContent), 0644); err != nil {
		t.Fatal(err)
	}

	routes := []phonewave.ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{inbox}},
	}

	d, err := NewDaemon(DaemonOptions{
		Routes:     routes,
		OutboxDirs: []string{outbox},
		StateDir:   stateDir,
	}, phonewave.NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}

	dlog, err := NewDeliveryLog(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	d.dlog = dlog
	defer dlog.Close()

	// when — simulate a Create event
	d.handleEvent(fsnotify.Event{
		Name: dmailPath,
		Op:   fsnotify.Create,
	})

	// then — should have a "daemon.handle_event" span
	spans := exp.GetSpans()
	s := findSpanByName(spans, "daemon.handle_event")
	if s == nil {
		t.Errorf("expected 'daemon.handle_event' span, got: %v", spanNames(spans))
	}
}

func TestDaemon_HandleEvent_RecordsErrorOnFailure(t *testing.T) {
	exp := setupTestTracer(t)

	// given — daemon with NO routes (delivery will fail)
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".siren", "outbox")
	stateDir := filepath.Join(repoDir, ".phonewave")
	for _, dir := range []string{outbox, stateDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	dmailContent := "---\ndmail-schema-version: \"1\"\nname: spec-fail\nkind: specification\ndescription: \"Fail test\"\n---\n"
	dmailPath := filepath.Join(outbox, "spec-fail.md")
	if err := os.WriteFile(dmailPath, []byte(dmailContent), 0644); err != nil {
		t.Fatal(err)
	}

	d, err := NewDaemon(DaemonOptions{
		Routes:     []phonewave.ResolvedRoute{},
		OutboxDirs: []string{outbox},
		StateDir:   stateDir,
	}, phonewave.NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}

	dlog, err := NewDeliveryLog(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	d.dlog = dlog
	defer dlog.Close()

	// when — simulate a Create event that will fail delivery
	d.handleEvent(fsnotify.Event{
		Name: dmailPath,
		Op:   fsnotify.Create,
	})

	// then — span should have an error recorded
	spans := exp.GetSpans()
	s := findSpanByName(spans, "daemon.handle_event")
	if s == nil {
		t.Fatalf("expected 'daemon.handle_event' span, got: %v", spanNames(spans))
	}
	if len(s.Events) == 0 {
		t.Error("expected error event on span, got none")
	}
}

func TestDeliverData_CreatesSpan(t *testing.T) {
	exp := setupTestTracer(t)

	// given — valid D-Mail with a matching route
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".siren", "outbox")
	inbox := filepath.Join(repoDir, ".expedition", "inbox")
	for _, dir := range []string{outbox, inbox} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	dmailContent := []byte("---\ndmail-schema-version: \"1\"\nname: spec-span\nkind: specification\ndescription: \"Span test\"\n---\n")
	dmailPath := filepath.Join(outbox, "spec-span.md")
	if err := os.WriteFile(dmailPath, dmailContent, 0644); err != nil {
		t.Fatal(err)
	}

	routes := []phonewave.ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{inbox}},
	}

	// when
	_, err := DeliverData(context.Background(), dmailPath, dmailContent, routes)

	// then — should succeed and produce a "delivery.deliver" span
	if err != nil {
		t.Fatalf("DeliverData: %v", err)
	}
	spans := exp.GetSpans()
	s := findSpanByName(spans, "delivery.deliver")
	if s == nil {
		t.Fatalf("expected 'delivery.deliver' span, got: %v", spanNames(spans))
	}

	// Verify attributes
	var hasKind, hasPath bool
	for _, attr := range s.Attributes {
		switch string(attr.Key) {
		case "dmail.kind":
			hasKind = attr.Value.AsString() == "specification"
		case "dmail.path":
			hasPath = true
		}
	}
	if !hasKind {
		t.Error("span missing dmail.kind=specification attribute")
	}
	if !hasPath {
		t.Error("span missing dmail.path attribute")
	}
}

func TestDeliverData_RecordsErrorSpan(t *testing.T) {
	exp := setupTestTracer(t)

	// given — valid D-Mail but NO matching route
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".siren", "outbox")
	if err := os.MkdirAll(outbox, 0755); err != nil {
		t.Fatal(err)
	}

	dmailContent := []byte("---\ndmail-schema-version: \"1\"\nname: spec-err\nkind: specification\ndescription: \"Error span test\"\n---\n")
	dmailPath := filepath.Join(outbox, "spec-err.md")

	// Empty routes — will fail to find route
	routes := []phonewave.ResolvedRoute{}

	// when
	_, err := DeliverData(context.Background(), dmailPath, dmailContent, routes)

	// then — should fail
	if err == nil {
		t.Fatal("expected error for missing route")
	}

	// Span should exist with error status
	spans := exp.GetSpans()
	s := findSpanByName(spans, "delivery.deliver")
	if s == nil {
		t.Fatalf("expected 'delivery.deliver' span, got: %v", spanNames(spans))
	}
	if len(s.Events) == 0 {
		t.Error("expected error event on delivery span, got none")
	}
}
