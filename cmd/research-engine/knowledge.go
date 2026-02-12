// Copyright Mesh Intelligence Inc., 2026. All rights reserved.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pdiddy/research-engine/internal/knowledge"
	"github.com/pdiddy/research-engine/pkg/types"
)

var knowledgeCmd = &cobra.Command{
	Use:   "knowledge",
	Short: "Manage the knowledge base (store, retrieve, export)",
	Long: `Knowledge manages a local SQLite knowledge base built from extracted
knowledge items. Use subcommands to index items, query them, or export.`,
}

// --- store subcommand ---

var knowledgeStoreCmd = &cobra.Command{
	Use:   "store",
	Short: "Ingest extracted knowledge items into the knowledge base",
	Long: `Store reads extraction YAML files from knowledge/extracted/, ingests
them into a SQLite database with FTS5 indexing, and writes an export file.
Unchanged papers are skipped on subsequent runs.`,
	RunE: runKnowledgeStore,
}

func runKnowledgeStore(cmd *cobra.Command, args []string) error {
	cfg, papersDir := knowledgeConfig(cmd)

	store, err := knowledge.NewStore(cfg, papersDir)
	if err != nil {
		return err
	}
	defer store.Close()

	summary, err := store.Ingest(context.Background(), os.Stdout)
	if err != nil {
		return err
	}
	if summary.Failed > 0 {
		return fmt.Errorf("%d paper(s) failed indexing", summary.Failed)
	}
	return nil
}

// --- retrieve subcommand ---

var knowledgeRetrieveCmd = &cobra.Command{
	Use:   "retrieve [query]",
	Short: "Query the knowledge base with full-text search and filters",
	Long: `Retrieve searches the knowledge base using FTS5 full-text search,
structured filters (type, tag, paper), or a combination of both.
Results include provenance links to the source paper and section.

Use --trace with an item ID to view the surrounding source context.`,
	RunE: runKnowledgeRetrieve,
}

func runKnowledgeRetrieve(cmd *cobra.Command, args []string) error {
	traceID, _ := cmd.Flags().GetString("trace")

	cfg, papersDir := knowledgeConfig(cmd)
	store, err := knowledge.NewStore(cfg, papersDir)
	if err != nil {
		return err
	}
	defer store.Close()

	// Trace mode: show source context for a specific item.
	if traceID != "" {
		text, err := store.Trace(context.Background(), traceID)
		if err != nil {
			return err
		}
		fmt.Println(text)
		return nil
	}

	opts := queryOptsFromFlags(cmd, args)
	if opts.IsEmpty() {
		return fmt.Errorf("query or filter required: provide a search query, --type, --tag, or --paper")
	}

	results, err := store.Retrieve(context.Background(), opts)
	if err != nil {
		return err
	}

	jsonOutput, _ := cmd.Flags().GetBool("json")
	return formatRetrieveOutput(results, jsonOutput)
}

func formatRetrieveOutput(results []knowledge.QueryResult, jsonOutput bool) error {
	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(results)
	}

	if len(results) == 0 {
		fmt.Println("No results found.")
		return nil
	}

	fmt.Fprintf(os.Stdout, "%-4s  %-8s  %-50s  %-20s  %-10s  %s\n",
		"Rank", "Type", "Content", "Paper", "Section", "Page")
	fmt.Fprintln(os.Stdout, strings.Repeat("-", 110))

	for i, r := range results {
		content := r.Content
		if len(content) > 50 {
			content = content[:47] + "..."
		}
		paper := r.PaperID
		if len(paper) > 20 {
			paper = paper[:17] + "..."
		}
		section := r.Section
		if len(section) > 10 {
			section = section[:7] + "..."
		}
		fmt.Fprintf(os.Stdout, "%-4d  %-8s  %-50s  %-20s  %-10s  %d\n",
			i+1, r.Type, content, paper, section, r.Page)
	}

	fmt.Fprintf(os.Stdout, "\n%d results\n", len(results))
	return nil
}

// --- export subcommand ---

var knowledgeExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export the knowledge base to YAML or JSON",
	Long: `Export writes the full knowledge base (or a filtered subset) to
knowledge/index/export.yaml or export.json. Supports the same filter
flags as retrieve for partial exports.`,
	RunE: runKnowledgeExport,
}

