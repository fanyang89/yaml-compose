package fsutils

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFileExists(t *testing.T) {
	require := require.New(t)

	tmpDir := t.TempDir()
	p := filepath.Join(tmpDir, "a.txt")
	err := os.WriteFile(p, []byte("x"), 0644)
	require.NoError(err)

	exists, err := FileExists(p)
	require.NoError(err)
	require.True(exists)
}

func TestFileExistsReturnsFalseForMissingPath(t *testing.T) {
	require := require.New(t)

	exists, err := FileExists(filepath.Join(t.TempDir(), "missing.txt"))
	require.NoError(err)
	require.False(exists)
}

func TestFileExistsReturnsErrorWhenStatFails(t *testing.T) {
	require := require.New(t)

	exists, err := fileExistsWithStat(func(string) (os.FileInfo, error) {
		return nil, errors.New("stat failed")
	}, "a.txt")
	require.Error(err)
	require.False(exists)
	require.Contains(err.Error(), "stat failed")
}

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

func TestDirExistsReturnsErrorWhenStatFails(t *testing.T) {
	require := require.New(t)

	exists, err := dirExistsWithStat(func(string) (os.FileInfo, error) {
		return nil, errors.New("stat failed")
	}, "a-dir")
	require.Error(err)
	require.False(exists)
	require.Contains(err.Error(), "stat failed")
}
