package phonewave

import "time"

// ConfigLoader loads and saves phonewave configuration files.
// The session layer provides a filesystem-backed implementation.
type ConfigLoader interface {
	// Load reads and parses a phonewave.yaml file at the given path.
	Load(path string) (*Config, error)

	// Save writes the configuration to the given path as YAML.
	Save(path string, cfg *Config) error
}

// Scanner discovers D-Mail endpoints in a repository.
// The session layer provides a filesystem-backed implementation.
type Scanner interface {
	// ScanRepository scans a repository path for dot-directories
	// containing D-Mail skill declarations.
	ScanRepository(repoPath string) ([]Endpoint, error)
}

// ErrorStore provides durable storage for failed D-Mail deliveries.
// Stage records failures in a durable store; retry logic reads pending
// entries and re-attempts delivery.
//
// All methods must be safe for concurrent use by multiple goroutines
// within a single process and tolerant of concurrent access from
// separate CLI processes.
type ErrorStore interface {
	// RecordFailure saves a failed D-Mail for later retry. Idempotent:
	// re-recording the same entry (by name) is a no-op.
	RecordFailure(entry RetryEntry) error

	// ListPending returns entries that have not exceeded maxRetries,
	// ordered by creation time (oldest first).
	ListPending(maxRetries int) ([]RetryEntry, error)

	// MarkRetried increments the attempt counter and updates the error
	// message for the named entry.
	MarkRetried(name string, newError string) error

	// RemoveEntry deletes a successfully retried entry from the store.
	RemoveEntry(name string) error

	// Close releases database resources.
	Close() error
}

// RetryEntry holds metadata and payload for a failed D-Mail delivery.
type RetryEntry struct {
	Name         string    // unique key (timestamp-kind-originalName)
	SourceOutbox string    // absolute outbox directory that produced the D-Mail
	Kind         string    // D-Mail kind (specification, report, feedback, convergence)
	OriginalName string    // original filename in the outbox
	Data         []byte    // raw D-Mail content
	Attempts     int       // number of delivery attempts so far
	Error        string    // most recent error message
	CreatedAt    time.Time // first failure time
	UpdatedAt    time.Time // last retry time
}
