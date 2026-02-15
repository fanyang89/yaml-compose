package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeRootCmdTestFiles(t *testing.T) (string, string) {
	t.Helper()

	tmpDir := t.TempDir()
	base := filepath.Join(tmpDir, "a.yaml")
	baseDir := base + ".d"

	err := os.WriteFile(base, []byte("service: base\n"), 0644)
	require.NoError(t, err)
	err = os.MkdirAll(baseDir, 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(baseDir, "1-layer.yaml"), []byte("service: layer\n"), 0644)
	require.NoError(t, err)

	return tmpDir, base
}

func TestRootCmd(t *testing.T) {
	require := require.New(t)

	tmpDir, base := writeRootCmdTestFiles(t)
	out := filepath.Join(tmpDir, "out.yaml")

	flagOutput = ""
	rootCmd.SetArgs([]string{base, "-o", out})
	err := rootCmd.Execute()
	require.NoError(err)

	b, err := os.ReadFile(out)
	require.NoError(err)
	require.Contains(string(b), "service: layer")
}

func TestRootCmdFailsWhenBaseMissing(t *testing.T) {
	require := require.New(t)

	base := filepath.Join(t.TempDir(), "missing.yaml")
	flagOutput = ""
	rootCmd.SetArgs([]string{base})
	err := rootCmd.Execute()
	require.Error(err)
	require.Contains(err.Error(), "not found")
}

func TestRootCmdFailsWhenLayerPathIsFile(t *testing.T) {
	require := require.New(t)

	tmpDir := t.TempDir()
	base := filepath.Join(tmpDir, "a.yaml")
	err := os.WriteFile(base, []byte("service: base\n"), 0644)
	require.NoError(err)
	err = os.WriteFile(base+".d", []byte("not a dir\n"), 0644)
	require.NoError(err)

	flagOutput = ""
	rootCmd.SetArgs([]string{base})
	err = rootCmd.Execute()
	require.Error(err)
	require.Contains(err.Error(), "not a directory")
}
