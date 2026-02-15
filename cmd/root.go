package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/fanyang89/yaml-compose/v1/compose"
	"github.com/fanyang89/yaml-compose/v1/fsutils"
)

type composeRunner interface {
	Run() (string, error)
}

type commandDeps struct {
	fileExists func(string) (bool, error)
	dirExists  func(string) (bool, error)
	readDir    func(string) ([]os.DirEntry, error)
	mkdirAll   func(string, os.FileMode) error
	writeFile  func(string, []byte, os.FileMode) error
	printLine  func(...interface{}) (int, error)
	newCompose func(string, []string) composeRunner
}

func defaultCommandDeps() commandDeps {
	return commandDeps{
		fileExists: fsutils.FileExists,
		dirExists:  fsutils.DirExists,
		readDir:    os.ReadDir,
		mkdirAll:   os.MkdirAll,
		writeFile:  os.WriteFile,
		printLine:  fmt.Println,
		newCompose: func(base string, layers []string) composeRunner {
			return compose.New(base, layers)
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
	exists, err := deps.fileExists(base)
	if err != nil {
		return fmt.Errorf("check base file: %w", err)
	}
	if !exists {
		return fmt.Errorf("%s not found", base)
	}

	baseDir := base + ".d"
	exists, err = deps.dirExists(baseDir)
	if err != nil {
		return fmt.Errorf("check layer directory: %w", err)
	}
	if !exists {
		return fmt.Errorf("%s not found", baseDir)
	}

	layerInfos, err := deps.readDir(baseDir)
	if err != nil {
		return fmt.Errorf("read layer directory: %w", err)
	}
	layers := collectLayerFilenames(layerInfos)

	c := deps.newCompose(base, layers)
	ret, err := c.Run()
	if err != nil {
		return fmt.Errorf("compose files: %w", err)
	}

	if output != "" {
		outputBaseDir := filepath.Dir(output)
		if err := deps.mkdirAll(outputBaseDir, 0755); err != nil {
			return fmt.Errorf("create output directory: %w", err)
		}
		err = deps.writeFile(output, []byte(ret), 0644)
		if err != nil {
			return fmt.Errorf("write output file: %w", err)
		}
		return nil
	}

	deps.printLine(ret)
	return nil
}

func collectLayerFilenames(layerInfos []os.DirEntry) []string {
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

func Execute() {
	execute(rootCmd.Execute, os.Exit)
}
