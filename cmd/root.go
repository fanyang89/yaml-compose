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

var (
	flagOutput       string
	flagExtractLayer string
)

var (
	fileExists     = fsutils.FileExists
	dirExists      = fsutils.DirExists
	readDir        = os.ReadDir
	mkdirAll       = os.MkdirAll
	writeFile      = os.WriteFile
	printLine      = fmt.Println
	executeRootCmd = func() error { return rootCmd.Execute() }
	exitProcess    = os.Exit
)

var rootCmd = &cobra.Command{
	Use:  "yaml-compose [YAML-FILE]",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		base := args[0]
		exists, err := fileExists(base)
		if err != nil {
			return fmt.Errorf("check base file: %w", err)
		}
		if !exists {
			return fmt.Errorf("%s not found", base)
		}

		baseDir := base + ".d"
		exists, err = dirExists(baseDir)
		if err != nil {
			return fmt.Errorf("check layer directory: %w", err)
		}
		if !exists {
			return fmt.Errorf("%s not found", baseDir)
		}

		layerInfos, err := readDir(baseDir)
		if err != nil {
			return fmt.Errorf("read layer directory: %w", err)
		}
		layers := make([]string, 0)
		for _, info := range layerInfos {
			if strings.HasSuffix(info.Name(), ".yaml") || strings.HasSuffix(info.Name(), ".yml") {
				layers = append(layers, info.Name())
			}
		}

		c := compose.New(base, layers)
		c.ExtractLayerPath = flagExtractLayer
		ret, err := c.Run()
		if err != nil {
			return fmt.Errorf("compose files: %w", err)
		}

		if flagOutput != "" {
			outputBaseDir := filepath.Dir(flagOutput)
			if err := mkdirAll(outputBaseDir, 0755); err != nil {
				return fmt.Errorf("create output directory: %w", err)
			}
			err = writeFile(flagOutput, []byte(ret), 0644)
			if err != nil {
				return fmt.Errorf("write output file: %w", err)
			}
		} else {
			printLine(ret)
		}

		return nil
	},
}

func Execute() {
	err := executeRootCmd()
	if err != nil {
		exitProcess(1)
	}
}

func init() {
	rootCmd.Flags().StringVarP(&flagOutput, "output", "o", "", "config file")
	rootCmd.Flags().StringVarP(&flagExtractLayer, "extract-layer", "e", "", "extract field path from each layer before compose")
}
