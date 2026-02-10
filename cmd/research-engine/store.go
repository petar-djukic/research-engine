package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var storeCmd = &cobra.Command{
	Use:   "store",
	Short: "Index knowledge items and query the knowledge base",
	Long: `Store ingests extracted knowledge items into a SQLite knowledge base,
builds a retrieval index, and supports full-text search and structured
queries by item type, tag, or paper.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(os.Stderr, "store: not yet implemented")
		return fmt.Errorf("not yet implemented")
	},
}

func init() {
	storeCmd.Flags().String("knowledge-dir", "knowledge", "base directory for knowledge (contains extracted/, index/)")
	storeCmd.Flags().Int("max-results", 20, "maximum number of query results")
	storeCmd.Flags().String("type", "", "filter by item type: claim, method, definition, result")
	storeCmd.Flags().String("tag", "", "filter by tag")
	storeCmd.Flags().String("paper", "", "filter by paper ID")
	storeCmd.Flags().String("query", "", "full-text search query")
	storeCmd.Flags().Bool("json", false, "output results as JSON")
	storeCmd.Flags().Bool("export", false, "export knowledge base to YAML and JSON files")

	rootCmd.AddCommand(storeCmd)
}
