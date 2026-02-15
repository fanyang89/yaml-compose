package cmd

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"

	"github.com/spf13/cobra"

	"github.com/fanyang89/yaml-compose/v1/compose"
	"github.com/fanyang89/yaml-compose/v1/fsutils"
)

var (
	flagOutput string
)

var rootCmd = &cobra.Command{
	Use:  "yaml-compose [YAML-FILE]",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		base := args[0]
		exists, err := fsutils.FileExists(base)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		if !exists {
			log.Fatalf("Error: %v not found", base)
		}

		baseDir := base + ".d"
		exists, err = fsutils.DirExists(baseDir)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		if !exists {
			log.Fatalf("Error: %v not found", baseDir)
		}

		layerInfos, err := ioutil.ReadDir(baseDir)
		layers := make([]string, 0)
		for _, info := range layerInfos {
			if strings.HasSuffix(info.Name(), ".yaml") || strings.HasSuffix(info.Name(), ".yml") {
				layers = append(layers, info.Name())
			}
		}

		c := compose.New(base, layers)
		ret, err := c.Run()
		if err != nil {
			log.Fatalf("Error: %v", err)
		}

		if flagOutput != "" {
			outputBaseDir := path.Dir(flagOutput)
			if err := os.MkdirAll(outputBaseDir, 0755); err != nil {
				log.Fatalf("Error: %v", err)
			}
			err = ioutil.WriteFile(flagOutput, []byte(ret), 0644)
			if err != nil {
				log.Fatalf("Error: %v", err)
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
}
