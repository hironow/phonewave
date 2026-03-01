package phonewave

import "time"

// ErrorMetadata holds metadata for a failed D-Mail stored as a .err sidecar.
type ErrorMetadata struct {
	SourceOutbox string    `yaml:"source_outbox"`
	Kind         string    `yaml:"kind"`
	OriginalName string    `yaml:"original_name"`
	Attempts     int       `yaml:"attempts"`
	Error        string    `yaml:"error"`
	Timestamp    time.Time `yaml:"timestamp"`
}
