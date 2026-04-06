package session

import (
	"errors"
	"os"
	"strings"

	"github.com/hironow/phonewave/internal/domain"
)

// ClassifyDeliveryError maps a delivery error to a DeliveryErrorInfo
// for the circuit breaker. Lives in session (not domain) because it
// inspects os.Err* types.
func ClassifyDeliveryError(err error) domain.DeliveryErrorInfo {
	if err == nil {
		return domain.DeliveryErrorInfo{Kind: domain.DeliveryErrorNone}
	}

	errMsg := err.Error()

	// Persistent errors: malformed content, not retryable
	if strings.Contains(errMsg, "parse D-Mail") ||
		strings.Contains(errMsg, "unsupported dmail-schema-version") ||
		strings.Contains(errMsg, "missing required") {
		return domain.DeliveryErrorInfo{
			Kind: domain.DeliveryErrorPersistent,
			Err:  err,
		}
	}

	// Transient filesystem errors
	if errors.Is(err, os.ErrPermission) ||
		errors.Is(err, os.ErrNotExist) ||
		strings.Contains(errMsg, "no space left") ||
		strings.Contains(errMsg, "read-only file system") {
		return domain.DeliveryErrorInfo{
			Kind: domain.DeliveryErrorTransient,
			Err:  err,
		}
	}

	// Default: treat as transient (safe to trip CB for unknown errors)
	return domain.DeliveryErrorInfo{
		Kind: domain.DeliveryErrorTransient,
		Err:  err,
	}
}
