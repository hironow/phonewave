package domain_test

import (
	"testing"

	"github.com/hironow/phonewave/internal/domain"
)

func TestContentIdempotencyKey_Deterministic(t *testing.T) {
	// given
	data := []byte("---\nkind: specification\n---\nsome body content")

	// when
	key1 := domain.ContentIdempotencyKey(data)
	key2 := domain.ContentIdempotencyKey(data)

	// then
	if key1 != key2 {
		t.Errorf("expected deterministic key, got %s != %s", key1, key2)
	}
	if len(key1) != 64 {
		t.Errorf("expected 64 hex chars (SHA256), got %d", len(key1))
	}
}

func TestContentIdempotencyKey_DifferentContentDifferentKey(t *testing.T) {
	// given
	data1 := []byte("content A")
	data2 := []byte("content B")

	// when
	key1 := domain.ContentIdempotencyKey(data1)
	key2 := domain.ContentIdempotencyKey(data2)

	// then
	if key1 == key2 {
		t.Error("expected different keys for different content")
	}
}
