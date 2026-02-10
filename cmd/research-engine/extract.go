package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var extractCmd = &cobra.Command{
	Use:   "extract [papers...]",
	Short: "Extract typed knowledge items from converted papers",
	Long: `Extract reads structured Markdown and produces typed knowledge items
(claims, methods, definitions, results) with provenance links back to
the source paper, section, and page.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(os.Stderr, "extract: not yet implemented")
		return fmt.Errorf("not yet implemented")
	},
}

func init() {
	extractCmd.Flags().String("model", "", "AI model identifier for extraction")
	extractCmd.Flags().String("papers-dir", "papers", "base directory for papers (contains markdown/)")
	extractCmd.Flags().String("knowledge-dir", "knowledge", "base directory for knowledge output (contains extracted/)")
	extractCmd.Flags().Bool("batch", false, "process all unconverted papers in papers-dir")

	rootCmd.AddCommand(extractCmd)
}
