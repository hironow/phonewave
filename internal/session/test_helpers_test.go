package session

import "testing"

// newTestDeliveryStore creates a temporary SQLiteDeliveryStore for tests.
func newTestDeliveryStore(t *testing.T) *SQLiteDeliveryStore {
	t.Helper()
	stateDir := t.TempDir()
	ds, err := NewSQLiteDeliveryStore(stateDir)
	if err != nil {
		t.Fatalf("NewSQLiteDeliveryStore: %v", err)
	}
	t.Cleanup(func() { ds.Close() })
	return ds
}
