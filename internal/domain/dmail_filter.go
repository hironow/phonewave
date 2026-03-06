package domain

import (
	"path/filepath"
	"strings"
)

// IsDMailFile returns true if name refers to a deliverable D-Mail file.
// It checks for a .md extension and rejects temporary files written by
// atomicWrite (prefixed with ".phonewave-tmp-").
func IsDMailFile(name string) bool {
	base := filepath.Base(name)
	if base == "" || base == "." {
		return false
	}
	return filepath.Ext(base) == ".md" && !strings.HasPrefix(base, ".phonewave-tmp-")
}
