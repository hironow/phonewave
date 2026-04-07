package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/harness"
	"github.com/hironow/phonewave/internal/platform"
	"github.com/hironow/phonewave/internal/usecase/port"
)

// Deliver reads a D-Mail file and delivers it to all matching inboxes.
func Deliver(ctx context.Context, dmailPath string, routes []domain.ResolvedRoute, ds port.DeliveryStore) (*domain.DeliveryResult, error) {
	data, err := os.ReadFile(dmailPath)
	if err != nil {
		return nil, fmt.Errorf("read D-Mail: %w", err)
	}
	return DeliverData(ctx, dmailPath, data, routes, ds, nil)
}

// DeliverData processes pre-read D-Mail data via Stage→Flush transactional delivery.
// Returns error only for parse/route/stage failures (error queue eligible).
// Flush partial failures are handled internally by DeliveryStore retry_count.
// If dedup is non-nil, per-target exact-match dedup is applied: already-delivered
// targets are skipped, and newly delivered targets are recorded.
func DeliverData(ctx context.Context, dmailPath string, data []byte, routes []domain.ResolvedRoute, ds port.DeliveryStore, dedup port.DeliveryDedupStore) (*domain.DeliveryResult, error) {
	fm, err := domain.ParseDMailFrontmatter(data)
	if err != nil {
		return nil, fmt.Errorf("parse D-Mail %s: %w", dmailPath, err)
	}
	kind := fm.Kind
	if fm.SchemaVersion == "" {
		return nil, fmt.Errorf("parse D-Mail %s: D-Mail missing required 'dmail-schema-version' field", dmailPath)
	}
	if fm.SchemaVersion != domain.SupportedDMailSchemaVersion {
		return nil, fmt.Errorf("parse D-Mail %s: unsupported dmail-schema-version %q: only %q is supported", dmailPath, fm.SchemaVersion, domain.SupportedDMailSchemaVersion)
	}
	if kind == "" {
		return nil, fmt.Errorf("parse D-Mail %s: D-Mail missing required 'kind' field", dmailPath)
	}
	if err := domain.ValidateKind(kind); err != nil {
		return nil, fmt.Errorf("parse D-Mail %s: %w", dmailPath, err)
	}
	metadata := domain.CorrectionMetadataFromMap(fm.Metadata)

	ctx, span := platform.Tracer.Start(ctx, "delivery.deliver",
		trace.WithAttributes(
			attribute.String("dmail.path", platform.SanitizeUTF8(dmailPath)),
			attribute.String("dmail.kind", platform.SanitizeUTF8(string(kind))),
		),
	)
	defer span.End()

	// Find matching route
	sourceDir := filepath.Dir(dmailPath)
	var matchedRoute *domain.ResolvedRoute
	for i := range routes {
		if routes[i].Kind == kind && routes[i].FromOutbox == sourceDir {
			matchedRoute = &routes[i]
			break
		}
	}
	if matchedRoute == nil {
		err := fmt.Errorf("no route for kind=%q from %s", kind, sourceDir)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	fileName := filepath.Base(dmailPath)
	result := &domain.DeliveryResult{
		SourcePath: dmailPath,
		Kind: kind,
	}

	// Stage delivery intent (transactional, dmailPath = full path for uniqueness)
	targetInboxes := harness.SelectDeliveryInboxes(string(kind), matchedRoute.ToInboxes, fm.Targets, metadata)
	idempotencyKey := domain.ContentIdempotencyKey(data)

	// Per-target exact dedup: filter out already-delivered logical targets (inbox dirs).
	// Key = content hash (idempotency_key), target = inbox directory (not file path).
	// This ensures content-based dedup regardless of filename changes.
	// Dedup is on the correctness path — read failures abort delivery (fail-closed)
	// so the file stays in the outbox for retry via error queue or next scan.
	if dedup != nil {
		var remaining []string
		for _, inbox := range targetInboxes {
			delivered, err := dedup.HasDelivered(ctx, idempotencyKey, inbox)
			if err != nil {
				dedupErr := fmt.Errorf("dedup read %s: %w", inbox, err)
				span.RecordError(dedupErr)
				span.SetStatus(codes.Error, dedupErr.Error())
				return nil, dedupErr
			}
			if !delivered {
				remaining = append(remaining, inbox)
			}
		}
		if len(remaining) == 0 {
			// All targets already delivered — clean up source and skip
			if removeErr := os.Remove(dmailPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				span.RecordError(fmt.Errorf("dedup cleanup source %s: %w", dmailPath, removeErr))
			}
			return result, nil
		}
		targetInboxes = remaining
	}

	targetPaths := make([]string, len(targetInboxes))
	for i, inbox := range targetInboxes {
		targetPaths[i] = filepath.Join(inbox, fileName)
	}

	// StageDelivery only inserts/updates rows where flushed=0.
	// Already-flushed rows (from a previous successful delivery) are preserved,
	// preventing duplicate delivery when only the dedup record needs retry.
	if err := ds.StageDelivery(ctx, dmailPath, data, targetPaths); err != nil {
		stageErr := fmt.Errorf("stage delivery %s: %w", dmailPath, err)
		span.RecordError(stageErr)
		span.SetStatus(codes.Error, stageErr.Error())
		return nil, stageErr
	}

	// Flush all staged items (2-phase: SELECT → atomicWrite → UPDATE)
	flushed, flushErr := ds.FlushDeliveries(ctx)
	if flushErr != nil {
		span.RecordError(flushErr)
		// Non-fatal: partial results may have been flushed
	}

	// Build DeliveredTo from flushed items for this dmailPath
	for _, f := range flushed {
		if f.DMailPath == dmailPath {
			result.DeliveredTo = append(result.DeliveredTo, f.Target)
		}
	}

	// Record dedup only for targets that were actually flushed (result.DeliveredTo).
	// Using targetInboxes here would record dedup for unflushed targets in a
	// partial flush, causing HasDelivered=true for targets that were never delivered.
	var dedupRecordFailed bool
	if dedup != nil {
		flushedInboxes := make([]string, len(result.DeliveredTo))
		for i, target := range result.DeliveredTo {
			flushedInboxes[i] = filepath.Dir(target)
		}
		allRecorded := recordDedupForTargets(ctx, span, dedup, idempotencyKey, flushedInboxes)
		dedupRecordFailed = !allRecorded
	}

	// Remove source only when ALL targets are flushed AND all dedup records written.
	// When dedupRecordFailed, source stays in outbox — next scan enters the
	// allPreviouslyFlushed fast path and retries RecordDelivery without re-staging.
	allDone, _ := ds.AllFlushedFor(dmailPath)
	if allDone && !dedupRecordFailed {
		if err := os.Remove(dmailPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			removeErr := fmt.Errorf("remove source %s: %w", dmailPath, err)
			span.RecordError(removeErr)
			span.SetStatus(codes.Error, removeErr.Error())
			return result, removeErr
		}
	}
	// Partial: source stays in outbox, no error returned.
	// Next scan: allPreviouslyFlushed fast path retries dedup record only.

	attrs := []attribute.KeyValue{
		attribute.Int("inbox.count", len(result.DeliveredTo)),
		attribute.String("dmail.failure_type", platform.SanitizeUTF8(string(metadata.FailureType))),
		attribute.String("dmail.severity", platform.SanitizeUTF8(string(domain.NormalizeSeverity(metadata.Severity)))),
		attribute.String("dmail.target_agent", platform.SanitizeUTF8(metadata.TargetAgent)),
		attribute.String("dmail.routing_mode", platform.SanitizeUTF8(string(domain.NormalizeRoutingMode(metadata.RoutingMode)))),
		attribute.String("dmail.routing_history", platform.SanitizeUTF8(domain.FormatImprovementHistory(metadata.RoutingHistory))),
		attribute.String("dmail.owner_history", platform.SanitizeUTF8(domain.FormatImprovementHistory(metadata.OwnerHistory))),
		attribute.String("dmail.correlation_id", platform.SanitizeUTF8(metadata.CorrelationID)),
		attribute.String("dmail.trace_id", platform.SanitizeUTF8(metadata.TraceID)),
		attribute.String("dmail.outcome", platform.SanitizeUTF8(string(metadata.Outcome))),
		attribute.Int("dmail.recurrence_count", metadata.RecurrenceCount),
		attribute.String("dmail.improvement_schema_version", platform.SanitizeUTF8(metadata.SchemaVersion)),
	}
	if metadata.RetryAllowed != nil {
		attrs = append(attrs, attribute.String("dmail.retry_allowed", platform.SanitizeUTF8(strconv.FormatBool(*metadata.RetryAllowed))))
	}
	if metadata.EscalationReason != "" {
		attrs = append(attrs, attribute.String("dmail.escalation_reason", platform.SanitizeUTF8(metadata.EscalationReason)))
	}
	span.SetAttributes(attrs...)
	return result, nil
}

// recordDedupForTargets writes dedup records for all target inboxes.
// Returns true if all records were written successfully.
// On failure, errors are recorded on the span and the function continues
// to attempt remaining targets (partial records are better than none).
func recordDedupForTargets(ctx context.Context, span trace.Span, dedup port.DeliveryDedupStore, idempotencyKey string, targetInboxes []string) bool {
	allOK := true
	for _, inbox := range targetInboxes {
		if recErr := dedup.RecordDelivery(ctx, idempotencyKey, inbox); recErr != nil {
			span.RecordError(fmt.Errorf("dedup record %s→%s: %w", idempotencyKey[:8], inbox, recErr))
			allOK = false
		}
	}
	if !allOK {
		span.SetAttributes(attribute.Bool("dedup.record_incomplete", true))
	}
	return allOK
}

// atomicWrite writes data to a temporary file in the same directory,
// then renames it to the target path (atomic on same filesystem).
func atomicWrite(targetPath string, data []byte) error {
	dir := filepath.Dir(targetPath)
	tmp, err := os.CreateTemp(dir, ".phonewave-tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}

	return os.Rename(tmpPath, targetPath)
}
