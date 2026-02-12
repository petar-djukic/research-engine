// Copyright Mesh Intelligence Inc., 2026. All rights reserved.

package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/pdiddy/research-engine/internal/search"
	"github.com/pdiddy/research-engine/pkg/types"
)

const defaultSearchTimeout = 30 * time.Second

var searchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search academic APIs for candidate papers",
	Long: `Search queries academic APIs (arXiv, Semantic Scholar, OpenAlex) for papers
matching a research question or structured query parameters. Results are
deduplicated across sources and ranked by relevance.

Use --query-file to save results to a YAML file for later review. When
--query-file is provided without a query, the saved results are displayed.

Use --csl to output results in CSL YAML format for Pandoc and reference managers.`,
	RunE: runSearch,
}

func init() {
	searchCmd.Flags().String("query", "", "free-text research question")
	searchCmd.Flags().String("author", "", "filter by author name")
	searchCmd.Flags().String("keywords", "", "filter by keywords (comma-separated)")
	searchCmd.Flags().String("from", "", "publication date range start (YYYY-MM-DD)")
	searchCmd.Flags().String("to", "", "publication date range end (YYYY-MM-DD)")
	searchCmd.Flags().Int("max-results", 20, "maximum number of results to return")
	searchCmd.Flags().Bool("json", false, "output results as JSON")
	searchCmd.Flags().Bool("csl", false, "output results as CSL YAML for reference managers")
	searchCmd.Flags().Bool("recency-bias", false, "boost recently published papers")
	searchCmd.Flags().String("query-file", "", "YAML file to save/load query and results")

	rootCmd.AddCommand(searchCmd)
}

func runSearch(cmd *cobra.Command, args []string) error {
	queryText, _ := cmd.Flags().GetString("query")
	author, _ := cmd.Flags().GetString("author")
	keywords, _ := cmd.Flags().GetString("keywords")
	fromStr, _ := cmd.Flags().GetString("from")
	toStr, _ := cmd.Flags().GetString("to")
	maxResults, _ := cmd.Flags().GetInt("max-results")
	jsonOutput, _ := cmd.Flags().GetBool("json")
	cslOutput, _ := cmd.Flags().GetBool("csl")
	recencyBias, _ := cmd.Flags().GetBool("recency-bias")
	queryFile, _ := cmd.Flags().GetString("query-file")

	// If no --query flag, use positional args as the query.
	if queryText == "" && len(args) > 0 {
		queryText = strings.Join(args, " ")
	}

	hasQuery := queryText != "" || author != "" || keywords != "" || fromStr != "" || toStr != ""

	// Load from query file when no query is provided (R4.6).
	if queryFile != "" && !hasQuery {
		return loadAndDisplayQueryFile(queryFile, jsonOutput, cslOutput)
	}

	query := search.Query{
		FreeText: queryText,
		Author:   author,
	}
	if keywords != "" {
		for _, kw := range strings.Split(keywords, ",") {
			kw = strings.TrimSpace(kw)
			if kw != "" {
				query.Keywords = append(query.Keywords, kw)
			}
		}
	}
	if fromStr != "" {
		t, err := time.Parse("2006-01-02", fromStr)
		if err != nil {
			return fmt.Errorf("invalid --from date %q: use YYYY-MM-DD", fromStr)
		}
		query.DateFrom = t
	}
	if toStr != "" {
		t, err := time.Parse("2006-01-02", toStr)
		if err != nil {
			return fmt.Errorf("invalid --to date %q: use YYYY-MM-DD", toStr)
		}
		query.DateTo = t
	}

	cfg := types.SearchConfig{
		HTTPConfig: types.HTTPConfig{
			Timeout:   defaultSearchTimeout,
			UserAgent: defaultUserAgent,
		},
		MaxResults:            maxResults,
		EnableArxiv:           true,
		EnableSemanticScholar: true,
		EnableOpenAlex:        true,
		InterBackendDelay:     1 * time.Second,
		RecencyBiasWindow:     2 * 365 * 24 * time.Hour,
	}

	client := &http.Client{Timeout: cfg.Timeout}

	var backends []search.Backend
	if cfg.EnableArxiv {
		backends = append(backends, &search.ArxivBackend{Client: client})
	}
	if cfg.EnableSemanticScholar {
		backends = append(backends, &search.SemanticScholarBackend{
			Client: client,
			APIKey: cfg.SemanticScholarAPIKey,
		})
	}
	if cfg.EnableOpenAlex {
		backends = append(backends, &search.OpenAlexBackend{
			Client: client,
			Email:  cfg.OpenAlexEmail,
		})
	}

	out, err := search.Search(context.Background(), query, backends, cfg, recencyBias, os.Stderr)
	if err != nil {
		return err
	}

	// Save to query file when --query-file is provided with a query (R4.6).
	if queryFile != "" {
		if err := search.WriteQueryFile(queryFile, query, cfg, recencyBias, out); err != nil {
			return fmt.Errorf("saving query file: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Saved query and %d results to %s\n", len(out.Results), queryFile)
	}

	return formatSearchOutput(out, jsonOutput, cslOutput)
}

func loadAndDisplayQueryFile(path string, jsonOutput, cslOutput bool) error {
	qf, err := search.ReadQueryFile(path)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Loaded %d results from %s (saved %s)\n",
		qf.Summary.Total, path, qf.Summary.Timestamp.Format("2006-01-02 15:04"))

	out := search.SearchOutput{
		Results:     qf.Results,
		DupsRemoved: qf.Summary.DuplicatesRemoved,
	}
	return formatSearchOutput(out, jsonOutput, cslOutput)
}

func formatSearchOutput(out search.SearchOutput, jsonOutput, cslOutput bool) error {
	if cslOutput {
		return search.FormatCSL(out, os.Stdout)
	}
	if jsonOutput {
		return search.FormatJSON(out, os.Stdout)
	}
	search.FormatTable(out, os.Stdout)
	return nil
}
