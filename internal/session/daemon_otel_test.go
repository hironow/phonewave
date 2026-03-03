package session

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/platform"
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
	oldTracer := platform.Tracer
	platform.Tracer = tp.Tracer("phonewave-test")
	t.Cleanup(func() {
		tp.Shutdown(context.Background())
		otel.SetTracerProvider(prev)
		platform.Tracer = oldTracer
	})
	return exp
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

	// given — daemon with one outbox
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".siren", "outbox")
	inbox := filepath.Join(repoDir, ".expedition", "inbox")
	stateDir := filepath.Join(repoDir, ".run")
	for _, d := range []string{outbox, inbox, stateDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}

	routes := []domain.ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{inbox}},
	}

	d, err := NewDaemon(domain.DaemonOptions{
		Routes:     routes,
		OutboxDirs: []string{outbox},
		StateDir:   stateDir,
	}, platform.NewLogger(nil, false))
	if err != nil {
		t.Fatal(err)
	}

	// when — run daemon briefly
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- d.Run(ctx) }()

	// Wait for startup scan to complete (PID file signals readiness)
	waitForFile(t, filepath.Join(stateDir, "watch.pid"), 5*time.Second)
	cancel()
	<-done

	// then — should have a startup scan span
	spans := exp.GetSpans()
	if s := findSpanByName(spans, "daemon.startup_scan"); s == nil {
		t.Errorf("missing startup_scan span; got spans: %v", spanNames(spans))
	}
}

func TestDaemon_HandleEvent_CreatesSpan(t *testing.T) {
	exp := setupTestTracer(t)

	// given — daemon with valid route and a D-Mail in outbox
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".siren", "outbox")
	inbox := filepath.Join(repoDir, ".expedition", "inbox")
	stateDir := filepath.Join(repoDir, ".run")
	for _, d := range []string{outbox, inbox, stateDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}

	dmailContent := "---\ndmail-schema-version: \"1\"\nname: otel-001\nkind: specification\ndescription: \"OTel test\"\n---\n\n# Test\n"
	dmailPath := filepath.Join(outbox, "otel-001.md")
	if err := os.WriteFile(dmailPath, []byte(dmailContent), 0644); err != nil {
		t.Fatal(err)
	}

	routes := []domain.ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{inbox}},
	}

	daemon, err := NewDaemon(domain.DaemonOptions{
		Routes:     routes,
		OutboxDirs: []string{outbox},
		StateDir:   stateDir,
	}, platform.NewLogger(nil, false))
	if err != nil {
		t.Fatal(err)
	}
	// Manually set dlog and deliveryStore so handleEvent can log and deliver
	dlog, err := NewDeliveryLog(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	daemon.dlog = dlog
	defer dlog.Close()
	daemon.deliveryStore = newTestDeliveryStore(t)

	// when — call handleEvent directly
	daemon.handleEvent(fsnotify.Event{Name: dmailPath, Op: fsnotify.Create})

	// then — should have a handle_event span
	spans := exp.GetSpans()
	if s := findSpanByName(spans, "daemon.handle_event"); s == nil {
		t.Errorf("missing handle_event span; got spans: %v", spanNames(spans))
	}
}

func TestDaemon_HandleEvent_RecordsErrorOnFailure(t *testing.T) {
	exp := setupTestTracer(t)

	// given — daemon with NO matching route (will cause delivery failure)
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".siren", "outbox")
	stateDir := filepath.Join(repoDir, ".run")
	for _, d := range []string{outbox, stateDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}

	dmailContent := "---\ndmail-schema-version: \"1\"\nname: otel-err\nkind: specification\ndescription: \"Will fail\"\n---\n"
	dmailPath := filepath.Join(outbox, "otel-err.md")
	if err := os.WriteFile(dmailPath, []byte(dmailContent), 0644); err != nil {
		t.Fatal(err)
	}

	// No routes for "specification" from this outbox
	routes := []domain.ResolvedRoute{
		{Kind: "feedback", FromOutbox: "/tmp/other", ToInboxes: []string{"/tmp/nope"}},
	}

	daemon, err := NewDaemon(domain.DaemonOptions{
		Routes:     routes,
		OutboxDirs: []string{outbox},
		StateDir:   stateDir,
	}, platform.NewLogger(nil, false))
	if err != nil {
		t.Fatal(err)
	}
	dlog, err := NewDeliveryLog(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	daemon.dlog = dlog
	defer dlog.Close()
	daemon.deliveryStore = newTestDeliveryStore(t)

	// when — handle event with no matching route
	daemon.handleEvent(fsnotify.Event{Name: dmailPath, Op: fsnotify.Create})

	// then — span should exist and have error status
	spans := exp.GetSpans()
	s := findSpanByName(spans, "delivery.deliver")
	if s == nil {
		t.Fatalf("missing delivery.deliver span; got spans: %v", spanNames(spans))
	}
	if s.Status.Code != codes.Error {
		t.Errorf("span status = %v, want Error", s.Status.Code)
	}
}

func TestDeliverData_CreatesSpan(t *testing.T) {
	exp := setupTestTracer(t)

	// given — valid D-Mail with matching route
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".siren", "outbox")
	inbox := filepath.Join(repoDir, ".expedition", "inbox")
	for _, d := range []string{outbox, inbox} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}

	dmailContent := "---\ndmail-schema-version: \"1\"\nname: span-test\nkind: specification\ndescription: \"Span test\"\n---\n\n# Content\n"
	dmailPath := filepath.Join(outbox, "span-test.md")
	if err := os.WriteFile(dmailPath, []byte(dmailContent), 0644); err != nil {
		t.Fatal(err)
	}

	routes := []domain.ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{inbox}},
	}

	ds := newTestDeliveryStore(t)

	// when
	_, err := DeliverData(context.Background(), dmailPath, []byte(dmailContent), routes, ds)
	if err != nil {
		t.Fatalf("DeliverData: %v", err)
	}

	// then — should have a delivery span
	spans := exp.GetSpans()
	if s := findSpanByName(spans, "delivery.deliver"); s == nil {
		t.Errorf("missing delivery.deliver span; got spans: %v", spanNames(spans))
	}
}

func TestDeliverData_RecordsErrorSpan(t *testing.T) {
	exp := setupTestTracer(t)

	// given — D-Mail with no matching route
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".siren", "outbox")
	if err := os.MkdirAll(outbox, 0755); err != nil {
		t.Fatal(err)
	}

	dmailContent := "---\ndmail-schema-version: \"1\"\nname: err-span\nkind: specification\ndescription: \"Error span\"\n---\n"

	routes := []domain.ResolvedRoute{
		{Kind: "feedback", FromOutbox: "/tmp/other", ToInboxes: []string{"/tmp/nope"}},
	}

	ds := newTestDeliveryStore(t)

	// when — deliver with no matching route
	_, err := DeliverData(context.Background(), filepath.Join(outbox, "err-span.md"), []byte(dmailContent), routes, ds)

	// then — should error
	if err == nil {
		t.Fatal("expected error for unmatched route")
	}

	// span should exist with error status
	spans := exp.GetSpans()
	s := findSpanByName(spans, "delivery.deliver")
	if s == nil {
		t.Fatalf("missing delivery.deliver span; got spans: %v", spanNames(spans))
	}
	if s.Status.Code != codes.Error {
		t.Errorf("span status = %v, want Error", s.Status.Code)
	}
}
