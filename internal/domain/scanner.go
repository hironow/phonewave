package domain

// Endpoint represents a discovered tool endpoint within a repository.
type Endpoint struct {
	Dir      string   // dot-directory name, e.g. ".siren"
	Produces []string // list of kind values this endpoint produces
	Consumes []string // list of kind values this endpoint consumes
}
