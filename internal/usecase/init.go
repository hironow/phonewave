package usecase

import (
	"fmt"
	"strings"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/usecase/port"
)

// RunInit validates the InitCommand and delegates repo scanning + config
// generation to the InitRunner port.
func RunInit(cmd domain.InitCommand, runner port.InitRunner) (*domain.InitResult, error) {
	if errs := cmd.Validate(); len(errs) > 0 {
		msgs := make([]string, len(errs))
		for i, e := range errs {
			msgs[i] = e.Error()
		}
		return nil, fmt.Errorf("invalid init command: %s", strings.Join(msgs, "; "))
	}
	return runner.ScanAndInit(cmd.RepoPaths, cmd.ConfigPath)
}
