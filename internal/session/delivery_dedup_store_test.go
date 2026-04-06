package session_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/hironow/phonewave/internal/session"
)

func TestSQLiteDeliveryDedupStore_RecordAndCheck(t *testing.T) {
	// given
	stateDir := t.TempDir()
	dbPath := filepath.Join(stateDir, ".run", "delivery_dedup.db")
	store, err := session.NewSQLiteDeliveryDedupStore(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	key := "abc123def456"

	// when — check before recording
	has, err := store.HasDelivered(ctx, key)

	// then
	if err != nil {
		t.Fatalf("HasDelivered: %v", err)
	}
	if has {
		t.Error("expected not delivered before recording")
	}

	// when — record delivery
	err = store.RecordDelivery(ctx, key, "/inbox/sightjack")
	if err != nil {
		t.Fatalf("RecordDelivery: %v", err)
	}

	// when — check after recording
	has, err = store.HasDelivered(ctx, key)
	if err != nil {
		t.Fatalf("HasDelivered after record: %v", err)
	}

	// then
	if !has {
		t.Error("expected delivered after recording")
	}
}

func TestSQLiteDeliveryDedupStore_DuplicateRecordIgnored(t *testing.T) {
	// given
	stateDir := t.TempDir()
	dbPath := filepath.Join(stateDir, ".run", "delivery_dedup.db")
	store, err := session.NewSQLiteDeliveryDedupStore(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	key := "same-key"

	// when — record twice (should not error)
	_ = store.RecordDelivery(ctx, key, "/inbox/a")
	err = store.RecordDelivery(ctx, key, "/inbox/b")

	// then — no error on duplicate
	if err != nil {
		t.Fatalf("duplicate RecordDelivery should not error: %v", err)
	}
}
