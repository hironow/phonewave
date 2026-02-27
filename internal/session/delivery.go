package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	phonewave "github.com/hironow/phonewave"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Deliver reads a D-Mail file and delivers it to all matching inboxes.
func Deliver(ctx context.Context, dmailPath string, routes []phonewave.ResolvedRoute) (*phonewave.DeliveryResult, error) {
	data, err := os.ReadFile(dmailPath)
	if err != nil {
		return nil, fmt.Errorf("read D-Mail: %w", err)
	}
	return DeliverData(ctx, dmailPath, data, routes)
}

// DeliverData processes pre-read D-Mail data: routes by kind,
// copies to all target inboxes atomically, then removes the source.
func DeliverData(ctx context.Context, dmailPath string, data []byte, routes []phonewave.ResolvedRoute) (*phonewave.DeliveryResult, error) {
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

	// Copy to all target inboxes (atomic: write temp -> rename).
	// On failure, roll back already-written files to prevent duplicates on retry.
	for _, inbox := range matchedRoute.ToInboxes {
		targetPath := filepath.Join(inbox, fileName)
		if err := atomicWrite(targetPath, data); err != nil {
			// Roll back all previously written inbox files
			for _, written := range result.DeliveredTo {
				os.Remove(written)
			}
			result.DeliveredTo = nil
			deliverErr := fmt.Errorf("deliver to %s: %w", inbox, err)
			span.RecordError(deliverErr)
			span.SetStatus(codes.Error, deliverErr.Error())
			return result, deliverErr
		}
		result.DeliveredTo = append(result.DeliveredTo, targetPath)
	}

	// Remove source only after all deliveries succeed (at-least-once).
	// Ignore ErrNotExist: the source may already have been cleaned up
	// (e.g. retry delivery from error queue).
	if err := os.Remove(dmailPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		removeErr := fmt.Errorf("remove source %s: %w", dmailPath, err)
		span.RecordError(removeErr)
		span.SetStatus(codes.Error, removeErr.Error())
		return result, removeErr
	}

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
