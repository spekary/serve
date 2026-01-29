package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/goradd/gro/codegen"
	"github.com/spf13/cobra"
)

var (
	cfgPath    string
	schemaPath string
	key        string
	outputPath string
	extension  string

	rootCmd = &cobra.Command{
		Use:   "serve",
		Short: "Serve — build utilities for using the Serve web server framework",
	}
)

func main() {
	var err error
	outputPath, err = os.Getwd()
	extension = ".tpl.got"
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	//initGen()
	initParse()

	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: true,
	})
	logger := slog.New(handler)
	slog.SetDefault(logger)

	if err = rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func initGen() {
	// Shared flags for all subcommands
	genCmd := &cobra.Command{
		Use:   "gen",
		Short: "Generate form code from the schema",
		RunE: func(cmd *cobra.Command, args []string) error {
			if outputPath == "" {
				return fmt.Errorf("missing required flag: -o/--output")
			}
			if schemaPath == "" {
				return fmt.Errorf("missing required flag: -s/--schema")
			}
			return codegen.Generate(schemaPath, outputPath)
		},
	}

	genCmd.Flags().StringVarP(&schemaPath, "schema", "s", "", "Path to schema file (required)")

	genCmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output directory for generated code")

	rootCmd.AddCommand(genCmd)
}

func initParse() {
	parseCmd := &cobra.Command{
		Use:   "parse",
		Short: "Parse html files and output GOT templates for the Serve framework",
		RunE: func(cmd *cobra.Command, args []string) error {
			files, err := expandArgs(args) // for os's that do not glob expand arguments (windows)
			if err != nil {
				return err
			}

			parse(outputPath, files...)
			return nil
		},
	}
	parseCmd.Flags().StringVarP(&outputPath, "dst", "d", outputPath, "Destination directory for generated code")
	parseCmd.Flags().StringVarP(&extension, "ext", "x", ".tpl.got", "Extension for template files generated")
	rootCmd.AddCommand(parseCmd)
}

func expandArgs(args []string) ([]string, error) {
	var out []string

	for _, arg := range args {
		matches, err := filepath.Glob(arg)
		if err != nil || matches == nil {
			// Not a glob, or no matches — keep original
			out = append(out, arg)
			continue
		}
		out = append(out, matches...)
	}

	return out, nil
}
