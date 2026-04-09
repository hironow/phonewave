package session

// white-box-reason: OTel instrumentation: tests unexported span recording and attribute verification

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
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

func spanAttributeValue(span *tracetest.SpanStub, key attribute.Key) string {
	for _, attr := range span.Attributes {
		if attr.Key == key {
			return attr.Value.AsString()
		}
	}
	return ""
}

func spanAttributeIntValue(span *tracetest.SpanStub, key attribute.Key) int64 {
	for _, attr := range span.Attributes {
		if attr.Key == key {
			return attr.Value.AsInt64()
		}
	}
	return 0
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
	daemon.handleEvent(context.Background(), fsnotify.Event{Name: dmailPath, Op: fsnotify.Create})

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
		{Kind: "design-feedback", FromOutbox: "/tmp/other", ToInboxes: []string{"/tmp/nope"}},
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
	daemon.handleEvent(context.Background(), fsnotify.Event{Name: dmailPath, Op: fsnotify.Create})

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
	_, err := DeliverData(context.Background(), dmailPath, []byte(dmailContent), routes, ds, nil)
	if err != nil {
		t.Fatalf("DeliverData: %v", err)
	}

	// then — should have a delivery span
	spans := exp.GetSpans()
	if s := findSpanByName(spans, "delivery.deliver"); s == nil {
		t.Errorf("missing delivery.deliver span; got spans: %v", spanNames(spans))
	}
}

func TestDeliverData_RecordsImprovementAttributes(t *testing.T) {
	exp := setupTestTracer(t)

	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".siren", "outbox")
	inbox := filepath.Join(repoDir, ".expedition", "inbox")
	for _, d := range []string{outbox, inbox} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}

	dmailContent := "---\ndmail-schema-version: \"1\"\nname: span-meta\nkind: implementation-feedback\ndescription: \"Span metadata\"\nmetadata:\n  improvement_schema_version: \"1\"\n  failure_type: execution_failure\n  severity: high\n  target_agent: paintress\n  correlation_id: corr-1\n  trace_id: trace-1\n  outcome: failed_again\n  recurrence_count: \"2\"\n  retry_allowed: \"false\"\n  escalation_reason: recurrence-threshold\n---\n"
	dmailPath := filepath.Join(outbox, "span-meta.md")
	if err := os.WriteFile(dmailPath, []byte(dmailContent), 0644); err != nil {
		t.Fatal(err)
	}

	routes := []domain.ResolvedRoute{
		{Kind: "implementation-feedback", FromOutbox: outbox, ToInboxes: []string{inbox}},
	}

	ds := newTestDeliveryStore(t)
	if _, err := DeliverData(context.Background(), dmailPath, []byte(dmailContent), routes, ds, nil); err != nil {
		t.Fatalf("DeliverData: %v", err)
	}

	spans := exp.GetSpans()
	s := findSpanByName(spans, "delivery.deliver")
	if s == nil {
		t.Fatalf("missing delivery.deliver span; got spans: %v", spanNames(spans))
	}
	if got := spanAttributeValue(s, "dmail.correlation_id"); got != "corr-1" {
		t.Fatalf("dmail.correlation_id = %q, want corr-1", got)
	}
	if got := spanAttributeValue(s, "dmail.outcome"); got != "failed_again" {
		t.Fatalf("dmail.outcome = %q, want failed_again", got)
	}
	if got := spanAttributeValue(s, "dmail.severity"); got != "high" {
		t.Fatalf("dmail.severity = %q, want high", got)
	}
	if got := spanAttributeValue(s, "dmail.improvement_schema_version"); got != "1" {
		t.Fatalf("dmail.improvement_schema_version = %q, want 1", got)
	}
	if got := spanAttributeValue(s, "dmail.retry_allowed"); got != "false" {
		t.Fatalf("dmail.retry_allowed = %q, want false", got)
	}
	if got := spanAttributeValue(s, "dmail.escalation_reason"); got != "recurrence-threshold" {
		t.Fatalf("dmail.escalation_reason = %q, want recurrence-threshold", got)
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
		{Kind: "design-feedback", FromOutbox: "/tmp/other", ToInboxes: []string{"/tmp/nope"}},
	}

	ds := newTestDeliveryStore(t)

	// when — deliver with no matching route
	_, err := DeliverData(context.Background(), filepath.Join(outbox, "err-span.md"), []byte(dmailContent), routes, ds, nil)

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

