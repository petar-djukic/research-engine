// Copyright Mesh Intelligence Inc., 2026. All rights reserved.

// Extract CLI command wires the extraction stage to the command line.
// Implements: prd003-extraction R6 (CLI surface, batch processing).
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.yaml.in/yaml/v3"

	"github.com/pdiddy/research-engine/internal/extract"
	"github.com/pdiddy/research-engine/pkg/types"
)

var extractCmd = &cobra.Command{
	Use:   "extract [papers...]",
	Short: "Extract typed knowledge items from converted papers",
	Long: `Extract reads structured Markdown and produces typed knowledge items
(claims, methods, definitions, results) with provenance links back to
the source paper, section, and page.

Provide paper IDs as positional arguments to extract specific papers,
or use --batch to process all papers in papers/markdown/.`,
	RunE: runExtract,
}

func init() {
	extractCmd.Flags().String("model", "", "AI model identifier for extraction")
	extractCmd.Flags().String("api-key", "", "API key for the AI backend (or set RESEARCH_ENGINE_EXTRACTION_API_KEY)")
	extractCmd.Flags().String("papers-dir", "papers", "base directory for papers (contains markdown/)")
	extractCmd.Flags().String("knowledge-dir", "knowledge", "base directory for knowledge output (contains extracted/)")
	extractCmd.Flags().Bool("batch", false, "process all unconverted papers in papers-dir")

	rootCmd.AddCommand(extractCmd)
}

func runExtract(cmd *cobra.Command, args []string) error {
	cfg := extractionConfig(cmd)

	if cfg.APIKey == "" {
		return fmt.Errorf("API key required: use --api-key or set RESEARCH_ENGINE_EXTRACTION_API_KEY")
	}
	if cfg.Model == "" {
		return fmt.Errorf("model required: use --model or set extraction.model in config")
	}

	batch, _ := cmd.Flags().GetBool("batch")
	if !batch && len(args) == 0 {
		return fmt.Errorf("provide paper IDs as arguments or use --batch")
	}

	backend := &extract.ClaudeBackend{
		APIKey: cfg.APIKey,
		Model:  cfg.Model,
		Client: &http.Client{},
	}

	ctx := context.Background()

	var summary extract.BatchSummary
	if batch {
		var err error
		summary, err = extract.ExtractAll(ctx, backend, cfg, os.Stdout)
		if err != nil {
			return err
		}
	} else {
		summary = extractPapers(ctx, backend, args, cfg)
	}

	fmt.Fprintf(os.Stdout, "\n%d extracted, %d skipped, %d failed (%d total)\n",
		summary.Extracted, summary.Skipped, summary.Failed, summary.Total())

	if summary.HasFailures() {
		return fmt.Errorf("%d paper(s) failed extraction", summary.Failed)
	}
	return nil
}

// extractPapers processes specific paper IDs rather than scanning the full
// markdown directory. It follows the same status output format as ExtractAll.
func extractPapers(ctx context.Context, backend extract.AIBackend, paperIDs []string, cfg types.ExtractionConfig) extract.BatchSummary {
	outDir := filepath.Join(cfg.KnowledgeDir, "extracted")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stdout, "failed  creating output directory: %v\n", err)
		return extract.BatchSummary{Failed: len(paperIDs)}
	}

	var summary extract.BatchSummary
	for _, paperID := range paperIDs {
		mdPath := filepath.Join(cfg.PapersDir, "markdown", paperID+".md")
		outPath := filepath.Join(outDir, paperID+"-items.yaml")

		if _, err := os.Stat(mdPath); err != nil {
			fmt.Fprintf(os.Stdout, "failed  %s: %v\n", paperID, err)
			summary.Failed++
			continue
		}

		fmt.Fprintf(os.Stdout, "extracting %s\n", paperID)

		result, err := extract.ExtractPaper(ctx, backend, paperID, mdPath, cfg)
		if err != nil {
			fmt.Fprintf(os.Stdout, "failed  %s: %v\n", paperID, err)
			summary.Failed++
			continue
		}

		data, err := yaml.Marshal(result)
		if err != nil {
			fmt.Fprintf(os.Stdout, "failed  %s: marshaling result: %v\n", paperID, err)
			summary.Failed++
			continue
		}
		if err := os.WriteFile(outPath, data, 0o644); err != nil {
			fmt.Fprintf(os.Stdout, "failed  %s: writing result: %v\n", paperID, err)
			summary.Failed++
			continue
		}

		fmt.Fprintf(os.Stdout, "extracted %s (%d items)\n", paperID, len(result.Items))
		summary.Extracted++
	}

	return summary
}

// extractionConfig builds ExtractionConfig from CLI flags and Viper config.
// CLI flags take precedence over config file and environment variables.
func extractionConfig(cmd *cobra.Command) types.ExtractionConfig {
	model, _ := cmd.Flags().GetString("model")
	apiKey, _ := cmd.Flags().GetString("api-key")
	papersDir, _ := cmd.Flags().GetString("papers-dir")
	knowledgeDir, _ := cmd.Flags().GetString("knowledge-dir")

	if model == "" {
		model = viper.GetString("extraction.model")
	}
	if apiKey == "" {
		apiKey = viper.GetString("extraction.api_key")
	}
	apiKey = secretDefault("anthropic-api-key", apiKey)
	if papersDir == "papers" {
		if v := viper.GetString("extraction.papers_dir"); v != "" {
			papersDir = v
		}
	}
	if knowledgeDir == "knowledge" {
		if v := viper.GetString("extraction.knowledge_dir"); v != "" {
			knowledgeDir = v
		}
	}

	maxRetries := viper.GetInt("extraction.max_retries")
	if maxRetries <= 0 {
		maxRetries = 3
	}

	return types.ExtractionConfig{
		AIConfig: types.AIConfig{
			Model:      model,
			APIKey:     apiKey,
			MaxRetries: maxRetries,
		},
		PapersDir:    papersDir,
		KnowledgeDir: knowledgeDir,
	}
}
