package phonewave

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
