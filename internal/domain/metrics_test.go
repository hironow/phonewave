package domain

import "testing"

func TestDeliveryMetrics_SuccessRate_AllDelivered(t *testing.T) {
	m := DeliveryMetrics{Delivered: 10, Failed: 0}

	if rate := m.SuccessRate(); rate != 1.0 {
		t.Errorf("SuccessRate = %f, want 1.0", rate)
	}
}

func TestDeliveryMetrics_SuccessRate_AllFailed(t *testing.T) {
	m := DeliveryMetrics{Delivered: 0, Failed: 5}

	if rate := m.SuccessRate(); rate != 0.0 {
		t.Errorf("SuccessRate = %f, want 0.0", rate)
	}
}

func TestDeliveryMetrics_SuccessRate_Mixed(t *testing.T) {
	m := DeliveryMetrics{Delivered: 7, Failed: 3}

	if rate := m.SuccessRate(); rate != 0.7 {
		t.Errorf("SuccessRate = %f, want 0.7", rate)
	}
}

func TestDeliveryMetrics_SuccessRate_NoEvents(t *testing.T) {
	m := DeliveryMetrics{}

	if rate := m.SuccessRate(); rate != 0.0 {
		t.Errorf("SuccessRate = %f, want 0.0", rate)
	}
}

func TestDeliveryMetrics_SuccessRate_RetriedCountsAsSuccess(t *testing.T) {
	// Retried doesn't affect the ratio (they become Delivered)
	m := DeliveryMetrics{Delivered: 8, Failed: 2, Retried: 3}

	if rate := m.SuccessRate(); rate != 0.8 {
		t.Errorf("SuccessRate = %f, want 0.8", rate)
	}
}

func TestFormatSuccessRate_WithEvents(t *testing.T) {
	// given
	rate := 0.857142
	success := 6
	total := 7

	// when
	msg := FormatSuccessRate(rate, success, total)

	// then
	if msg != "85.7% (6/7)" {
		t.Errorf("FormatSuccessRate = %q, want %q", msg, "85.7% (6/7)")
	}
}

func TestFormatSuccessRate_NoDeliveries(t *testing.T) {
	// given
	rate := 0.0
	success := 0
	total := 0

	// when
	msg := FormatSuccessRate(rate, success, total)

	// then
	if msg != "no deliveries" {
		t.Errorf("FormatSuccessRate = %q, want %q", msg, "no deliveries")
	}
}

func TestFormatSuccessRate_Perfect(t *testing.T) {
	// given
	rate := 1.0
	success := 10
	total := 10

	// when
	msg := FormatSuccessRate(rate, success, total)

	// then
	if msg != "100.0% (10/10)" {
		t.Errorf("FormatSuccessRate = %q, want %q", msg, "100.0% (10/10)")
	}
}
