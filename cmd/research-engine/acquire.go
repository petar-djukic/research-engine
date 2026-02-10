package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var acquireCmd = &cobra.Command{
	Use:   "acquire [identifiers...]",
	Short: "Download papers from URLs, DOIs, or arXiv IDs",
	Long: `Acquire resolves paper identifiers (arXiv IDs, DOIs, direct PDF URLs)
to PDF files, downloads them, and creates metadata records. Existing papers
are skipped.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(os.Stderr, "acquire: not yet implemented")
		return fmt.Errorf("not yet implemented")
	},
}

func init() {
	acquireCmd.Flags().Duration("timeout", 0, "HTTP request timeout (default 60s)")
	acquireCmd.Flags().Duration("delay", 0, "delay between consecutive downloads (default 1s)")
	acquireCmd.Flags().String("papers-dir", "papers", "base directory for papers")

	rootCmd.AddCommand(acquireCmd)
}
