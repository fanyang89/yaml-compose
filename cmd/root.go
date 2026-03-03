package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	"github.com/fanyang89/yaml-compose/v1/compose"
	"github.com/fanyang89/yaml-compose/v1/fsutils"
)

type composeRunner interface {
	Run() (string, error)
}

type commandDeps struct {
	fs         afero.Fs
	stdout     io.Writer
	newCompose func(string, []string, afero.Fs) composeRunner
}

func defaultCommandDeps() commandDeps {
	return commandDeps{
		fs:     afero.NewOsFs(),
		stdout: os.Stdout,
		newCompose: func(base string, layers []string, fs afero.Fs) composeRunner {
			return compose.NewWithFs(base, layers, fs)
		},
	}
}

func newRootCmd(deps commandDeps) *cobra.Command {
	flagOutput := ""
	cmd := &cobra.Command{
		Use:  "yaml-compose [YAML-FILE]",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRootCommand(args[0], flagOutput, deps)
		},
	}

	cmd.Flags().StringVarP(&flagOutput, "output", "o", "", "config file")
	return cmd
}

func runRootCommand(base, output string, deps commandDeps) error {
	exists, err := fsutils.FileExistsOn(deps.fs, base)
	if err != nil {
		return fmt.Errorf("check base file: %w", err)
	}
	if !exists {
		return fmt.Errorf("%s not found", base)
	}

	baseDir := base + ".d"
	exists, err = fsutils.DirExistsOn(deps.fs, baseDir)
	if err != nil {
		return fmt.Errorf("check layer directory: %w", err)
	}
	if !exists {
		return fmt.Errorf("%s not found", baseDir)
	}

	layerInfos, err := afero.ReadDir(deps.fs, baseDir)
	if err != nil {
		return fmt.Errorf("read layer directory: %w", err)
	}
	layers := collectLayerFilenames(layerInfos)

	c := deps.newCompose(base, layers, deps.fs)
	ret, err := c.Run()
	if err != nil {
		return fmt.Errorf("compose files: %w", err)
	}

	if output != "" {
		outputBaseDir := filepath.Dir(output)
		if err := deps.fs.MkdirAll(outputBaseDir, 0755); err != nil {
			return fmt.Errorf("create output directory: %w", err)
		}
		err = afero.WriteFile(deps.fs, output, []byte(ret), 0644)
		if err != nil {
			return fmt.Errorf("write output file: %w", err)
		}
		return nil
	}

	if _, err := fmt.Fprintln(deps.stdout, ret); err != nil {
		return fmt.Errorf("print output: %w", err)
	}
	return nil
}

func collectLayerFilenames(layerInfos []os.FileInfo) []string {
	layers := make([]string, 0)
	for _, info := range layerInfos {
		if strings.HasSuffix(info.Name(), ".yaml") || strings.HasSuffix(info.Name(), ".yml") {
			layers = append(layers, info.Name())
		}
	}
	return layers
}

func execute(commandExecutor func() error, exit func(int)) {
	err := commandExecutor()
	if err != nil {
		exit(1)
	}
}

var rootCmd = newRootCmd(defaultCommandDeps())

var osExit = os.Exit

func Execute() {
	execute(rootCmd.Execute, osExit)
}
