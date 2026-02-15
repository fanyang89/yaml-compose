package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func setupRootCmdDeps(t *testing.T) {
	t.Helper()

	originalFileExists := fileExists
	originalDirExists := dirExists
	originalReadDir := readDir
	originalMkdirAll := mkdirAll
	originalWriteFile := writeFile
	originalPrintLine := printLine

	t.Cleanup(func() {
		fileExists = originalFileExists
		dirExists = originalDirExists
		readDir = originalReadDir
		mkdirAll = originalMkdirAll
		writeFile = originalWriteFile
		printLine = originalPrintLine
		flagOutput = ""
		rootCmd.SetArgs(nil)
	})
}

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

func TestRootCmdWritesOutputFile(t *testing.T) {
	setupRootCmdDeps(t)
	require := require.New(t)

	tmpDir, base := writeRootCmdTestFiles(t)
	out := filepath.Join(tmpDir, "out.yaml")

	rootCmd.SetArgs([]string{base, "-o", out})
	err := rootCmd.Execute()
	require.NoError(err)

	b, err := os.ReadFile(out)
	require.NoError(err)
	require.Contains(string(b), "service: layer")
}

func TestRootCmdPrintsWhenOutputFlagMissing(t *testing.T) {
	setupRootCmdDeps(t)
	require := require.New(t)

	_, base := writeRootCmdTestFiles(t)
	printed := ""
	printLine = func(args ...interface{}) (int, error) {
		printed = fmt.Sprint(args...)
		return len(printed), nil
	}

	rootCmd.SetArgs([]string{base})
	err := rootCmd.Execute()
	require.NoError(err)
	require.Contains(printed, "service: layer")
}

func TestRootCmdFailsWhenBaseMissing(t *testing.T) {
	setupRootCmdDeps(t)
	require := require.New(t)

	base := filepath.Join(t.TempDir(), "missing.yaml")
	rootCmd.SetArgs([]string{base})
	err := rootCmd.Execute()
	require.Error(err)
	require.Contains(err.Error(), "not found")
}

func TestRootCmdFailsWhenBaseCheckFails(t *testing.T) {
	setupRootCmdDeps(t)
	require := require.New(t)

	fileExists = func(string) (bool, error) {
		return false, errors.New("stat failed")
	}

	rootCmd.SetArgs([]string{"base.yaml"})
	err := rootCmd.Execute()
	require.Error(err)
	require.Contains(err.Error(), "check base file")
	require.Contains(err.Error(), "stat failed")
}

func TestRootCmdFailsWhenLayerDirMissing(t *testing.T) {
	setupRootCmdDeps(t)
	require := require.New(t)

	base := filepath.Join(t.TempDir(), "a.yaml")
	err := os.WriteFile(base, []byte("service: base\n"), 0644)
	require.NoError(err)

	rootCmd.SetArgs([]string{base})
	err = rootCmd.Execute()
	require.Error(err)
	require.Contains(err.Error(), "not found")
	require.Contains(err.Error(), base+".d")
}

func TestRootCmdFailsWhenLayerPathIsFile(t *testing.T) {
	setupRootCmdDeps(t)
	require := require.New(t)

	tmpDir := t.TempDir()
	base := filepath.Join(tmpDir, "a.yaml")
	err := os.WriteFile(base, []byte("service: base\n"), 0644)
	require.NoError(err)
	err = os.WriteFile(base+".d", []byte("not a dir\n"), 0644)
	require.NoError(err)

	rootCmd.SetArgs([]string{base})
	err = rootCmd.Execute()
	require.Error(err)
	require.Contains(err.Error(), "check layer directory")
	require.Contains(err.Error(), "not a directory")
}

func TestRootCmdFailsWhenReadLayerDirectoryFails(t *testing.T) {
	setupRootCmdDeps(t)
	require := require.New(t)

	base := filepath.Join(t.TempDir(), "a.yaml")
	err := os.WriteFile(base, []byte("service: base\n"), 0644)
	require.NoError(err)
	dirExists = func(string) (bool, error) {
		return true, nil
	}
	readDir = func(string) ([]os.DirEntry, error) {
		return nil, errors.New("read failed")
	}

	rootCmd.SetArgs([]string{base})
	err = rootCmd.Execute()
	require.Error(err)
	require.Contains(err.Error(), "read layer directory")
	require.Contains(err.Error(), "read failed")
}

