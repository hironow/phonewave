package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/platform"
)

// handleEvent processes a single fsnotify event.
func (d *Daemon) handleEvent(event fsnotify.Event) {
	if !shouldProcessEvent(event) {
		return
	}

	ctx, span := platform.Tracer.Start(context.Background(), "daemon.handle_event",
		trace.WithAttributes(
			attribute.String("event.name", platform.SanitizeUTF8(event.Name)),
			attribute.String("event.op", event.Op.String()), // nosemgrep: otel-attribute-string-unsanitized — fsnotify Op.String() returns Go constant, always valid UTF-8 [permanent]
		),
	)
	defer span.End()

	// Small delay to let the file be fully written
	time.Sleep(50 * time.Millisecond)

	if d.opts.DryRun {
		d.logger.Info("[dry-run] Detected %s", event.Name)
		return
	}

	// Read file content upfront (needed for error queue on failure).
	// Silently ignore ErrNotExist: Rename events fire for both the
	// source (gone) and target (arrived) paths.
	data, readErr := os.ReadFile(event.Name)
	if readErr != nil {
		if errors.Is(readErr, os.ErrNotExist) {
			return
		}
		d.logger.Error("Read %s: %v", event.Name, readErr)
		span.RecordError(readErr)
		span.SetStatus(codes.Error, readErr.Error())
		return
	}

	result, err := DeliverData(ctx, event.Name, data, d.opts.Routes, d.deliveryStore)
	if err != nil {
		kind, _ := domain.ExtractDMailKind(data)
		if kind == "" {
			kind = domain.UnknownKind
		}
		d.logger.Error("Deliver %s: %v", event.Name, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		if d.dlog != nil {
			d.dlog.Failed(kind, event.Name, err.Error())
		}
		d.recordFailureEvent(event.Name, kind, err)

		d.enqueueDeliveryFailure(event.Name, data, kind, err)
		return
	}

	if d.dlog != nil {
		for _, target := range result.DeliveredTo {
			d.dlog.Delivered(result.Kind, result.SourcePath, target)
		}
		// Only log REMOVED if source was actually removed (all targets flushed)
		if _, statErr := os.Stat(result.SourcePath); errors.Is(statErr, os.ErrNotExist) {
			d.dlog.Removed(result.SourcePath)
		}
	}
	d.recordDeliveryEvent(result)

	if d.opts.Verbose {
		d.logger.OK("Delivered %s (kind=%s) to %v", result.SourcePath, result.Kind, result.DeliveredTo)
	}
}

// enqueueDeliveryFailure moves a failed delivery from the outbox to the error
// queue. On successful enqueue the outbox file is removed. If the error queue
// is unavailable the file is left in the outbox for the next startup scan.
func (d *Daemon) enqueueDeliveryFailure(path string, data []byte, kind string, deliverErr error) {
	meta := domain.ErrorMetadata{
		SourceOutbox: filepath.Dir(path),
		Kind:         kind,
		OriginalName: filepath.Base(path),
		Attempts:     1,
		Error:        deliverErr.Error(),
		Timestamp:    time.Now().UTC(),
	}
	if !d.hasErrorQueue() {
		d.logger.Error("Error queue unavailable, leaving in outbox for next startup")
		return
	}
	name := fmt.Sprintf("%s-%s-%s", meta.Timestamp.Format("2006-01-02T150405.000000000"), meta.Kind, meta.OriginalName)
	if saveErr := d.enqueueError(name, data, meta); saveErr != nil {
		d.logger.Error("Error queue enqueue: %v", saveErr)
		return
	}
	// Error queue write succeeded — safe to remove from outbox
	os.Remove(path)
}

// retryPending claims pending error queue entries via SQLite and attempts
// to re-deliver them. Returns the number of successful retries.
func (d *Daemon) retryPending() int {
	if !d.hasErrorQueue() {
		return 0
	}

	ctx, retrySpan := platform.Tracer.Start(context.Background(), "daemon.retry_pending")
	defer retrySpan.End()

	maxRetries := d.opts.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 10
	}

	claimerID := fmt.Sprintf("daemon-%d", os.Getpid())
	entries, err := d.claimPendingRetries(claimerID, maxRetries)
	if err != nil {
		d.logger.Error("Retry: claim pending: %v", err)
		return 0
	}

	if len(entries) == 0 {
		return 0
	}

	successCh := make(chan struct{}, len(entries))

	retryGroup := d.pool.NewGroup()
	for _, e := range entries {
		retryGroup.Submit(func() {
			originalPath := filepath.Join(e.SourceOutbox, e.OriginalName)
			result, deliverErr := DeliverData(ctx, originalPath, e.Data, d.opts.Routes, d.deliveryStore)
			if deliverErr != nil {
				if incErr := d.incrementRetry(e.Name, deliverErr.Error()); incErr != nil {
					d.logger.Warn("Retry: increment retry: %v", incErr)
				}
				if d.opts.Verbose {
					d.logger.Warn("Retry failed for %s (attempt %d): %v", e.OriginalName, e.RetryCount+1, deliverErr)
				}
				return
			}

			if markErr := d.markResolved(e.Name); markErr != nil {
				d.logger.Warn("Retry: mark resolved: %v", markErr)
			}

			if d.dlog != nil {
				for _, target := range result.DeliveredTo {
					d.dlog.Retried(result.Kind, originalPath, target)
				}
			}
			d.recordRetryEvent(e.OriginalName, result.Kind)

			if d.opts.Verbose {
				d.logger.OK("Retry: delivered %s (kind=%s) to %v", e.OriginalName, result.Kind, result.DeliveredTo)
			}

			successCh <- struct{}{}
		})
	}
	retryGroup.Wait()
	close(successCh)

	successes := 0
	for range successCh {
		successes++
	}
	return successes
}