func runKnowledgeExport(cmd *cobra.Command, args []string) error {
	format, _ := cmd.Flags().GetString("format")

	cfg, papersDir := knowledgeConfig(cmd)
	store, err := knowledge.NewStore(cfg, papersDir)
	if err != nil {
		return err
	}
	defer store.Close()

	opts := queryOptsFromFlags(cmd, args)

	switch format {
	case "yaml", "":
		if err := store.ExportYAML(context.Background(), opts); err != nil {
			return err
		}
		fmt.Println("Exported to knowledge/index/export.yaml")
	case "json":
		if err := store.ExportJSON(context.Background(), opts); err != nil {
			return err
		}
		fmt.Println("Exported to knowledge/index/export.json")
	default:
		return fmt.Errorf("unsupported format %q: use yaml or json", format)
	}

	return nil
}

// --- shared helpers ---

func knowledgeConfig(cmd *cobra.Command) (types.KnowledgeBaseConfig, string) {
	knowledgeDir, _ := cmd.Flags().GetString("knowledge-dir")
	if knowledgeDir == "" {
		knowledgeDir = "knowledge"
	}
	papersDir, _ := cmd.Flags().GetString("papers-dir")
	if papersDir == "" {
		papersDir = "papers"
	}
	maxResults, _ := cmd.Flags().GetInt("max-results")

	cfg := types.KnowledgeBaseConfig{
		KnowledgeDir: knowledgeDir,
		MaxResults:   maxResults,
	}
	return cfg, papersDir
}

func queryOptsFromFlags(cmd *cobra.Command, args []string) knowledge.QueryOptions {
	queryText, _ := cmd.Flags().GetString("query")
	if queryText == "" && len(args) > 0 {
		queryText = strings.Join(args, " ")
	}

	itemType, _ := cmd.Flags().GetString("type")
	tag, _ := cmd.Flags().GetString("tag")
	paperID, _ := cmd.Flags().GetString("paper")
	limit, _ := cmd.Flags().GetInt("limit")

	opts := knowledge.QueryOptions{
		Query:      queryText,
		Type:       types.KnowledgeItemType(itemType),
		PaperID:    paperID,
		MaxResults: limit,
	}
	if tag != "" {
		opts.Tags = []string{tag}
	}
	return opts
}

func init() {
	// Shared flags on the parent command, inherited by subcommands.
	knowledgeCmd.PersistentFlags().String("knowledge-dir", "knowledge", "base directory for knowledge (contains extracted/, index/)")
	knowledgeCmd.PersistentFlags().String("papers-dir", "papers", "base directory for papers (contains metadata/, markdown/)")
	knowledgeCmd.PersistentFlags().Int("max-results", 20, "maximum number of query results")

	// Retrieve flags.
	knowledgeRetrieveCmd.Flags().String("query", "", "full-text search query")
	knowledgeRetrieveCmd.Flags().String("type", "", "filter by item type: claim, method, definition, result")
	knowledgeRetrieveCmd.Flags().String("tag", "", "filter by tag")
	knowledgeRetrieveCmd.Flags().String("paper", "", "filter by paper ID")
	knowledgeRetrieveCmd.Flags().Int("limit", 0, "maximum results (0 = use default)")
	knowledgeRetrieveCmd.Flags().String("trace", "", "show source context for an item ID")
	knowledgeRetrieveCmd.Flags().Bool("json", false, "output results as JSON")

	// Export flags.
	knowledgeExportCmd.Flags().String("format", "yaml", "export format: yaml or json")
	knowledgeExportCmd.Flags().String("query", "", "full-text search filter for partial export")
	knowledgeExportCmd.Flags().String("type", "", "filter by item type for partial export")
	knowledgeExportCmd.Flags().String("tag", "", "filter by tag for partial export")
	knowledgeExportCmd.Flags().String("paper", "", "filter by paper ID for partial export")
	knowledgeExportCmd.Flags().Int("limit", 0, "maximum items to export (0 = all)")

	// Wire subcommands.
	knowledgeCmd.AddCommand(knowledgeStoreCmd)
	knowledgeCmd.AddCommand(knowledgeRetrieveCmd)
	knowledgeCmd.AddCommand(knowledgeExportCmd)

	rootCmd.AddCommand(knowledgeCmd)
}
