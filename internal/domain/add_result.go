package domain

// AddResult holds the result of an add operation.
type AddResult struct {
	Orphans    OrphanReport
	Warnings   []string
	RouteCount int
}
