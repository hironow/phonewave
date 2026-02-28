package phonewave

// DeliveryMetrics holds delivery counts for success rate calculation.
type DeliveryMetrics struct {
	Delivered int
	Failed    int
	Retried   int
}

// SuccessRate calculates the delivery success rate.
// Retried deliveries count as successes (they eventually delivered).
// Returns 0.0 if there are no deliveries.
func (m DeliveryMetrics) SuccessRate() float64 {
	total := m.Delivered + m.Failed
	if total == 0 {
		return 0.0
	}
	return float64(m.Delivered) / float64(total)
}
