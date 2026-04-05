package session

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/hironow/phonewave/internal/domain"
)

const providerStateFileName = "provider-state.json"

func writeProviderStateSnapshot(stateDir string, snapshot domain.ProviderStateSnapshot) error {
	data, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(stateDir, providerStateFileName), data, 0o644)
}

func loadProviderStateSnapshot(stateDir string) (domain.ProviderStateSnapshot, bool) {
	data, err := os.ReadFile(filepath.Join(stateDir, providerStateFileName))
	if err != nil {
		return domain.ProviderStateSnapshot{}, false
	}
	var snapshot domain.ProviderStateSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return domain.ProviderStateSnapshot{}, false
	}
	return snapshot, true
}
