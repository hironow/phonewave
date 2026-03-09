package session

import (
	"fmt"
	"path/filepath"

	"github.com/hironow/phonewave/internal/domain"
)

// InitAdapter implements port.InitRunner by delegating to session I/O functions.
type InitAdapter struct{}

// ScanAndInit scans repositories, writes config, and creates the state directory.
func (a *InitAdapter) ScanAndInit(repoPaths []string, cfgPath string) (*domain.InitResult, error) {
	result, err := Init(repoPaths)
	if err != nil {
		return nil, err
	}
	if err := WriteConfig(cfgPath, result.Config); err != nil {
		return nil, fmt.Errorf("write config: %w", err)
	}
	if err := EnsureStateDir(filepath.Dir(filepath.Dir(cfgPath))); err != nil {
		return nil, fmt.Errorf("create state dir: %w", err)
	}
	return result, nil
}
