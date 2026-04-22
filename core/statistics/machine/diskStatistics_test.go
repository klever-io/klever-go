package machine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDiskStatistics_CreatesInstance(t *testing.T) {
	t.Parallel()

	ds := NewDiskStatistics("/tmp", "/tmp/db")
	require.NotNil(t, ds)
	assert.Equal(t, "/tmp", ds.workingDir)
	assert.Equal(t, "/tmp/db", ds.dbPath)
}

func TestDiskStatistics_InitialValuesAreZero(t *testing.T) {
	t.Parallel()

	ds := NewDiskStatistics("/tmp", "/tmp/db")
	assert.Equal(t, uint64(0), ds.DiskTotal())
	assert.Equal(t, uint64(0), ds.DiskAvailable())
	assert.Equal(t, uint64(0), ds.DiskPercent())
	assert.Equal(t, uint64(0), ds.DbSize())
}

func TestDiskStatistics_ComputeDiskUsage_SetsValues(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	ds := NewDiskStatistics(tmpDir, tmpDir)
	ds.computeDiskUsage()

	assert.Greater(t, ds.DiskTotal(), uint64(0))
	assert.Greater(t, ds.DiskAvailable(), uint64(0))
}

func TestDiskStatistics_DbSize_CachesResult(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	ds := NewDiskStatistics(tmpDir, tmpDir)

	err := os.WriteFile(filepath.Join(tmpDir, "db.dat"), make([]byte, 100), 0600)
	require.NoError(t, err)

	// First call should compute and set lastDbCheck and DbSize
	ds.computeDbSizeIfNeeded()
	firstCheck := ds.lastDbCheck
	firstSize := ds.DbSize()
	assert.Greater(t, firstCheck, int64(0))
	assert.Equal(t, uint64(100), firstSize)

	err = os.WriteFile(filepath.Join(tmpDir, "db2.dat"), make([]byte, 100), 0600)
	require.NoError(t, err)

	// Immediate second call should skip (cached), both lastDbCheck and DbSize unchanged
	ds.computeDbSizeIfNeeded()
	assert.Equal(t, firstCheck, ds.lastDbCheck)
	assert.Equal(t, firstSize, ds.DbSize())
}

func TestGetDirSizeBytes_EmptyDir(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	size := getDirSizeBytes(tmpDir)
	assert.Equal(t, uint64(0), size)
}

func TestGetDirSizeBytes_NonexistentDir(t *testing.T) {
	t.Parallel()

	size := getDirSizeBytes("/nonexistent/path/that/does/not/exist")
	assert.Equal(t, uint64(0), size)
}

func TestGetDirSizeBytes_WithFiles(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Write two files with known sizes
	err := os.WriteFile(filepath.Join(tmpDir, "a.dat"), make([]byte, 100), 0600)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "b.dat"), make([]byte, 200), 0600)
	require.NoError(t, err)

	size := getDirSizeBytes(tmpDir)
	assert.Equal(t, uint64(300), size)
}

func TestGetDirSizeBytes_Recursive(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "sub")
	err := os.Mkdir(subDir, 0700)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(tmpDir, "root.dat"), make([]byte, 50), 0600)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(subDir, "nested.dat"), make([]byte, 150), 0600)
	require.NoError(t, err)

	size := getDirSizeBytes(tmpDir)
	assert.Equal(t, uint64(200), size)
}
