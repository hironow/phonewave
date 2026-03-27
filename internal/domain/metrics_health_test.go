package domain

import (
	"encoding/json"
	"testing"
	"time"
)

func TestAggregateHealthTimeSeries_Empty(t *testing.T) {
	now := time.Date(2026, 3, 27, 12, 0, 0, 0, time.UTC)
	start := now.Add(-24 * time.Hour)
	ts := AggregateHealthTimeSeries(nil, start, time.Hour, now)
	if ts.Totals.Delivered != 0 || ts.Totals.Failed != 0 {
		t.Errorf("expected zero totals, got %+v", ts.Totals)
	}
	if len(ts.Buckets) == 0 {
		t.Error("expected at least 1 bucket")
	}
}

func TestAggregateHealthTimeSeries_Counts(t *testing.T) {
	now := time.Date(2026, 3, 27, 12, 0, 0, 0, time.UTC)
	start := now.Add(-2 * time.Hour)

	events := []Event{
		{Type: EventDeliveryCompleted, Timestamp: start.Add(30 * time.Minute)},
		{Type: EventDeliveryCompleted, Timestamp: start.Add(45 * time.Minute)},
		{Type: EventDeliveryFailed, Timestamp: start.Add(50 * time.Minute)},
		{Type: EventDeliveryCompleted, Timestamp: start.Add(90 * time.Minute)},
		{Type: EventErrorRetried, Timestamp: start.Add(95 * time.Minute)},
	}

	ts := AggregateHealthTimeSeries(events, start, time.Hour, now)

	if ts.Totals.Delivered != 3 {
		t.Errorf("totals.Delivered = %d, want 3", ts.Totals.Delivered)
	}
	if ts.Totals.Failed != 1 {
		t.Errorf("totals.Failed = %d, want 1", ts.Totals.Failed)
	}
	if ts.Totals.Retried != 1 {
		t.Errorf("totals.Retried = %d, want 1", ts.Totals.Retried)
	}

	// Bucket 0 (0-1h): 2 delivered, 1 failed
	if ts.Buckets[0].Delivered != 2 || ts.Buckets[0].Failed != 1 {
		t.Errorf("bucket[0] = %+v", ts.Buckets[0])
	}
	// Bucket 1 (1-2h): 1 delivered, 0 failed, 1 retried
	if ts.Buckets[1].Delivered != 1 || ts.Buckets[1].Retried != 1 {
		t.Errorf("bucket[1] = %+v", ts.Buckets[1])
	}
}

func TestAggregateHealthTimeSeries_SuccessRate(t *testing.T) {
	now := time.Date(2026, 3, 27, 12, 0, 0, 0, time.UTC)
	start := now.Add(-time.Hour)

	events := []Event{
		{Type: EventDeliveryCompleted, Timestamp: start.Add(10 * time.Minute)},
		{Type: EventDeliveryCompleted, Timestamp: start.Add(20 * time.Minute)},
		{Type: EventDeliveryCompleted, Timestamp: start.Add(30 * time.Minute)},
		{Type: EventDeliveryFailed, Timestamp: start.Add(40 * time.Minute)},
	}

	ts := AggregateHealthTimeSeries(events, start, time.Hour, now)
	// 3/4 = 0.75
	if ts.Buckets[0].SuccessRate != 0.75 {
		t.Errorf("success rate = %f, want 0.75", ts.Buckets[0].SuccessRate)
	}
}

func TestAggregateHealthTimeSeries_Latency(t *testing.T) {
	now := time.Date(2026, 3, 27, 12, 0, 0, 0, time.UTC)
	start := now.Add(-time.Hour)

	data1, _ := json.Marshal(map[string]int64{"latency_ms": 100})
	data2, _ := json.Marshal(map[string]int64{"latency_ms": 200})

	events := []Event{
		{Type: EventDeliveryCompleted, Timestamp: start.Add(10 * time.Minute), Data: data1},
		{Type: EventDeliveryCompleted, Timestamp: start.Add(20 * time.Minute), Data: data2},
	}

	ts := AggregateHealthTimeSeries(events, start, time.Hour, now)
	if ts.Buckets[0].AvgLatencyMs == nil {
		t.Fatal("expected latency")
	}
	if *ts.Buckets[0].AvgLatencyMs != 150 {
		t.Errorf("avg latency = %d, want 150", *ts.Buckets[0].AvgLatencyMs)
	}
}

func TestAggregateHealthTimeSeries_SkipsOldEvents(t *testing.T) {
	now := time.Date(2026, 3, 27, 12, 0, 0, 0, time.UTC)
	start := now.Add(-time.Hour)

	events := []Event{
		{Type: EventDeliveryCompleted, Timestamp: start.Add(-10 * time.Minute)}, // before window
		{Type: EventDeliveryCompleted, Timestamp: start.Add(10 * time.Minute)},
	}

	ts := AggregateHealthTimeSeries(events, start, time.Hour, now)
	if ts.Totals.Delivered != 1 {
		t.Errorf("should skip old event, delivered = %d, want 1", ts.Totals.Delivered)
	}
}
