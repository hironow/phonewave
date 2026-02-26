package session

import (
	"os"
	"path/filepath"
	"strings"

	phonewave "github.com/hironow/phonewave"
)

// Compile-time check that FileScanner implements phonewave.Scanner.
var _ phonewave.Scanner = (*FileScanner)(nil)

// FileScanner implements phonewave.Scanner using the local filesystem.
type FileScanner struct{}

// ScanRepository scans a repository path for dot-directories containing
// D-Mail skill declarations.
func (FileScanner) ScanRepository(repoPath string) ([]phonewave.Endpoint, error) {
	return ScanRepository(repoPath)
}

// ScanRepository scans a repository path for dot-directories containing
// D-Mail skill declarations (skills/dmail-sendable/SKILL.md and
// skills/dmail-readable/SKILL.md).
func ScanRepository(repoPath string) ([]phonewave.Endpoint, error) {
	entries, err := os.ReadDir(repoPath)
	if err != nil {
		return nil, err
	}

	var endpoints []phonewave.Endpoint
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Only consider dot-prefixed directories
		if !strings.HasPrefix(name, ".") {
			continue
		}
		// Skip common non-tool dot directories
		if name == ".git" || name == ".github" || name == ".phonewave" {
			continue
		}

		ep, found, err := scanEndpoint(repoPath, name)
		if err != nil {
			return nil, err
		}
		if found {
			endpoints = append(endpoints, ep)
		}
	}

	return endpoints, nil
}

// scanEndpoint checks a single dot-directory for D-Mail skill declarations.
func scanEndpoint(repoPath, dirName string) (phonewave.Endpoint, bool, error) {
	ep := phonewave.Endpoint{Dir: dirName}
	found := false

	// Check for sendable skill
	sendablePath := filepath.Join(repoPath, dirName, "skills", phonewave.SkillSendable, "SKILL.md")
	if data, err := os.ReadFile(sendablePath); err == nil {
		skill, err := phonewave.ParseSkillFrontmatter(data)
		if err != nil {
			return ep, false, err
		}
		for _, p := range skill.Produces {
			ep.Produces = append(ep.Produces, p.Kind)
		}
		found = true
	}

	// Check for readable skill
	readablePath := filepath.Join(repoPath, dirName, "skills", phonewave.SkillReadable, "SKILL.md")
	if data, err := os.ReadFile(readablePath); err == nil {
		skill, err := phonewave.ParseSkillFrontmatter(data)
		if err != nil {
			return ep, false, err
		}
		for _, c := range skill.Consumes {
			ep.Consumes = append(ep.Consumes, c.Kind)
		}
		found = true
	}

	return ep, found, nil
}
