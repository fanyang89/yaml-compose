package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMain(t *testing.T) {
	require := require.New(t)

	tmpDir := t.TempDir()
	base := filepath.Join(tmpDir, "a.yaml")
	baseDir := base + ".d"
	out := filepath.Join(tmpDir, "out.yaml")
	err := os.WriteFile(base, []byte("service: base\n"), 0644)
	require.NoError(err)
	err = os.MkdirAll(baseDir, 0755)
	require.NoError(err)
	err = os.WriteFile(filepath.Join(baseDir, "1-layer.yaml"), []byte("service: layer\n"), 0644)
	require.NoError(err)

	originalArgs := os.Args
	t.Cleanup(func() {
		os.Args = originalArgs
	})
	os.Args = []string{"yaml-compose", base, "-o", out}

	main()

	b, err := os.ReadFile(out)
	require.NoError(err)
	require.Contains(string(b), "service: layer")
}
