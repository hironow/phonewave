package domain

// EndpointDiff describes a change to an endpoint during sync.
type EndpointDiff struct {
	Repo   string
	Dir    string
	Change string // "added", "removed", "changed"
}

// RouteDiff describes a change to a route during sync.
type RouteDiff struct {
	Kind   string
	From   string
	Change string // "added", "removed"
}

// SyncReport holds the result of a sync operation including change diffs.
type SyncReport struct {
	Orphans         OrphanReport
	EndpointChanges []EndpointDiff
	RouteChanges    []RouteDiff
	RepoCount       int
	TotalRoutes     int
	Warnings        []string
}
