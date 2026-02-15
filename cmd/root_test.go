package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

type fakeComposer struct {
	run func() (string, error)
}

func (f fakeComposer) Run() (string, error) {
	return f.run()
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

func newTestRootCmd(overrides func(*commandDeps)) *cobra.Command {
	deps := defaultCommandDeps()
	if overrides != nil {
		overrides(&deps)
	}
	return newRootCmd(deps)
}

func TestRootCmdWritesOutputFile(t *testing.T) {
	require := require.New(t)

	tmpDir, base := writeRootCmdTestFiles(t)
	out := filepath.Join(tmpDir, "out.yaml")

	cmd := newTestRootCmd(nil)
	cmd.SetArgs([]string{base, "-o", out})
	err := cmd.Execute()
	require.NoError(err)

	b, err := os.ReadFile(out)
	require.NoError(err)
	require.Contains(string(b), "service: layer")
}

func TestRootCmdPrintsWhenOutputFlagMissing(t *testing.T) {
	require := require.New(t)

	_, base := writeRootCmdTestFiles(t)
	printed := ""
	cmd := newTestRootCmd(func(deps *commandDeps) {
		deps.printLine = func(args ...interface{}) (int, error) {
			printed = fmt.Sprint(args...)
			return len(printed), nil
		}
	})

	cmd.SetArgs([]string{base})
	err := cmd.Execute()
	require.NoError(err)
	require.Contains(printed, "service: layer")
}

func TestRootCmdFailsWhenBaseMissing(t *testing.T) {
	require := require.New(t)

	base := filepath.Join(t.TempDir(), "missing.yaml")
	cmd := newTestRootCmd(nil)
	cmd.SetArgs([]string{base})
	err := cmd.Execute()
	require.Error(err)
	require.Contains(err.Error(), "not found")
}

func TestRootCmdFailsWhenBaseCheckFails(t *testing.T) {
	require := require.New(t)

	cmd := newTestRootCmd(func(deps *commandDeps) {
		deps.fileExists = func(string) (bool, error) {
			return false, errors.New("stat failed")
		}
	})

	cmd.SetArgs([]string{"base.yaml"})
	err := cmd.Execute()
	require.Error(err)
	require.Contains(err.Error(), "check base file")
	require.Contains(err.Error(), "stat failed")
}

func TestRootCmdFailsWhenLayerDirMissing(t *testing.T) {
	require := require.New(t)

	base := filepath.Join(t.TempDir(), "a.yaml")
	err := os.WriteFile(base, []byte("service: base\n"), 0644)
	require.NoError(err)

	cmd := newTestRootCmd(nil)
	cmd.SetArgs([]string{base})
	err = cmd.Execute()
	require.Error(err)
	require.Contains(err.Error(), "not found")
	require.Contains(err.Error(), base+".d")
}

func TestRootCmdFailsWhenLayerPathIsFile(t *testing.T) {
	require := require.New(t)

	tmpDir := t.TempDir()
	base := filepath.Join(tmpDir, "a.yaml")
	err := os.WriteFile(base, []byte("service: base\n"), 0644)
	require.NoError(err)
	err = os.WriteFile(base+".d", []byte("not a dir\n"), 0644)
	require.NoError(err)

	cmd := newTestRootCmd(nil)
	cmd.SetArgs([]string{base})
	err = cmd.Execute()
	require.Error(err)
	require.Contains(err.Error(), "check layer directory")
	require.Contains(err.Error(), "not a directory")
}

func TestRootCmdFailsWhenReadLayerDirectoryFails(t *testing.T) {
	require := require.New(t)

	base := filepath.Join(t.TempDir(), "a.yaml")
	err := os.WriteFile(base, []byte("service: base\n"), 0644)
	require.NoError(err)
	cmd := newTestRootCmd(func(deps *commandDeps) {
		deps.dirExists = func(string) (bool, error) {
			return true, nil
		}
		deps.readDir = func(string) ([]os.DirEntry, error) {
			return nil, errors.New("read failed")
		}
	})

	cmd.SetArgs([]string{base})
	err = cmd.Execute()
	require.Error(err)
	require.Contains(err.Error(), "read layer directory")
	require.Contains(err.Error(), "read failed")
}

func TestRootCmdIncludesYAMLAndYMLExtensionsOnly(t *testing.T) {
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
	cmd := newTestRootCmd(nil)
	cmd.SetArgs([]string{base, "-o", out})
	err = cmd.Execute()
	require.NoError(err)

	b, err := os.ReadFile(out)
	require.NoError(err)
	require.Contains(string(b), "service: yml")
	require.NotContains(string(b), "ignored")
}

func TestRootCmdFailsWhenComposeFails(t *testing.T) {
	require := require.New(t)

	base := filepath.Join(t.TempDir(), "a.yaml")
	err := os.WriteFile(base, []byte("service: base\n"), 0644)
	require.NoError(err)
	cmd := newTestRootCmd(func(deps *commandDeps) {
		deps.dirExists = func(string) (bool, error) {
			return true, nil
		}
		deps.readDir = func(string) ([]os.DirEntry, error) {
			return []os.DirEntry{}, nil
		}
		deps.newCompose = func(string, []string) composeRunner {
			return fakeComposer{run: func() (string, error) {
				return "", errors.New("compose failed")
			}}
		}
	})

	cmd.SetArgs([]string{base})
	err = cmd.Execute()
	require.Error(err)
	require.Contains(err.Error(), "compose files")
	require.Contains(err.Error(), "compose failed")
}

func TestRootCmdFailsWhenCreateOutputDirectoryFails(t *testing.T) {
	require := require.New(t)

	tmpDir, base := writeRootCmdTestFiles(t)
	out := filepath.Join(tmpDir, "nested", "out.yaml")
	cmd := newTestRootCmd(func(deps *commandDeps) {
		deps.mkdirAll = func(string, os.FileMode) error {
			return errors.New("mkdir failed")
		}
	})

	cmd.SetArgs([]string{base, "-o", out})
	err := cmd.Execute()
	require.Error(err)
	require.Contains(err.Error(), "create output directory")
	require.Contains(err.Error(), "mkdir failed")
}

func TestRootCmdFailsWhenWriteOutputFails(t *testing.T) {
	require := require.New(t)

	tmpDir, base := writeRootCmdTestFiles(t)
	out := filepath.Join(tmpDir, "out.yaml")
	cmd := newTestRootCmd(func(deps *commandDeps) {
		deps.writeFile = func(string, []byte, os.FileMode) error {
			return errors.New("write failed")
		}
	})

	cmd.SetArgs([]string{base, "-o", out})
	err := cmd.Execute()
	require.Error(err)
	require.Contains(err.Error(), "write output file")
	require.Contains(err.Error(), "write failed")
}

func TestCollectLayerFilenames(t *testing.T) {
	require := require.New(t)

	tmpDir := t.TempDir()
	err := os.WriteFile(filepath.Join(tmpDir, "1-a.yaml"), []byte("a: 1\n"), 0644)
	require.NoError(err)
	err = os.WriteFile(filepath.Join(tmpDir, "2-b.yml"), []byte("b: 2\n"), 0644)
	require.NoError(err)
	err = os.WriteFile(filepath.Join(tmpDir, "3-c.txt"), []byte("c: 3\n"), 0644)
	require.NoError(err)

	entries, err := os.ReadDir(tmpDir)
	require.NoError(err)
	layers := collectLayerFilenames(entries)
	require.ElementsMatch([]string{"1-a.yaml", "2-b.yml"}, layers)
}

func TestExecuteRunsCommandExecutor(t *testing.T) {
	require := require.New(t)

	called := false
	execute(func() error {
		called = true
		return nil
	}, func(int) {
		require.Fail("exit should not be called")
	})

	require.True(called)
}

func TestExecuteCallsExitOnError(t *testing.T) {
	require := require.New(t)

	exitCode := -1
	execute(func() error {
		return errors.New("boom")
	}, func(code int) {
		exitCode = code
	})

	require.Equal(1, exitCode)
}

func TestExecuteUsesRootCommand(t *testing.T) {
	require := require.New(t)

	originalRootCmd := rootCmd
	t.Cleanup(func() {
		rootCmd = originalRootCmd
	})

	tmpDir, base := writeRootCmdTestFiles(t)
	out := filepath.Join(tmpDir, "out.yaml")
	rootCmd = newRootCmd(defaultCommandDeps())
	rootCmd.SetArgs([]string{base, "-o", out})

	Execute()
	b, err := os.ReadFile(out)
	require.NoError(err)
	require.Contains(string(b), "service: layer")
}
