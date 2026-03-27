package domain

import (
	"encoding/json"
	"fmt"
	"time"
)

// DeliveryMetrics holds delivery counts for success rate calculation.
type DeliveryMetrics struct {
	Delivered int `json:"delivered"`
	Failed    int `json:"failed"`
	Retried   int `json:"retried"`
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

// FormatSuccessRate formats a delivery success rate as a human-readable string.
// Returns "no deliveries" when total is 0.
func FormatSuccessRate(rate float64, success, total int) string {
	if total == 0 {
		return "no deliveries"
	}
	return fmt.Sprintf("%.1f%% (%d/%d)", rate*100, success, total)
}

// BucketMetrics holds aggregated delivery metrics for a single time bucket.
type BucketMetrics struct {
	Start        time.Time `json:"start"`
	Delivered    int       `json:"delivered"`
	Failed       int       `json:"failed"`
	Retried      int       `json:"retried"`
	SuccessRate  float64   `json:"success_rate"`
	AvgLatencyMs *int64    `json:"avg_latency_ms,omitempty"`
}

// HealthTimeSeries is the READ MODEL for delivery health over a time window.
type HealthTimeSeries struct {
	GeneratedAt time.Time       `json:"generated_at"`
	Window      string          `json:"window"`
	BucketSize  string          `json:"bucket_size"`
	Buckets     []BucketMetrics `json:"buckets"`
	Totals      DeliveryMetrics `json:"totals"`
}

// AggregateHealthTimeSeries projects delivery events into a bucketed time series.
// Pure function: no I/O, deterministic, testable in isolation.
func AggregateHealthTimeSeries(events []Event, windowStart time.Time, bucketSize time.Duration, now time.Time) HealthTimeSeries {
	numBuckets := int(now.Sub(windowStart) / bucketSize)
	if now.Sub(windowStart)%bucketSize > 0 {
		numBuckets++ // partial last bucket
	}
	if numBuckets <= 0 {
		numBuckets = 1
	}

	type accumulator struct {
		delivered    int
		failed       int
		retried      int
		totalLatency int64
		latencyCount int
	}
	buckets := make([]accumulator, numBuckets)

	var totals DeliveryMetrics
	for _, ev := range events {
		if ev.Timestamp.Before(windowStart) {
			continue
		}
		idx := int(ev.Timestamp.Sub(windowStart) / bucketSize)
		if idx >= numBuckets {
			idx = numBuckets - 1
		}

		switch ev.Type {
		case EventDeliveryCompleted:
			buckets[idx].delivered++
			totals.Delivered++
			// Extract latency if present in event data
			if len(ev.Data) > 0 {
				var data struct {
					LatencyMs int64 `json:"latency_ms"`
				}
				if json.Unmarshal(ev.Data, &data) == nil && data.LatencyMs > 0 {
					buckets[idx].totalLatency += data.LatencyMs
					buckets[idx].latencyCount++
				}
			}
		case EventDeliveryFailed:
			buckets[idx].failed++
			totals.Failed++
		case EventErrorRetried:
			buckets[idx].retried++
			totals.Retried++
		}
	}

	result := HealthTimeSeries{
		GeneratedAt: now,
		Window:      fmt.Sprintf("%s", bucketSize*time.Duration(numBuckets)),
		BucketSize:  bucketSize.String(),
		Totals:      totals,
	}

	for i, acc := range buckets {
		bm := BucketMetrics{
			Start:     windowStart.Add(time.Duration(i) * bucketSize),
			Delivered: acc.delivered,
			Failed:    acc.failed,
			Retried:   acc.retried,
		}
		total := acc.delivered + acc.failed
		if total > 0 {
			bm.SuccessRate = float64(acc.delivered) / float64(total)
		}
		if acc.latencyCount > 0 {
			avg := acc.totalLatency / int64(acc.latencyCount)
			bm.AvgLatencyMs = &avg
		}
		result.Buckets = append(result.Buckets, bm)
	}

	return result
}
