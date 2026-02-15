package cmd

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

type fakeComposer struct {
	run        func() (string, error)
	setExtract func(string)
}

type errorWriter struct{}

func (errorWriter) Write([]byte) (int, error) {
	return 0, errors.New("write stdout failed")
}

func (f fakeComposer) Run() (string, error) {
	return f.run()
}

func (f fakeComposer) SetExtractLayerPath(path string) {
	if f.setExtract != nil {
		f.setExtract(path)
	}
}

func setupComposeFiles(t *testing.T, fs afero.Fs) string {
	t.Helper()

	base := "/base.yaml"
	setupComposeFilesAt(t, fs, base)
	return base
}

func setupComposeFilesAt(t *testing.T, fs afero.Fs, base string) {
	t.Helper()

	err := afero.WriteFile(fs, base, []byte("service: base\n"), 0644)
	require.NoError(t, err)
	err = fs.MkdirAll(base+".d", 0755)
	require.NoError(t, err)
	err = afero.WriteFile(fs, base+".d/1-layer.yaml", []byte("service: layer\n"), 0644)
	require.NoError(t, err)
}

func newTestRootCmd(fs afero.Fs, stdout io.Writer, override func(*commandDeps)) *cobra.Command {
	deps := defaultCommandDeps()
	deps.fs = fs
	deps.stdout = stdout
	if override != nil {
		override(&deps)
	}
	cmd := newRootCmd(deps)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetErr(io.Discard)
	return cmd
}

func TestRootCmdWritesOutputFile(t *testing.T) {
	require := require.New(t)
	fs := afero.NewMemMapFs()
	base := setupComposeFiles(t, fs)

	cmd := newTestRootCmd(fs, io.Discard, nil)
	cmd.SetArgs([]string{base, "-o", "/out.yaml"})
	err := cmd.Execute()
	require.NoError(err)

	b, err := afero.ReadFile(fs, "/out.yaml")
	require.NoError(err)
	require.Contains(string(b), "service: layer")
}

func TestRootCmdPrintsWhenOutputFlagMissing(t *testing.T) {
	require := require.New(t)
	fs := afero.NewMemMapFs()
	base := setupComposeFiles(t, fs)
	var out bytes.Buffer

	cmd := newTestRootCmd(fs, &out, nil)
	cmd.SetArgs([]string{base})
	err := cmd.Execute()
	require.NoError(err)
	require.Contains(out.String(), "service: layer")
}

func TestRootCmdExtractLayer(t *testing.T) {
	require := require.New(t)
	fs := afero.NewMemMapFs()

	err := afero.WriteFile(fs, "/base.yaml", []byte("app:\n  db:\n    host: base\n    pool: 10\nkeep: true\n"), 0644)
	require.NoError(err)
	err = fs.MkdirAll("/base.yaml.d", 0755)
	require.NoError(err)
	err = afero.WriteFile(fs, "/base.yaml.d/1-layer.yaml", []byte("app:\n  db:\n    host: layer\nunrelated: x\n"), 0644)
	require.NoError(err)

	cmd := newTestRootCmd(fs, io.Discard, nil)
	cmd.SetArgs([]string{"/base.yaml", "-e", "app.db", "-o", "/out.yaml"})
	err = cmd.Execute()
	require.NoError(err)

	b, err := afero.ReadFile(fs, "/out.yaml")
	require.NoError(err)
	content := string(b)
	require.Contains(content, "host: layer")
	require.Contains(content, "pool: 10")
	require.Contains(content, "keep: true")
	require.NotContains(content, "unrelated")
}

