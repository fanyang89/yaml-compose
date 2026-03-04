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
	SetTransformLogWriter(io.Writer)
	SetTemplateVars(map[string]string)
	SetLayerDir(string)
}

type commandDeps struct {
	fs         afero.Fs
	stdout     io.Writer
	stderr     io.Writer
	newCompose func(string, []string, afero.Fs) composeRunner
}

func defaultCommandDeps() commandDeps {
	return commandDeps{
		fs:     afero.NewOsFs(),
		stdout: os.Stdout,
		stderr: os.Stderr,
		newCompose: func(base string, layers []string, fs afero.Fs) composeRunner {
			return compose.NewWithFs(base, layers, fs)
		},
	}
}

func newRootCmd(deps commandDeps) *cobra.Command {
	flagOutput := ""
	flagBase := ""
	flagLayerDir := ""
	flagLayer := ""
	flagVars := []string{}
	cmd := &cobra.Command{
		Use:  "yaml-compose [YAML-FILE]",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			base, err := resolveBasePath(args, flagBase)
			if err != nil {
				return err
			}
			return runRootCommand(base, flagLayerDir, flagOutput, flagLayer, flagVars, deps)
		},
	}
	cmd.SilenceUsage = true

	cmd.Flags().StringVar(&flagBase, "base", "", "base yaml file path")
	cmd.Flags().StringVar(&flagLayerDir, "layer-dir", "", "layer yaml directory path")
	cmd.Flags().StringVarP(&flagOutput, "output", "o", "", "config file")
	cmd.Flags().StringVar(&flagLayer, "layer", "", "run only one layer file (for debugging)")
	cmd.Flags().StringArrayVar(&flagVars, "var", nil, "template variable in KEY=VALUE format (repeatable)")
	return cmd
}

func runRootCommand(base, layerDir, output, layer string, rawVars []string, deps commandDeps) error {
	exists, err := fsutils.FileExistsOn(deps.fs, base)
	if err != nil {
		return fmt.Errorf("check base file: %w", err)
	}
	if !exists {
		return fmt.Errorf("%s not found", base)
	}

	resolvedLayerDir := layerDir
	if resolvedLayerDir == "" {
		resolvedLayerDir = base + ".d"
	}

	exists, err = fsutils.DirExistsOn(deps.fs, resolvedLayerDir)
	if err != nil {
		return fmt.Errorf("check layer directory: %w", err)
	}
	if !exists {
		return fmt.Errorf("%s not found", resolvedLayerDir)
	}

	layerInfos, err := afero.ReadDir(deps.fs, resolvedLayerDir)
	if err != nil {
		return fmt.Errorf("read layer directory: %w", err)
	}
	layers := collectLayerFilenames(layerInfos)
	if layer != "" {
		layers, err = filterLayersByName(layers, layer)
		if err != nil {
			return err
		}
	}

	templateVars, err := parseTemplateVars(rawVars)
	if err != nil {
		return err
	}

	c := deps.newCompose(base, layers, deps.fs)
	c.SetTransformLogWriter(deps.stderr)
	c.SetTemplateVars(templateVars)
	c.SetLayerDir(resolvedLayerDir)
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

func filterLayersByName(layers []string, target string) ([]string, error) {
	for _, layer := range layers {
		if layer == target {
			return []string{layer}, nil
		}
	}
	return nil, fmt.Errorf("layer %q not found", target)
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

func parseTemplateVars(rawVars []string) (map[string]string, error) {
	vars := make(map[string]string, len(rawVars))
	for _, raw := range rawVars {
		key, value, ok := strings.Cut(raw, "=")
		if !ok {
			return nil, fmt.Errorf("invalid --var %q: expected KEY=VALUE", raw)
		}
		if key == "" {
			return nil, fmt.Errorf("invalid --var %q: key cannot be empty", raw)
		}
		vars[key] = value
	}
	return vars, nil
}

func resolveBasePath(args []string, flagBase string) (string, error) {
	if len(args) == 1 && flagBase != "" {
		return "", fmt.Errorf("base path must be provided either as argument or --base, not both")
	}

	if flagBase != "" {
		return flagBase, nil
	}

	if len(args) == 1 {
		return args[0], nil
	}

	return "", fmt.Errorf("base path is required (argument or --base)")
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