func TestRootCmdIncludesYAMLAndYMLExtensionsOnly(t *testing.T) {
	setupRootCmdDeps(t)
	require := require.New(t)

	tmpDir := t.TempDir()
	base := filepath.Join(tmpDir, "a.yaml")
	baseDir := base + ".d"
	err := os.WriteFile(base, []byte("service: base\n"), 0644)
	require.NoError(err)
	err = os.MkdirAll(baseDir, 0755)
	require.NoError(err)
	err = os.WriteFile(filepath.Join(baseDir, "1-layer.yaml"), []byte("service: yaml\n"), 0644)
	require.NoError(err)
	err = os.WriteFile(filepath.Join(baseDir, "2-layer.yml"), []byte("service: yml\n"), 0644)
	require.NoError(err)
	err = os.WriteFile(filepath.Join(baseDir, "3-layer.txt"), []byte("service: ignored\n"), 0644)
	require.NoError(err)

	out := filepath.Join(tmpDir, "out.yaml")
	rootCmd.SetArgs([]string{base, "-o", out})
	err = rootCmd.Execute()
	require.NoError(err)

	b, err := os.ReadFile(out)
	require.NoError(err)
	require.Contains(string(b), "service: yml")
	require.NotContains(string(b), "ignored")
}

func TestRootCmdFailsWhenComposeFails(t *testing.T) {
	setupRootCmdDeps(t)
	require := require.New(t)

	tmpDir := t.TempDir()
	base := filepath.Join(tmpDir, "a.yaml")
	baseDir := base + ".d"
	err := os.WriteFile(base, []byte("service: base\n"), 0644)
	require.NoError(err)
	err = os.MkdirAll(baseDir, 0755)
	require.NoError(err)
	err = os.WriteFile(filepath.Join(baseDir, "bad.yaml"), []byte("service: layer\n"), 0644)
	require.NoError(err)

	rootCmd.SetArgs([]string{base})
	err = rootCmd.Execute()
	require.Error(err)
	require.Contains(err.Error(), "compose files")
	require.Contains(err.Error(), "invalid layer file name")
}

func TestRootCmdFailsWhenCreateOutputDirectoryFails(t *testing.T) {
	setupRootCmdDeps(t)
	require := require.New(t)

	tmpDir, base := writeRootCmdTestFiles(t)
	mkdirAll = func(string, os.FileMode) error {
		return errors.New("mkdir failed")
	}
	out := filepath.Join(tmpDir, "nested", "out.yaml")

	rootCmd.SetArgs([]string{base, "-o", out})
	err := rootCmd.Execute()
	require.Error(err)
	require.Contains(err.Error(), "create output directory")
	require.Contains(err.Error(), "mkdir failed")
}

func TestRootCmdFailsWhenWriteOutputFails(t *testing.T) {
	setupRootCmdDeps(t)
	require := require.New(t)

	tmpDir, base := writeRootCmdTestFiles(t)
	writeFile = func(string, []byte, os.FileMode) error {
		return errors.New("write failed")
	}

	out := filepath.Join(tmpDir, "out.yaml")
	rootCmd.SetArgs([]string{base, "-o", out})
	err := rootCmd.Execute()
	require.Error(err)
	require.Contains(err.Error(), "write output file")
	require.Contains(err.Error(), "write failed")
}

func TestExecute(t *testing.T) {
	setupRootCmdDeps(t)
	require := require.New(t)

	originalExecuteRootCmd := executeRootCmd
	originalExitProcess := exitProcess
	t.Cleanup(func() {
		executeRootCmd = originalExecuteRootCmd
		exitProcess = originalExitProcess
	})

	called := false
	executeRootCmd = func() error {
		called = true
		return nil
	}
	exitProcess = func(int) {
		require.Fail("exitProcess should not be called on success")
	}

	Execute()
	require.True(called)
}

func TestExecuteUsesDefaultRootCommandExecutor(t *testing.T) {
	setupRootCmdDeps(t)
	require := require.New(t)

	originalExitProcess := exitProcess
	t.Cleanup(func() {
		exitProcess = originalExitProcess
	})

	tmpDir, base := writeRootCmdTestFiles(t)
	out := filepath.Join(tmpDir, "out.yaml")
	rootCmd.SetArgs([]string{base, "-o", out})
	exitProcess = func(int) {
		require.Fail("exitProcess should not be called on success")
	}

	Execute()
	b, err := os.ReadFile(out)
	require.NoError(err)
	require.Contains(string(b), "service: layer")
}

func TestExecuteExitsOnError(t *testing.T) {
	setupRootCmdDeps(t)
	require := require.New(t)

	originalExecuteRootCmd := executeRootCmd
	originalExitProcess := exitProcess
	t.Cleanup(func() {
		executeRootCmd = originalExecuteRootCmd
		exitProcess = originalExitProcess
	})

	executeRootCmd = func() error {
		return errors.New("boom")
	}
	exitCode := -1
	exitProcess = func(code int) {
		exitCode = code
	}

	Execute()
	require.Equal(1, exitCode)
}
