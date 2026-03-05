package session

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/port"
)

// ScanAndDeliver processes all existing .md files in the given outbox directory,
// delivering each one according to the provided routes. Files are delivered
// sequentially. Failed deliveries are enqueued via errorQueue (SQLite).
// If errorQueue is nil, failed files remain in the outbox for next startup.
func ScanAndDeliver(ctx context.Context, outboxDir string, routes []domain.ResolvedRoute, stateDir string, logger domain.Logger, ds port.DeliveryStore, errorQueue port.ErrorQueueStore) ([]*domain.DeliveryResult, []error) {
	if logger == nil {
		logger = &domain.NopLogger{}
	}
	entries, err := os.ReadDir(outboxDir)
	if err != nil {
		return nil, []error{fmt.Errorf("scan outbox %s: %w", outboxDir, err)}
	}

	// Filter eligible entries using domain.IsDMailFile
	var filtered []os.DirEntry
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !domain.IsDMailFile(entry.Name()) {
			continue
		}
		filtered = append(filtered, entry)
	}

	if len(filtered) == 0 {
		return nil, nil
	}

	// Deliver files sequentially. The caller (daemon startup scan) already
	// parallelises per-outbox via its worker pool, so a nested pool here
	// would multiply concurrency to NumCPU² and spike FD/memory usage.
	var results []*domain.DeliveryResult
	var errs []error
	for _, entry := range filtered {
		dmailPath := filepath.Join(outboxDir, entry.Name())

		data, readErr := os.ReadFile(dmailPath)
		if readErr != nil {
			errs = append(errs, fmt.Errorf("read %s: %w", dmailPath, readErr))
			continue
		}

		result, deliverErr := DeliverData(ctx, dmailPath, data, routes, ds)
		if deliverErr != nil {
			kind, _ := domain.ExtractDMailKind(data)
			if kind == "" {
				kind = "unknown"
			}
			meta := domain.ErrorMetadata{
				SourceOutbox: outboxDir,
				Kind:         kind,
				OriginalName: entry.Name(),
				Attempts:     1,
				Error:        deliverErr.Error(),
				Timestamp:    time.Now().UTC(),
			}
			if errorQueue == nil {
				logger.Error("Error queue unavailable, leaving %s in outbox", dmailPath)
			} else {
				name := fmt.Sprintf("%s-%s-%s", meta.Timestamp.Format("2006-01-02T150405.000000000"), meta.Kind, meta.OriginalName)
				if saveErr := errorQueue.Enqueue(name, data, meta); saveErr != nil {
					logger.Error("Error queue enqueue: %v", saveErr)
				} else {
					os.Remove(dmailPath)
				}
			}
			errs = append(errs, fmt.Errorf("deliver %s: %w", dmailPath, deliverErr))
			continue
		}

		results = append(results, result)
	}

	return results, errs
}
