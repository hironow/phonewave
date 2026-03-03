package session

import (
	"context"
	"fmt"
	"time"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/platform"
)

// DaemonSession holds session-layer dependencies for the daemon's I/O
// orchestration. The root Daemon retains fsnotify + worker pool; DaemonSession
// provides stores and logging for delivery, error recording, and event persistence.
type DaemonSession struct {
	ErrorQueue  domain.ErrorQueueStore
	EventStore  domain.EventStore
	DeliveryLog *DeliveryLog
	Routes      []domain.ResolvedRoute
	StateDir    string
	Logger      *domain.Logger
	Dispatcher  domain.EventDispatcher
}

// NewDaemonSession creates a DaemonSession with the given dependencies.
// All fields except Dispatcher are required; Dispatcher may be nil.
func NewDaemonSession(
	errorQueue domain.ErrorQueueStore,
	eventStore domain.EventStore,
	deliveryLog *DeliveryLog,
	routes []domain.ResolvedRoute,
	stateDir string,
	logger *domain.Logger,
) *DaemonSession {
	return &DaemonSession{
		ErrorQueue:  errorQueue,
		EventStore:  eventStore,
		DeliveryLog: deliveryLog,
		Routes:      routes,
		StateDir:    stateDir,
		Logger:      logger,
	}
}

// RecordDeliveryEvent records a delivery.completed event to the event store.
// Best-effort: errors are logged but do not fail the delivery.
func (s *DaemonSession) RecordDeliveryEvent(result *domain.DeliveryResult) {
	platform.RecordDelivery(context.Background(), "completed", result.Kind)
	if s.EventStore == nil {
		return
	}
	ev, err := domain.NewEvent(domain.EventDeliveryCompleted, map[string]string{
		"kind":   result.Kind,
		"source": result.SourcePath,
	}, time.Now().UTC())
	if err != nil {
		s.Logger.Warn("record delivery event: %v", err)
		return
	}
	if err := s.EventStore.Append(ev); err != nil {
		s.Logger.Warn("append delivery event: %v", err)
	}
	if s.Dispatcher != nil {
		s.Dispatcher.Dispatch(context.Background(), ev) //nolint:errcheck
	}
}

// RecordFailureEvent records a delivery.failed event to the event store.
// Best-effort: errors are logged but do not fail the error recording.
func (s *DaemonSession) RecordFailureEvent(filePath string, kind string, deliverErr error) {
	platform.RecordDelivery(context.Background(), "failed", kind)
	if s.EventStore == nil {
		return
	}
	ev, err := domain.NewEvent(domain.EventDeliveryFailed, map[string]string{
		"file":  filePath,
		"kind":  kind,
		"error": deliverErr.Error(),
	}, time.Now().UTC())
	if err != nil {
		s.Logger.Warn("record failure event: %v", err)
		return
	}
	if err := s.EventStore.Append(ev); err != nil {
		s.Logger.Warn("append failure event: %v", err)
	}
	if s.Dispatcher != nil {
		s.Dispatcher.Dispatch(context.Background(), ev) //nolint:errcheck
	}
}

// RecordScanEvent records a scan.completed event to the event store.
func (s *DaemonSession) RecordScanEvent(outboxDir string, deliveredCount int, errorCount int) {
	if s.EventStore == nil {
		return
	}
	ev, err := domain.NewEvent(domain.EventScanCompleted, map[string]string{
		"outbox":    outboxDir,
		"delivered": intToStr(deliveredCount),
		"errors":    intToStr(errorCount),
	}, time.Now().UTC())
	if err != nil {
		s.Logger.Warn("record scan event: %v", err)
		return
	}
	if err := s.EventStore.Append(ev); err != nil {
		s.Logger.Warn("append scan event: %v", err)
	}
	if s.Dispatcher != nil {
		s.Dispatcher.Dispatch(context.Background(), ev) //nolint:errcheck
	}
}

// RecordRetryEvent records an error.retried event to the event store.
func (s *DaemonSession) RecordRetryEvent(name string, kind string) {
	if s.EventStore == nil {
		return
	}
	ev, err := domain.NewEvent(domain.EventErrorRetried, map[string]string{
		"name": name,
		"kind": kind,
	}, time.Now().UTC())
	if err != nil {
		s.Logger.Warn("record retry event: %v", err)
		return
	}
	if err := s.EventStore.Append(ev); err != nil {
		s.Logger.Warn("append retry event: %v", err)
	}
	if s.Dispatcher != nil {
		s.Dispatcher.Dispatch(context.Background(), ev) //nolint:errcheck
	}
}

func intToStr(n int) string {
	return fmt.Sprintf("%d", n)
}
