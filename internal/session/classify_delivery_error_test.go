package session_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/session"
)

func TestClassifyDeliveryError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantKind domain.DeliveryErrorKind
		wantTrip bool
	}{
		{
			name:     "nil error",
			err:      nil,
			wantKind: domain.DeliveryErrorNone,
			wantTrip: false,
		},
		{
			name:     "permission denied is transient",
			err:      fmt.Errorf("write inbox: %w", os.ErrPermission),
			wantKind: domain.DeliveryErrorTransient,
			wantTrip: true,
		},
		{
			name:     "not exist is transient",
			err:      fmt.Errorf("open target: %w", os.ErrNotExist),
			wantKind: domain.DeliveryErrorTransient,
			wantTrip: true,
		},
		{
			name:     "parse D-Mail failure is persistent",
			err:      fmt.Errorf("parse D-Mail: missing frontmatter"),
			wantKind: domain.DeliveryErrorPersistent,
			wantTrip: false,
		},
		{
			name:     "unknown error defaults to transient",
			err:      fmt.Errorf("some unexpected error"),
			wantKind: domain.DeliveryErrorTransient,
			wantTrip: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			info := session.ClassifyDeliveryError(tt.err)

			// then
			if info.Kind != tt.wantKind {
				t.Errorf("Kind = %v, want %v", info.Kind, tt.wantKind)
			}
			if info.IsTrip() != tt.wantTrip {
				t.Errorf("IsTrip() = %v, want %v", info.IsTrip(), tt.wantTrip)
			}
		})
	}
}
