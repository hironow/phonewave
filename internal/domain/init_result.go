package domain

// InitResult holds the result of an init operation.
type InitResult struct {
	Config    *Config
	Orphans   OrphanReport
	RepoCount int
	Warnings  []string
}
