package fsutils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDirExists(t *testing.T) {
	require := require.New(t)

	tmpDir := t.TempDir()

	exists, err := DirExists(tmpDir)
	require.NoError(err)
	require.True(exists)
}

func TestDirExistsReturnsFalseForMissingPath(t *testing.T) {
	require := require.New(t)

	exists, err := DirExists(filepath.Join(t.TempDir(), "missing"))
	require.NoError(err)
	require.False(exists)
}

func TestDirExistsReturnsErrorForFilePath(t *testing.T) {
	require := require.New(t)

	tmpDir := t.TempDir()
	p := filepath.Join(tmpDir, "a.txt")
	err := os.WriteFile(p, []byte("x"), 0644)
	require.NoError(err)

	exists, err := DirExists(p)
	require.Error(err)
	require.False(exists)
	require.Contains(err.Error(), "not a directory")
}
