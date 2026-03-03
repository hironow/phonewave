package usecase

import (
	"fmt"
	"path/filepath"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/session"
)

// configBaseFromPath returns the directory containing the config file.
func configBaseFromPath(cfgPath string) string {
	return filepath.Dir(cfgPath)
}

// InitEcosystem scans repositories, generates a config, writes it, and
// ensures the state directory exists.
// Wraps the full Load→Init→Write→EnsureStateDir cycle.
func InitEcosystem(cfgPath string, repoPaths []string, logger domain.Logger) (*domain.InitResult, error) {
	result, err := session.Init(repoPaths)
	if err != nil {
		return nil, err
	}

	if err := session.WriteConfig(cfgPath, result.Config); err != nil {
		return nil, fmt.Errorf("write config: %w", err)
	}

	if err := session.EnsureStateDir(configBaseFromPath(cfgPath)); err != nil {
		return nil, fmt.Errorf("create state dir: %w", err)
	}

	return result, nil
}

// AddRepository adds a new repository to the ecosystem and persists the config.
// Wraps LoadConfig→Add→WriteConfig.
func AddRepository(cfgPath, repoPath string, logger domain.Logger) (*domain.AddResult, error) {
	cfg, err := session.LoadConfig(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	result, err := session.Add(cfg, repoPath)
	if err != nil {
		return nil, err
	}

	if err := session.WriteConfig(cfgPath, cfg); err != nil {
		return nil, fmt.Errorf("write config: %w", err)
	}

	result.RouteCount = len(cfg.Routes)
	return result, nil
}

// RemoveResult holds the result of a remove operation.
type RemoveResult struct {
	Orphans    domain.OrphanReport
	RouteCount int
}

// RemoveRepository removes a repository from the ecosystem and persists the config.
// Wraps LoadConfig→Remove→WriteConfig.
func RemoveRepository(cfgPath, repoPath string, logger domain.Logger) (*RemoveResult, error) {
	cfg, err := session.LoadConfig(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	orphans, err := session.Remove(cfg, repoPath)
	if err != nil {
		return nil, err
	}

	if err := session.WriteConfig(cfgPath, cfg); err != nil {
		return nil, fmt.Errorf("write config: %w", err)
	}

	return &RemoveResult{
		Orphans:    *orphans,
		RouteCount: len(cfg.Routes),
	}, nil
}

// SyncEcosystem re-scans all repositories and reconciles the routing table.
// Wraps LoadConfig→Sync→WriteConfig.
func SyncEcosystem(cfgPath string, logger domain.Logger) (*domain.SyncReport, error) {
	cfg, err := session.LoadConfig(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	report, err := session.Sync(cfg)
	if err != nil {
		return nil, fmt.Errorf("sync: %w", err)
	}

	if err := session.WriteConfig(cfgPath, cfg); err != nil {
		return nil, fmt.Errorf("write config: %w", err)
	}

	return report, nil
}

// GetStatus loads config and returns the current daemon/ecosystem status.
// Wraps LoadConfig→Status.
func GetStatus(cfgPath, stateDir string) (domain.StatusReport, error) {
	cfg, err := session.LoadConfig(cfgPath)
	if err != nil {
		return domain.StatusReport{}, fmt.Errorf("load config: %w", err)
	}

	return session.Status(cfg, stateDir), nil
}

// RunDoctor loads config and performs a health check.
// Wraps LoadConfig→Doctor.
func RunDoctor(cfgPath, stateDir string) (domain.DoctorReport, error) {
	cfg, err := session.LoadConfig(cfgPath)
	if err != nil {
		return domain.DoctorReport{}, fmt.Errorf("load config: %w", err)
	}

	return session.Doctor(cfg, stateDir), nil
}

// FormatDoctorJSON marshals a DoctorReport to indented JSON.
// Wraps session.FormatDoctorJSON.
func FormatDoctorJSON(report domain.DoctorReport) ([]byte, error) {
	return session.FormatDoctorJSON(report)
}

// ListExpiredEventFiles returns .jsonl files older than the given days.
// Wraps session.ListExpiredEventFiles.
func ListExpiredEventFiles(stateDir string, days int) ([]string, error) {
	return session.ListExpiredEventFiles(stateDir, days)
}

// PruneEventFiles deletes the named .jsonl files from the events directory.
// Wraps session.PruneEventFiles.
func PruneEventFiles(stateDir string, files []string) ([]string, error) {
	return session.PruneEventFiles(stateDir, files)
}
