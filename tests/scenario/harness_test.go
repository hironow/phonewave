//go:build scenario

package scenario_test

import (
	"bytes"
	"context"
	"os/exec"
)

// Workspace represents a temporary directory with all 4 tool state dirs initialized.
type Workspace struct {
	Root     string // t.TempDir()
	RepoPath string // workspace 内の simulated repo
	BinDir   string
	Env      []string
}

// ToolProcess wraps a running tool process.
type ToolProcess struct {
	Cmd    *exec.Cmd
	Cancel context.CancelFunc
	Stdout *bytes.Buffer
	Stderr *bytes.Buffer
}