func TestRootCmdPassesExtractLayerToComposer(t *testing.T) {
	require := require.New(t)
	fs := afero.NewMemMapFs()
	base := setupComposeFiles(t, fs)

	gotPath := ""
	cmd := newTestRootCmd(fs, io.Discard, func(deps *commandDeps) {
		deps.newCompose = func(string, []string, afero.Fs) composeRunner {
			return fakeComposer{
				setExtract: func(path string) {
					gotPath = path
				},
				run: func() (string, error) {
					return "service: layer\n", nil
				},
			}
		}
	})

	cmd.SetArgs([]string{base, "-e", "app.db", "-o", "/out.yaml"})
	err := cmd.Execute()
	require.NoError(err)
	require.Equal("app.db", gotPath)
}

func TestRootCmdFailsWhenPrintOutputFails(t *testing.T) {
	require := require.New(t)
	fs := afero.NewMemMapFs()
	base := setupComposeFiles(t, fs)

	cmd := newTestRootCmd(fs, errorWriter{}, nil)
	cmd.SetArgs([]string{base})
	err := cmd.Execute()
	require.Error(err)
	require.Contains(err.Error(), "print output")
}

func TestRootCmdFailsWhenBaseMissing(t *testing.T) {
	require := require.New(t)
	fs := afero.NewMemMapFs()

	cmd := newTestRootCmd(fs, io.Discard, nil)
	cmd.SetArgs([]string{"/missing.yaml"})
	err := cmd.Execute()
	require.Error(err)
	require.Contains(err.Error(), "not found")
}

func TestRootCmdFailsWhenBaseCheckFails(t *testing.T) {
	require := require.New(t)
	fs := afero.NewOsFs()

	cmd := newTestRootCmd(fs, io.Discard, nil)
	cmd.SetArgs([]string{"bad\x00.yaml"})
	err := cmd.Execute()
	require.Error(err)
	require.Contains(err.Error(), "check base file")
}

func TestRootCmdFailsWhenLayerDirMissing(t *testing.T) {
	require := require.New(t)
	fs := afero.NewMemMapFs()
	err := afero.WriteFile(fs, "/base.yaml", []byte("service: base\n"), 0644)
	require.NoError(err)

	cmd := newTestRootCmd(fs, io.Discard, nil)
	cmd.SetArgs([]string{"/base.yaml"})
	err = cmd.Execute()
	require.Error(err)
	require.Contains(err.Error(), "/base.yaml.d not found")
}

func TestRootCmdFailsWhenLayerPathIsFile(t *testing.T) {
	require := require.New(t)
	fs := afero.NewMemMapFs()
	err := afero.WriteFile(fs, "/base.yaml", []byte("service: base\n"), 0644)
	require.NoError(err)
	err = afero.WriteFile(fs, "/base.yaml.d", []byte("not a dir\n"), 0644)
	require.NoError(err)

	cmd := newTestRootCmd(fs, io.Discard, nil)
	cmd.SetArgs([]string{"/base.yaml"})
	err = cmd.Execute()
	require.Error(err)
	require.Contains(err.Error(), "check layer directory")
	require.Contains(err.Error(), "not a directory")
}

func TestRootCmdFailsWhenReadLayerDirectoryFails(t *testing.T) {
	require := require.New(t)
	fs := afero.NewOsFs()
	tmpDir := t.TempDir()
	base := filepath.Join(tmpDir, "base.yaml")
	setupComposeFilesAt(t, fs, base)

	baseDir := base + ".d"
	err := os.Chmod(baseDir, 0111)
	require.NoError(err)
	t.Cleanup(func() {
		_ = os.Chmod(baseDir, 0755)
	})

	cmd := newTestRootCmd(fs, io.Discard, nil)
	cmd.SetArgs([]string{base})
	err = cmd.Execute()
	require.Error(err)
	require.Contains(err.Error(), "read layer directory")
}

