package session_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/session"
)

func TestInsightWriter_WriteNew(t *testing.T) {
	dir := t.TempDir()
	insightsDir := filepath.Join(dir, "insights")
	runDir := filepath.Join(dir, ".run")
	os.MkdirAll(insightsDir, 0o755)
	os.MkdirAll(runDir, 0o755)

	w := session.NewInsightWriter(insightsDir, runDir)

	entry := domain.InsightEntry{
		Title:       "test insight",
		What:        "observed X",
		Why:         "because Y",
		How:         "do Z",
		When:        "always",
		Who:         "test",
		Constraints: "none",
	}

	err := w.Append("test.md", "test-kind", "test-tool", entry)
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	// Verify file was created
	data, err := os.ReadFile(filepath.Join(insightsDir, "test.md"))
	if err != nil {
		t.Fatalf("read insight file: %v", err)
	}

	file, err := domain.UnmarshalInsightFile(data)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(file.All()) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(file.All()))
	}
	if file.All()[0].Title != "test insight" {
		t.Errorf("expected title 'test insight', got %q", file.All()[0].Title)
	}
	if file.Kind != "test-kind" {
		t.Errorf("expected kind 'test-kind', got %q", file.Kind)
	}
}

func TestInsightWriter_AppendExisting(t *testing.T) {
	dir := t.TempDir()
	insightsDir := filepath.Join(dir, "insights")
	runDir := filepath.Join(dir, ".run")
	os.MkdirAll(insightsDir, 0o755)
	os.MkdirAll(runDir, 0o755)

	w := session.NewInsightWriter(insightsDir, runDir)

	e1 := domain.InsightEntry{Title: "first", What: "a", Why: "b", How: "c", When: "d", Who: "e", Constraints: "f"}
	e2 := domain.InsightEntry{Title: "second", What: "g", Why: "h", How: "i", When: "j", Who: "k", Constraints: "l"}

	if err := w.Append("multi.md", "lumina", "paintress", e1); err != nil {
		t.Fatalf("first append: %v", err)
	}
	if err := w.Append("multi.md", "lumina", "paintress", e2); err != nil {
		t.Fatalf("second append: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(insightsDir, "multi.md"))
	file, _ := domain.UnmarshalInsightFile(data)

	if len(file.All()) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(file.All()))
	}
	if file.All()[0].Title != "first" {
		t.Errorf("first entry title: %q", file.All()[0].Title)
	}
	if file.All()[1].Title != "second" {
		t.Errorf("second entry title: %q", file.All()[1].Title)
	}
}

func TestInsightWriter_AtomicNoCorruption(t *testing.T) {
	dir := t.TempDir()
	insightsDir := filepath.Join(dir, "insights")
	runDir := filepath.Join(dir, ".run")
	os.MkdirAll(insightsDir, 0o755)
	os.MkdirAll(runDir, 0o755)

	w := session.NewInsightWriter(insightsDir, runDir)
	entry := domain.InsightEntry{Title: "atomic", What: "a", Why: "b", How: "c", When: "d", Who: "e", Constraints: "f"}

	_ = w.Append("atomic.md", "test", "test", entry)

	// No temp files should remain
	matches, _ := filepath.Glob(filepath.Join(insightsDir, ".*.tmp"))
	if len(matches) > 0 {
		t.Errorf("temp files should be cleaned up, found: %v", matches)
	}
}

func TestInsightWriter_IdempotentAppend(t *testing.T) {
	dir := t.TempDir()
	insightsDir := filepath.Join(dir, "insights")
	runDir := filepath.Join(dir, ".run")
	os.MkdirAll(insightsDir, 0o755)
	os.MkdirAll(runDir, 0o755)

	w := session.NewInsightWriter(insightsDir, runDir)

	entry := domain.InsightEntry{Title: "dedup me", What: "a", Why: "b", How: "c", When: "d", Who: "e", Constraints: "f"}

	// Append twice with same title
	if err := w.Append("dedup.md", "test", "test", entry); err != nil {
		t.Fatalf("first append: %v", err)
	}
	if err := w.Append("dedup.md", "test", "test", entry); err != nil {
		t.Fatalf("second append: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(insightsDir, "dedup.md"))
	file, _ := domain.UnmarshalInsightFile(data)

	if len(file.All()) != 1 {
		t.Errorf("expected 1 entry (idempotent), got %d", len(file.All()))
	}
}

func TestInsightWriter_PropagatesNonENOENT(t *testing.T) {
	dir := t.TempDir()
	insightsDir := filepath.Join(dir, "insights")
	runDir := filepath.Join(dir, ".run")
	os.MkdirAll(insightsDir, 0o755)
	os.MkdirAll(runDir, 0o755)

	// Create a corrupt file (invalid YAML frontmatter)
	os.WriteFile(filepath.Join(insightsDir, "corrupt.md"), []byte("not valid insight file"), 0o644)

	w := session.NewInsightWriter(insightsDir, runDir)
	entry := domain.InsightEntry{Title: "test", What: "a", Why: "b", How: "c", When: "d", Who: "e", Constraints: "f"}

	err := w.Append("corrupt.md", "test", "test", entry)
	if err == nil {
		t.Fatal("expected error for corrupt file, got nil")
	}
}

func TestInsightWriter_ReadEntries(t *testing.T) {
	dir := t.TempDir()
	insightsDir := filepath.Join(dir, "insights")
	runDir := filepath.Join(dir, ".run")
	os.MkdirAll(insightsDir, 0o755)
	os.MkdirAll(runDir, 0o755)

	w := session.NewInsightWriter(insightsDir, runDir)
	entry := domain.InsightEntry{Title: "readable", What: "a", Why: "b", How: "c", When: "d", Who: "e", Constraints: "f"}
	_ = w.Append("read.md", "lumina", "paintress", entry)

	file, err := w.Read("read.md")
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if len(file.All()) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(file.All()))
	}
}

