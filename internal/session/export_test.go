package session

import "database/sql"

// DBForTest returns the underlying database connection for testing.
// Only available in test builds.
func (s *SQLiteErrorStore) DBForTest() *sql.DB { return s.db }