func TestRootCmdIncludesYAMLAndYMLExtensionsOnly(t *testing.T) {
	require := require.New(t)
	fs := afero.NewMemMapFs()
	err := afero.WriteFile(fs, "/base.yaml", []byte("service: base\n"), 0644)
	require.NoError(err)
	err = fs.MkdirAll("/base.yaml.d", 0755)
	require.NoError(err)
	err = afero.WriteFile(fs, "/base.yaml.d/1-layer.yaml", []byte("service: yaml\n"), 0644)
	require.NoError(err)
	err = afero.WriteFile(fs, "/base.yaml.d/2-layer.yml", []byte("service: yml\n"), 0644)
	require.NoError(err)
	err = afero.WriteFile(fs, "/base.yaml.d/3-layer.txt", []byte("service: ignored\n"), 0644)
	require.NoError(err)

	cmd := newTestRootCmd(fs, io.Discard, nil)
	cmd.SetArgs([]string{"/base.yaml", "-o", "/out.yaml"})
	err = cmd.Execute()
	require.NoError(err)

	b, err := afero.ReadFile(fs, "/out.yaml")
	require.NoError(err)
	require.Contains(string(b), "service: yml")
	require.NotContains(string(b), "ignored")
}

func TestRootCmdFailsWhenComposeFails(t *testing.T) {
	require := require.New(t)
	fs := afero.NewMemMapFs()
	base := setupComposeFiles(t, fs)

	cmd := newTestRootCmd(fs, io.Discard, func(deps *commandDeps) {
		deps.newCompose = func(string, []string, afero.Fs) composeRunner {
			return fakeComposer{run: func() (string, error) {
				return "", errors.New("compose failed")
			}}
		}
	})
	cmd.SetArgs([]string{base})
	err := cmd.Execute()
	require.Error(err)
	require.Contains(err.Error(), "compose files")
	require.Contains(err.Error(), "compose failed")
}

func TestRootCmdFailsWhenCreateOutputDirectoryFails(t *testing.T) {
	require := require.New(t)
	mem := afero.NewMemMapFs()
	base := setupComposeFiles(t, mem)
	fs := afero.NewReadOnlyFs(mem)

	cmd := newTestRootCmd(fs, io.Discard, nil)
	cmd.SetArgs([]string{base, "-o", "/nested/out.yaml"})
	err := cmd.Execute()
	require.Error(err)
	require.Contains(err.Error(), "create output directory")
}

func TestRootCmdFailsWhenWriteOutputFails(t *testing.T) {
	require := require.New(t)
	fs := afero.NewOsFs()
	tmpDir := t.TempDir()
	base := filepath.Join(tmpDir, "base.yaml")
	setupComposeFilesAt(t, fs, base)

	cmd := newTestRootCmd(fs, io.Discard, nil)
	cmd.SetArgs([]string{base, "-o", "bad\x00.yaml"})
	err := cmd.Execute()
	require.Error(err)
	require.Contains(err.Error(), "write output file")
}

func TestCollectLayerFilenames(t *testing.T) {
	require := require.New(t)
	fs := afero.NewMemMapFs()
	err := afero.WriteFile(fs, "/1-a.yaml", []byte("a: 1\n"), 0644)
	require.NoError(err)
	err = afero.WriteFile(fs, "/2-b.yml", []byte("b: 2\n"), 0644)
	require.NoError(err)
	err = afero.WriteFile(fs, "/3-c.txt", []byte("c: 3\n"), 0644)
	require.NoError(err)

	infos, err := afero.ReadDir(fs, "/")
	require.NoError(err)
	layers := collectLayerFilenames(infos)
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
	originalExit := osExit
	t.Cleanup(func() {
		rootCmd = originalRootCmd
		osExit = originalExit
	})

	fs := afero.NewMemMapFs()
	base := setupComposeFiles(t, fs)
	rootCmd = newTestRootCmd(fs, io.Discard, nil)
	rootCmd.SetArgs([]string{base, "-o", "/out.yaml"})
	osExit = func(int) {
		require.Fail("osExit should not be called")
	}

	Execute()
	b, err := afero.ReadFile(fs, "/out.yaml")
	require.NoError(err)
	require.Contains(string(b), "service: layer")
}