func TestInsightWriter_DeliveryFailureInsight(t *testing.T) {
	// given: simulate what the delivery.failed policy handler writes
	dir := t.TempDir()
	insightsDir := filepath.Join(dir, "insights")
	runDir := filepath.Join(dir, ".run")
	os.MkdirAll(insightsDir, 0o755)
	os.MkdirAll(runDir, 0o755)

	w := session.NewInsightWriter(insightsDir, runDir)

	entry := domain.InsightEntry{
		Title:       "delivery-failed-specification-20260310T143000",
		What:        "Delivery failed for kind specification from /repo/.siren/outbox: permission denied",
		Why:         "Permission denied on target inbox directory",
		How:         "Check target inbox directory permissions and disk space",
		When:        "During delivery scan cycle",
		Who:         "phonewave courier daemon (event-abc123)",
		Constraints: "Automatic retry via error queue",
		Extra: map[string]string{
			"route": "/repo/.siren/outbox -> targets",
		},
	}

	// when
	err := w.Append("delivery.md", "delivery-failure", "phonewave", entry)

	// then: file was created with correct content
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(insightsDir, "delivery.md"))
	if err != nil {
		t.Fatalf("read insight file: %v", err)
	}

	file, err := domain.UnmarshalInsightFile(data)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if file.Kind != "delivery-failure" {
		t.Errorf("expected kind 'delivery-failure', got %q", file.Kind)
	}
	if file.Tool != "phonewave" {
		t.Errorf("expected tool 'phonewave', got %q", file.Tool)
	}
	if len(file.All()) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(file.All()))
	}

	e := file.All()[0]
	if !strings.Contains(e.Title, "delivery-failed-specification-") {
		t.Errorf("expected title pattern, got: %s", e.Title)
	}
	if !strings.Contains(e.What, "permission denied") {
		t.Errorf("expected what to contain error, got: %s", e.What)
	}
	if !strings.Contains(e.Why, "Permission denied") {
		t.Errorf("expected why categorization, got: %s", e.Why)
	}
	if e.Extra["route"] == "" {
		t.Error("expected extra 'route' to be set")
	}
}

func TestInsightWriter_DeliveryFailureIdempotent(t *testing.T) {
	// given: same insight title should be deduplicated
	dir := t.TempDir()
	insightsDir := filepath.Join(dir, "insights")
	runDir := filepath.Join(dir, ".run")
	os.MkdirAll(insightsDir, 0o755)
	os.MkdirAll(runDir, 0o755)

	w := session.NewInsightWriter(insightsDir, runDir)

	entry := domain.InsightEntry{
		Title:       "delivery-failed-specification-20260310T143000",
		What:        "Delivery failed",
		Why:         "Permission denied",
		How:         "Check permissions",
		When:        "During scan",
		Who:         "phonewave",
		Constraints: "Retry via error queue",
	}

	// when: append same entry twice
	if err := w.Append("delivery.md", "delivery-failure", "phonewave", entry); err != nil {
		t.Fatalf("first append: %v", err)
	}
	if err := w.Append("delivery.md", "delivery-failure", "phonewave", entry); err != nil {
		t.Fatalf("second append: %v", err)
	}

	// then: only one entry (idempotent)
	data, _ := os.ReadFile(filepath.Join(insightsDir, "delivery.md"))
	file, _ := domain.UnmarshalInsightFile(data)
	if len(file.All()) != 1 {
		t.Errorf("expected 1 entry (idempotent), got %d", len(file.All()))
	}
}
