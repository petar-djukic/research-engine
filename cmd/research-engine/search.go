package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search academic APIs for candidate papers",
	Long: `Search queries academic APIs (arXiv, Semantic Scholar) for papers matching
a research question or structured query parameters. Results are deduplicated
across sources and ranked by relevance.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(os.Stderr, "search: not yet implemented")
		return fmt.Errorf("not yet implemented")
	},
}

func init() {
	searchCmd.Flags().String("query", "", "free-text research question")
	searchCmd.Flags().String("author", "", "filter by author name")
	searchCmd.Flags().String("keywords", "", "filter by keywords (comma-separated)")
	searchCmd.Flags().String("from", "", "publication date range start (YYYY-MM-DD)")
	searchCmd.Flags().String("to", "", "publication date range end (YYYY-MM-DD)")
	searchCmd.Flags().Int("max-results", 20, "maximum number of results to return")
	searchCmd.Flags().Bool("json", false, "output results as JSON")
	searchCmd.Flags().Bool("recency-bias", false, "boost recently published papers")

	rootCmd.AddCommand(searchCmd)
}
