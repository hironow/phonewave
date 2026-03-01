package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	phonewave "github.com/hironow/phonewave"
)

// Deliver reads a D-Mail file and delivers it to all matching inboxes.
func Deliver(ctx context.Context, dmailPath string, routes []phonewave.ResolvedRoute, ds phonewave.DeliveryStore) (*phonewave.DeliveryResult, error) {
	data, err := os.ReadFile(dmailPath)
	if err != nil {
		return nil, fmt.Errorf("read D-Mail: %w", err)
	}
	return DeliverData(ctx, dmailPath, data, routes, ds)
}

// DeliverData processes pre-read D-Mail data via Stage→Flush transactional delivery.
// Returns error only for parse/route/stage failures (error queue eligible).
// Flush partial failures are handled internally by DeliveryStore retry_count.
func DeliverData(ctx context.Context, dmailPath string, data []byte, routes []phonewave.ResolvedRoute, ds phonewave.DeliveryStore) (*phonewave.DeliveryResult, error) {
	kind, err := phonewave.ExtractDMailKind(data)
	if err != nil {
		return nil, fmt.Errorf("parse D-Mail %s: %w", dmailPath, err)
	}

	ctx, span := phonewave.Tracer.Start(ctx, "delivery.deliver",
		trace.WithAttributes(
			attribute.String("dmail.path", dmailPath),
			attribute.String("dmail.kind", kind),
		),
	)
	defer span.End()

	// Find matching route
	sourceDir := filepath.Dir(dmailPath)
	var matchedRoute *phonewave.ResolvedRoute
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
	result := &phonewave.DeliveryResult{
		SourcePath: dmailPath,
		Kind:       kind,
	}

	// Stage delivery intent (transactional, dmailPath = full path for uniqueness)
	targetPaths := make([]string, len(matchedRoute.ToInboxes))
	for i, inbox := range matchedRoute.ToInboxes {
		targetPaths[i] = filepath.Join(inbox, fileName)
	}
	if err := ds.StageDelivery(dmailPath, data, targetPaths); err != nil {
		stageErr := fmt.Errorf("stage delivery %s: %w", dmailPath, err)
		span.RecordError(stageErr)
		span.SetStatus(codes.Error, stageErr.Error())
		return nil, stageErr
	}

	// Flush all staged items (2-phase: SELECT → atomicWrite → UPDATE)
	flushed, flushErr := ds.FlushDeliveries()
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

	// Remove source only when ALL targets are flushed
	allDone, _ := ds.AllFlushedFor(dmailPath)
	if allDone {
		if err := os.Remove(dmailPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			removeErr := fmt.Errorf("remove source %s: %w", dmailPath, err)
			span.RecordError(removeErr)
			span.SetStatus(codes.Error, removeErr.Error())
			return result, removeErr
		}
	}
	// Partial: source stays in outbox, no error returned.
	// DeliveryStore retry_count handles re-flush on next delivery or startup scan.

	span.SetAttributes(attribute.Int("inbox.count", len(result.DeliveredTo)))
	return result, nil
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
