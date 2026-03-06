package session

import (
	"context"
	"time"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/platform"
	"github.com/hironow/phonewave/internal/usecase/port"
)

// DaemonSession holds session-layer dependencies for the daemon's I/O
// orchestration. The root Daemon retains fsnotify + worker pool; DaemonSession
// provides stores and logging for delivery, error recording, and event persistence.
type DaemonSession struct {
	Emitter     port.DaemonEventEmitter
	ErrorQueue  port.ErrorQueueStore
	DeliveryLog *DeliveryLog
	Routes      []domain.ResolvedRoute
	StateDir    string
	Logger      domain.Logger
}

// NewDaemonSession creates a DaemonSession with the given dependencies.
// Emitter is injected separately via the DaemonRunner.SetEmitter call.
func NewDaemonSession(
	errorQueue port.ErrorQueueStore,
	deliveryLog *DeliveryLog,
	routes []domain.ResolvedRoute,
	stateDir string,
	logger domain.Logger,
) *DaemonSession {
	return &DaemonSession{
		ErrorQueue:  errorQueue,
		DeliveryLog: deliveryLog,
		Routes:      routes,
		StateDir:    stateDir,
		Logger:      logger,
	}
}

// EnqueueError delegates to ErrorQueue.Enqueue, encapsulating the error queue dependency.
func (s *DaemonSession) EnqueueError(name string, data []byte, meta domain.ErrorMetadata) error {
	return s.ErrorQueue.Enqueue(name, data, meta)
}

// ClaimPendingRetries delegates to ErrorQueue.ClaimPendingRetries.
func (s *DaemonSession) ClaimPendingRetries(claimerID string, maxRetries int) ([]domain.ErrorEntry, error) {
	return s.ErrorQueue.ClaimPendingRetries(claimerID, maxRetries)
}

// IncrementRetry delegates to ErrorQueue.IncrementRetry.
func (s *DaemonSession) IncrementRetry(name string, newError string) error {
	return s.ErrorQueue.IncrementRetry(name, newError)
}

// MarkResolved delegates to ErrorQueue.MarkResolved.
func (s *DaemonSession) MarkResolved(name string) error {
	return s.ErrorQueue.MarkResolved(name)
}

// HasErrorQueue reports whether an error queue is available.
func (s *DaemonSession) HasErrorQueue() bool {
	return s != nil && s.ErrorQueue != nil
}

// ErrorQueueStore returns the underlying ErrorQueueStore, or nil if unavailable.
func (s *DaemonSession) ErrorQueueStore() port.ErrorQueueStore {
	if s == nil {
		return nil
	}
	return s.ErrorQueue
}

// RecordDeliveryEvent records a delivery.completed event to the event store.
// Best-effort: errors are logged but do not fail the delivery.
func (s *DaemonSession) RecordDeliveryEvent(result *domain.DeliveryResult) {
	platform.RecordDelivery(context.Background(), "completed", result.Kind)
	if s.Emitter == nil {
		return
	}
	if err := s.Emitter.EmitDelivery(result.SourcePath, result.Kind, time.Now().UTC()); err != nil {
		s.Logger.Warn("emit delivery event: %v", err)
	}
}

// RecordFailureEvent records a delivery.failed event to the event store.
// Best-effort: errors are logged but do not fail the error recording.
func (s *DaemonSession) RecordFailureEvent(filePath string, kind string, deliverErr error) {
	platform.RecordDelivery(context.Background(), "failed", kind)
	if s.Emitter == nil {
		return
	}
	if err := s.Emitter.EmitFailure(filePath, kind, deliverErr.Error(), time.Now().UTC()); err != nil {
		s.Logger.Warn("emit failure event: %v", err)
	}
}

// RecordScanEvent records a scan.completed event to the event store.
func (s *DaemonSession) RecordScanEvent(outboxDir string, deliveredCount int, errorCount int) {
	if s.Emitter == nil {
		return
	}
	if err := s.Emitter.EmitScan(outboxDir, deliveredCount, errorCount, time.Now().UTC()); err != nil {
		s.Logger.Warn("emit scan event: %v", err)
	}
}

// RecordRetryEvent records an error.retried event to the event store.
func (s *DaemonSession) RecordRetryEvent(name string, kind string) {
	if s.Emitter == nil {
		return
	}
	if err := s.Emitter.EmitRetry(name, kind, time.Now().UTC()); err != nil {
		s.Logger.Warn("emit retry event: %v", err)
	}
}
