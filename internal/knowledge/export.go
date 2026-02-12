// Copyright Mesh Intelligence Inc., 2026. All rights reserved.

package knowledge

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"
)

// ExportEntry holds a knowledge item with paper metadata for export (R6.3).
type ExportEntry struct {
	ID         string       `json:"id" yaml:"id"`
	Type       string       `json:"type" yaml:"type"`
	Content    string       `json:"content" yaml:"content"`
	PaperID    string       `json:"paper_id" yaml:"paper_id"`
	Section    string       `json:"section" yaml:"section"`
	Page       int          `json:"page" yaml:"page"`
	Confidence float64      `json:"confidence" yaml:"confidence"`
	Tags       []string     `json:"tags" yaml:"tags"`
	Paper      *ExportPaper `json:"paper,omitempty" yaml:"paper,omitempty"`
}

// ExportPaper holds the paper-level fields included in each export entry.
type ExportPaper struct {
	Title   string   `json:"title" yaml:"title"`
	Authors []string `json:"authors" yaml:"authors"`
}

const exportLimit = 100000

// ExportYAML writes the knowledge base to knowledge/index/export.yaml (R6.1).
// It supports the same filters as Retrieve (R6.4).
func (s *Store) ExportYAML(ctx context.Context, opts QueryOptions) error {
	entries, err := s.exportEntries(ctx, opts)
	if err != nil {
		return err
	}

	path := filepath.Join(s.knowledgeDir, indexDir, "export.yaml")
	data, err := yaml.Marshal(entries)
	if err != nil {
		return fmt.Errorf("marshaling YAML: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

// ExportJSON writes the knowledge base to knowledge/index/export.json (R6.2).
// It supports the same filters as Retrieve (R6.4).
func (s *Store) ExportJSON(ctx context.Context, opts QueryOptions) error {
	entries, err := s.exportEntries(ctx, opts)
	if err != nil {
		return err
	}

	path := filepath.Join(s.knowledgeDir, indexDir, "export.json")
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling JSON: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

func (s *Store) exportEntries(ctx context.Context, opts QueryOptions) ([]ExportEntry, error) {
	opts.MaxResults = exportLimit
	results, err := s.Retrieve(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("querying for export: %w", err)
	}

	entries := make([]ExportEntry, len(results))
	for i, r := range results {
		entries[i] = ExportEntry{
			ID:         r.ID,
			Type:       string(r.Type),
			Content:    r.Content,
			PaperID:    r.PaperID,
			Section:    r.Section,
			Page:       r.Page,
			Confidence: r.Confidence,
			Tags:       r.Tags,
		}
		if r.PaperTitle != "" || len(r.PaperAuthors) > 0 {
			entries[i].Paper = &ExportPaper{
				Title:   r.PaperTitle,
				Authors: r.PaperAuthors,
			}
		}
	}

	return entries, nil
}
