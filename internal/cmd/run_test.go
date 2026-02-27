package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunCmd_FailsWithoutInit(t *testing.T) {
	// given: empty directory with no phonewave.yaml or .phonewave/
	dir := t.TempDir()

	cmd := NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--config", dir + "/phonewave.yaml", "run"})

	// when
	err := cmd.Execute()

	// then: should fail with init guidance
	if err == nil {
		t.Fatal("expected error for uninitialized state, got nil")
	}
	got := err.Error()
	if !strings.Contains(got, "init") {
		t.Errorf("expected error to mention 'init', got: %s", got)
	}
}
