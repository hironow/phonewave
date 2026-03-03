package session

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTryLockDaemon_AcquiresLock(t *testing.T) {
	dir := t.TempDir()
	unlock, err := TryLockDaemon(dir)
	require.NoError(t, err)
	require.NotNil(t, unlock)
	defer unlock()
}

func TestTryLockDaemon_RejectsSecondLock(t *testing.T) {
	dir := t.TempDir()
	unlock1, err := TryLockDaemon(dir)
	require.NoError(t, err)
	defer unlock1()

	_, err = TryLockDaemon(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already running")
}

func TestTryLockDaemon_ReleasesOnUnlock(t *testing.T) {
	dir := t.TempDir()
	unlock1, err := TryLockDaemon(dir)
	require.NoError(t, err)
	unlock1()

	unlock2, err := TryLockDaemon(dir)
	require.NoError(t, err)
	defer unlock2()
}

func TestTryLockDaemon_CreatesRunDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".run")
	unlock, err := TryLockDaemon(dir)
	require.NoError(t, err)
	defer unlock()
	_, statErr := os.Stat(filepath.Join(dir, "daemon.lock"))
	require.NoError(t, statErr)
}