func TestRecordRetryCycleTelemetry_RecordsProviderStateAttrs(t *testing.T) {
	exp := setupTestTracer(t)

	recordRetryCycleTelemetry(context.Background(), 0, domain.ProviderStateSnapshot{
		State:           domain.ProviderStateWaiting,
		Reason:          "delivery_retry_backoff",
		RetryBudget:     1,
		ResumeCondition: "backoff-elapses",
	})

	spans := exp.GetSpans()
	s := findSpanByName(spans, "daemon.retry_cycle")
	if s == nil {
		t.Fatalf("missing daemon.retry_cycle span; got spans: %v", spanNames(spans))
	}
	if got := spanAttributeValue(s, domain.MetadataProviderState); got != string(domain.ProviderStateWaiting) {
		t.Fatalf("provider_state = %q, want %q", got, domain.ProviderStateWaiting)
	}
	if got := spanAttributeValue(s, domain.MetadataProviderReason); got != "delivery_retry_backoff" {
		t.Fatalf("provider_reason = %q, want delivery_retry_backoff", got)
	}
	if got := spanAttributeValue(s, domain.MetadataProviderResumeWhen); got != "backoff-elapses" {
		t.Fatalf("provider_resume_when = %q, want backoff-elapses", got)
	}
	if got := spanAttributeIntValue(s, domain.MetadataProviderRetryBudget); got != 1 {
		t.Fatalf("provider_retry_budget = %d, want 1", got)
	}
	if got := spanAttributeIntValue(s, "retry.success.count"); got != 0 {
		t.Fatalf("retry.success.count = %d, want 0", got)
	}
}

func TestHandleEvent_PropagatesTraceContext(t *testing.T) {
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

	dmailContent := "---\ndmail-schema-version: \"1\"\nname: ctx-prop\nkind: specification\ndescription: \"Context propagation test\"\n---\n\n# Test\n"
	dmailPath := filepath.Join(outbox, "ctx-prop.md")
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
	dlog, err := NewDeliveryLog(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	daemon.dlog = dlog
	defer dlog.Close()
	daemon.deliveryStore = newTestDeliveryStore(t)

	// when — create a parent span and pass its context to handleEvent
	parentCtx, parentSpan := platform.Tracer.Start(context.Background(), "test.parent")
	parentSpanCtx := parentSpan.SpanContext()
	daemon.handleEvent(parentCtx, fsnotify.Event{Name: dmailPath, Op: fsnotify.Create})
	parentSpan.End()

	// then — the child span "daemon.handle_event" should have the parent's trace ID
	spans := exp.GetSpans()
	child := findSpanByName(spans, "daemon.handle_event")
	if child == nil {
		t.Fatalf("missing daemon.handle_event span; got spans: %v", spanNames(spans))
	}

	if child.SpanContext.TraceID() != parentSpanCtx.TraceID() {
		t.Errorf("child trace ID = %s, want parent trace ID = %s", child.SpanContext.TraceID(), parentSpanCtx.TraceID())
	}
	if child.Parent.SpanID() != parentSpanCtx.SpanID() {
		t.Errorf("child parent span ID = %s, want %s", child.Parent.SpanID(), parentSpanCtx.SpanID())
	}
}

func TestRetryPending_PropagatesTraceContext(t *testing.T) {
	exp := setupTestTracer(t)

	// given — daemon with error queue so retryPending creates a span
	repoDir := t.TempDir()
	stateDir := filepath.Join(repoDir, ".phonewave")
	runDir := filepath.Join(stateDir, ".run")
	if err := os.MkdirAll(runDir, 0755); err != nil {
		t.Fatal(err)
	}

	daemon, err := NewDaemon(domain.DaemonOptions{
		StateDir:   stateDir,
		MaxRetries: 3,
	}, platform.NewLogger(nil, false))
	if err != nil {
		t.Fatal(err)
	}

	// Create a real error queue store so HasErrorQueue() returns true
	eq, eqErr := NewErrorQueueStore(stateDir)
	if eqErr != nil {
		t.Fatal(eqErr)
	}
	defer eq.Close()

	sess := NewDaemonSession(eq, nil, nil, stateDir, platform.NewLogger(nil, false))
	daemon.session = sess

	// when — create a parent span and pass its context to retryPending
	parentCtx, parentSpan := platform.Tracer.Start(context.Background(), "test.retry_parent")
	parentSpanCtx := parentSpan.SpanContext()
	daemon.retryPending(parentCtx)
	parentSpan.End()

	// then — the child span "daemon.retry_pending" should share the parent's trace ID
	spans := exp.GetSpans()
	child := findSpanByName(spans, "daemon.retry_pending")
	if child == nil {
		t.Fatalf("missing daemon.retry_pending span; got spans: %v", spanNames(spans))
	}

	if child.SpanContext.TraceID() != parentSpanCtx.TraceID() {
		t.Errorf("child trace ID = %s, want parent trace ID = %s", child.SpanContext.TraceID(), parentSpanCtx.TraceID())
	}
	if child.Parent.SpanID() != parentSpanCtx.SpanID() {
		t.Errorf("child parent span ID = %s, want %s", child.Parent.SpanID(), parentSpanCtx.SpanID())
	}
}
