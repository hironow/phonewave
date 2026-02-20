package cmd

import "github.com/hironow/phonewave"

func printOrphanWarnings(orphans phonewave.OrphanReport) {
	for _, kind := range orphans.UnconsumedKinds {
		phonewave.LogWarn("Orphaned: kind=%q is produced but not consumed", kind)
	}
	for _, kind := range orphans.UnproducedKinds {
		phonewave.LogWarn("Orphaned: kind=%q is consumed but not produced", kind)
	}
}
