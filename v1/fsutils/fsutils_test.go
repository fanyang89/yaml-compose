package fsutils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
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

func TestFileExistsOnMemFs(t *testing.T) {
	require := require.New(t)
	fs := afero.NewMemMapFs()
	err := afero.WriteFile(fs, "/a.txt", []byte("x"), 0644)
	require.NoError(err)

	exists, err := FileExistsOn(fs, "/a.txt")
	require.NoError(err)
	require.True(exists)
}

func TestFileExistsOnReturnsErrorForInvalidPath(t *testing.T) {
	require := require.New(t)

	exists, err := FileExistsOn(afero.NewOsFs(), "bad\x00path")
	require.Error(err)
	require.False(exists)
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

func TestDirExistsOnMemFs(t *testing.T) {
	require := require.New(t)
	fs := afero.NewMemMapFs()
	err := fs.MkdirAll("/a-dir", 0755)
	require.NoError(err)

	exists, err := DirExistsOn(fs, "/a-dir")
	require.NoError(err)
	require.True(exists)
}

func TestDirExistsOnReturnsErrorForInvalidPath(t *testing.T) {
	require := require.New(t)

	exists, err := DirExistsOn(afero.NewOsFs(), "bad\x00path")
	require.Error(err)
	require.False(exists)
}
