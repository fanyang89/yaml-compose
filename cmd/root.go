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

var rootCmd = &cobra.Command{
	Use:  "yaml-compose [YAML-FILE]",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		base := args[0]
		exists, err := fsutils.FileExists(base)
		if err != nil {
			return fmt.Errorf("check base file: %w", err)
		}
		if !exists {
			return fmt.Errorf("%s not found", base)
		}

		baseDir := base + ".d"
		exists, err = fsutils.DirExists(baseDir)
		if err != nil {
			return fmt.Errorf("check layer directory: %w", err)
		}
		if !exists {
			return fmt.Errorf("%s not found", baseDir)
		}

		layerInfos, err := os.ReadDir(baseDir)
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
			if err := os.MkdirAll(outputBaseDir, 0755); err != nil {
				return fmt.Errorf("create output directory: %w", err)
			}
			err = os.WriteFile(flagOutput, []byte(ret), 0644)
			if err != nil {
				return fmt.Errorf("write output file: %w", err)
			}
		} else {
			fmt.Println(ret)
		}

		return nil
	},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringVarP(&flagOutput, "output", "o", "", "config file")
	rootCmd.Flags().StringVarP(&flagExtractLayer, "extract-layer", "e", "", "extract field path from each layer before compose")
}
