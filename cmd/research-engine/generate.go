package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate draft documents from knowledge base content",
	Long: `Generate produces draft document sections grounded in knowledge base
content with inline citations. Supports brainstorming queries, outline-driven
generation, and iterative refinement.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(os.Stderr, "generate: not yet implemented")
		return fmt.Errorf("not yet implemented")
	},
}

func init() {
	generateCmd.Flags().String("outline", "", "path to outline file (YAML or Markdown)")
	generateCmd.Flags().String("query", "", "brainstorming query or generation topic")
	generateCmd.Flags().String("model", "", "AI model identifier for generation")
	generateCmd.Flags().String("format", "markdown", "output format: markdown or latex")
	generateCmd.Flags().String("output-dir", "output/drafts", "directory for generated drafts")
	generateCmd.Flags().String("notes-dir", "output/notes", "directory for brainstorming notes")
	generateCmd.Flags().String("section", "", "regenerate a specific section from an existing draft")

	rootCmd.AddCommand(generateCmd)
}
