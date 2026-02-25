package service

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanRepository_DiscoverEndpoints(t *testing.T) {
	// given — set up a fake repository with .siren and .expedition
	repoDir := t.TempDir()

	// .siren with sendable skill
	sirenSendable := filepath.Join(repoDir, ".siren", "skills", "dmail-sendable")
	if err := os.MkdirAll(sirenSendable, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sirenSendable, "SKILL.md"), []byte(`---
name: "dmail-sendable"
description: "Produces D-Mail messages"
license: Apache-2.0
metadata:
  dmail-schema-version: "1"
  produces:
    - kind: specification
      description: "Issue specification"
---
`), 0644); err != nil {
		t.Fatal(err)
	}

	// .siren with readable skill
	sirenReadable := filepath.Join(repoDir, ".siren", "skills", "dmail-readable")
	if err := os.MkdirAll(sirenReadable, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sirenReadable, "SKILL.md"), []byte(`---
name: "dmail-readable"
description: "Reads D-Mail messages"
license: Apache-2.0
metadata:
  dmail-schema-version: "1"
  consumes:
    - kind: feedback
      description: "Corrective feedback"
---
`), 0644); err != nil {
		t.Fatal(err)
	}

	// .expedition with sendable
	expedSendable := filepath.Join(repoDir, ".expedition", "skills", "dmail-sendable")
	if err := os.MkdirAll(expedSendable, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(expedSendable, "SKILL.md"), []byte(`---
name: "dmail-sendable"
description: "Produces implementation reports"
license: Apache-2.0
metadata:
  dmail-schema-version: "1"
  produces:
    - kind: report
      description: "Implementation report"
---
`), 0644); err != nil {
		t.Fatal(err)
	}

	// .expedition with readable
	expedReadable := filepath.Join(repoDir, ".expedition", "skills", "dmail-readable")
	if err := os.MkdirAll(expedReadable, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(expedReadable, "SKILL.md"), []byte(`---
name: "dmail-readable"
description: "Reads D-Mail messages"
license: Apache-2.0
metadata:
  dmail-schema-version: "1"
  consumes:
    - kind: specification
      description: "Issue specification"
    - kind: feedback
      description: "Corrective feedback"
---
`), 0644); err != nil {
		t.Fatal(err)
	}

	// when
	endpoints, err := ScanRepository(repoDir)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(endpoints) != 2 {
		t.Fatalf("want 2 endpoints, got %d", len(endpoints))
	}

	// Find endpoints by dir name
	endpointMap := make(map[string]struct{ produces, consumes []string })
	for _, ep := range endpoints {
		endpointMap[ep.Dir] = struct{ produces, consumes []string }{ep.Produces, ep.Consumes}
	}

	siren, ok := endpointMap[".siren"]
	if !ok {
		t.Fatal("missing .siren endpoint")
	}
	if len(siren.produces) != 1 || siren.produces[0] != "specification" {
		t.Errorf(".siren produces = %v, want [specification]", siren.produces)
	}
	if len(siren.consumes) != 1 || siren.consumes[0] != "feedback" {
		t.Errorf(".siren consumes = %v, want [feedback]", siren.consumes)
	}

	exped, ok := endpointMap[".expedition"]
	if !ok {
		t.Fatal("missing .expedition endpoint")
	}
	if len(exped.produces) != 1 || exped.produces[0] != "report" {
		t.Errorf(".expedition produces = %v, want [report]", exped.produces)
	}
	if len(exped.consumes) != 2 {
		t.Errorf(".expedition consumes = %v, want [specification feedback]", exped.consumes)
	}
}

func TestScanRepository_NoDotDirs(t *testing.T) {
	// given — empty repo
	repoDir := t.TempDir()

	// when
	endpoints, err := ScanRepository(repoDir)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(endpoints) != 0 {
		t.Errorf("want 0 endpoints, got %d", len(endpoints))
	}
}

func TestScanRepository_SkipsNonDotDirs(t *testing.T) {
	// given — a regular directory that happens to have skills/
	repoDir := t.TempDir()
	regularSkills := filepath.Join(repoDir, "src", "skills", "dmail-sendable")
	if err := os.MkdirAll(regularSkills, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(regularSkills, "SKILL.md"), []byte(`---
name: "dmail-sendable"
description: "test"
license: Apache-2.0
metadata:
  dmail-schema-version: "1"
  produces:
    - kind: specification
---
`), 0644); err != nil {
		t.Fatal(err)
	}

	// when
	endpoints, err := ScanRepository(repoDir)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(endpoints) != 0 {
		t.Errorf("want 0 endpoints (non-dot dirs should be skipped), got %d", len(endpoints))
	}
}
