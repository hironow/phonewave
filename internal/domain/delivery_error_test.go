package domain_test

import (
	"errors"
	"testing"

	"github.com/hironow/phonewave/internal/domain"
)

func TestDeliveryErrorInfo_IsTrip(t *testing.T) {
	tests := []struct {
		name string
		kind domain.DeliveryErrorKind
		want bool
	}{
		{"none does not trip", domain.DeliveryErrorNone, false},
		{"transient trips", domain.DeliveryErrorTransient, true},
		{"persistent does not trip", domain.DeliveryErrorPersistent, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := domain.DeliveryErrorInfo{
				Kind: tt.kind,
				Err:  errors.New("test"),
			}
			if got := info.IsTrip(); got != tt.want {
				t.Errorf("IsTrip() = %v, want %v", got, tt.want)
			}
		})
	}
}
