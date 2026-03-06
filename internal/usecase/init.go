package usecase

import (
	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/usecase/port"
)

// RunInit delegates repo scanning + config generation to the InitRunner port.
// The command is always-valid by construction — no validation needed.
func RunInit(cmd domain.InitCommand, runner port.InitRunner) (*domain.InitResult, error) {
	return runner.ScanAndInit(cmd.RepoPaths().Strings(), cmd.ConfigPath().String())
}
