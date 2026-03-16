package session

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/hironow/phonewave/internal/domain"
)

const deliveryFilterFile = "delivered.bloom"

// SaveDeliveryFilter persists a Bloom filter to {stateDir}/.run/delivered.bloom.
func SaveDeliveryFilter(stateDir string, bf *domain.BloomFilter) error {
	if bf == nil {
		return nil
	}
	data := bf.MarshalBinary()
	return os.WriteFile(filepath.Join(stateDir, ".run", deliveryFilterFile), data, 0o600)
}

// LoadDeliveryFilter loads a persisted Bloom filter from {stateDir}/.run/delivered.bloom.
// Returns (nil, nil) when the file does not exist.
func LoadDeliveryFilter(stateDir string) (*domain.BloomFilter, error) {
	path := filepath.Join(stateDir, ".run", deliveryFilterFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	return domain.UnmarshalBloomFilter(data)
}
