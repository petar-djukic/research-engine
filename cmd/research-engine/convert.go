package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var convertCmd = &cobra.Command{
	Use:   "convert [papers...]",
	Short: "Convert PDF files to structured Markdown",
	Long: `Convert transforms PDF files into structured Markdown that preserves
section hierarchy, paragraphs, and reference lists. Supports GROBID,
pdftotext, and markitdown (container-based) backends.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(os.Stderr, "convert: not yet implemented")
		return fmt.Errorf("not yet implemented")
	},
}

func init() {
	convertCmd.Flags().String("backend", "markitdown", "conversion backend: grobid, pdftotext, or markitdown")
	convertCmd.Flags().String("papers-dir", "papers", "base directory for papers")
	convertCmd.Flags().Bool("batch", false, "process all unconverted papers in papers-dir")

	rootCmd.AddCommand(convertCmd)
}
