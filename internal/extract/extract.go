// Package extract identifies typed knowledge items within converted text.
// Implements: prd003-extraction (R1, R2, R5, R6);
//
//	docs/ARCHITECTURE § Extraction.
package extract

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/pdiddy/research-engine/pkg/types"
)

const (
	markdownDir  = "markdown"
	extractedDir = "extracted"
)

// validItemTypes is the set of accepted KnowledgeItemType values (R1.1).
var validItemTypes = map[types.KnowledgeItemType]bool{
	types.ItemClaim:      true,
	types.ItemMethod:     true,
	types.ItemDefinition: true,
	types.ItemResult:     true,
}

// AIBackend abstracts the Generative AI API so tests can supply a mock.
// Each implementation handles a single section of Markdown and returns
// the raw response. Per Strategy pattern (prd003-extraction R5.2).
type AIBackend interface {
	Extract(ctx context.Context, section string) (AIResponse, error)
}

// AIResponse is the structured response from the AI backend for one section.
type AIResponse struct {
	Items []AIResponseItem `json:"items" yaml:"items"`
}

// AIResponseItem is a single item as returned by the AI backend.
type AIResponseItem struct {
	Type       string   `json:"type" yaml:"type"`
	Content    string   `json:"content" yaml:"content"`
	Section    string   `json:"section" yaml:"section"`
	Page       int      `json:"page" yaml:"page"`
	Confidence float64  `json:"confidence" yaml:"confidence"`
	Tags       []string `json:"tags" yaml:"tags"`
}

// BatchSummary holds counts from a batch extraction run (R6.4).
type BatchSummary struct {
	Extracted int
	Skipped   int
	Failed    int
}

// Total returns the number of papers processed.
func (s BatchSummary) Total() int {
	return s.Extracted + s.Skipped + s.Failed
}

// HasFailures reports whether any papers failed (R6.5).
func (s BatchSummary) HasFailures() bool {
	return s.Failed > 0
}

// ExtractAll processes all Markdown files in papersDir/markdown/, extracts
// knowledge items via the AI backend, and writes results to knowledgeDir/extracted/.
// It skips unchanged files and re-extracts changed ones (R6.1, R6.2).
func ExtractAll(ctx context.Context, backend AIBackend, cfg types.ExtractionConfig, w io.Writer) (BatchSummary, error) {
	mdDir := filepath.Join(cfg.PapersDir, markdownDir)
	outDir := filepath.Join(cfg.KnowledgeDir, extractedDir)

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return BatchSummary{}, fmt.Errorf("creating output directory: %w", err)
	}

	entries, err := os.ReadDir(mdDir)
	if err != nil {
		return BatchSummary{}, fmt.Errorf("reading markdown directory %s: %w", mdDir, err)
	}

	var summary BatchSummary

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		paperID := strings.TrimSuffix(entry.Name(), ".md")
		mdPath := filepath.Join(mdDir, entry.Name())
		outPath := filepath.Join(outDir, paperID+"-items.yaml")

		changed, err := hasChanged(mdPath, outPath)
		if err != nil {
			fmt.Fprintf(w, "failed  %s: %v\n", paperID, err)
			summary.Failed++
			continue
		}
		if !changed {
			fmt.Fprintf(w, "skipped %s\n", paperID)
			summary.Skipped++
			continue
		}

		fmt.Fprintf(w, "extracting %s\n", paperID)

		result, err := ExtractPaper(ctx, backend, paperID, mdPath, cfg)
		if err != nil {
			fmt.Fprintf(w, "failed  %s: %v\n", paperID, err)
			summary.Failed++
			continue
		}

		if err := writeResult(outPath, result); err != nil {
			fmt.Fprintf(w, "failed  %s: write error: %v\n", paperID, err)
			summary.Failed++
			continue
		}

		fmt.Fprintf(w, "extracted %s (%d items)\n", paperID, len(result.Items))
		summary.Extracted++
	}

	return summary, nil
}

// ExtractPaper extracts knowledge items from a single paper's Markdown.
// It chunks the Markdown by section headings and calls the AI backend
// for each chunk (R5.1, R5.3).
func ExtractPaper(ctx context.Context, backend AIBackend, paperID, mdPath string, cfg types.ExtractionConfig) (*types.ExtractionResult, error) {
	content, err := os.ReadFile(mdPath)
	if err != nil {
		return nil, fmt.Errorf("reading markdown %s: %w", mdPath, err)
	}

	sections := chunkByHeadings(string(content))

	result := &types.ExtractionResult{
		PaperID: paperID,
	}

	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}

	for _, sec := range sections {
		if strings.TrimSpace(sec.body) == "" {
			continue
		}

		chunk := formatChunk(sec)

		resp, err := callWithRetry(ctx, backend, chunk, maxRetries)
		if err != nil {
			return nil, fmt.Errorf("extracting section %q: %w", sec.heading, err)
		}

		items, validationErrors := convertItems(resp.Items, paperID, sec.heading)
		if len(validationErrors) > 0 {
			return nil, fmt.Errorf("validation errors in section %q: %s", sec.heading, strings.Join(validationErrors, "; "))
		}

		result.Items = append(result.Items, items...)
	}

	return result, nil
}

// section represents a chunk of Markdown under one heading.
type section struct {
	heading string
	body    string
	page    int
}

// chunkByHeadings splits Markdown into sections based on heading boundaries
// (## or ###). Each section carries the heading text and the body up to the
// next heading. Page numbers are extracted from HTML comments like
// <!-- page 3 --> (R5.3).
func chunkByHeadings(content string) []section {
	lines := strings.Split(content, "\n")
	var sections []section
	currentHeading := ""
	currentPage := 1
	var bodyLines []string

	flush := func() {
		body := strings.Join(bodyLines, "\n")
		if currentHeading != "" || strings.TrimSpace(body) != "" {
			sections = append(sections, section{
				heading: currentHeading,
				body:    body,
				page:    currentPage,
			})
		}
		bodyLines = nil
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Detect page markers: <!-- page N -->
		if page, ok := parsePageMarker(trimmed); ok {
			currentPage = page
			continue
		}

		// Detect headings (## or ###)
		if isHeading(trimmed) {
			flush()
			currentHeading = stripHeadingPrefix(trimmed)
			continue
		}

		bodyLines = append(bodyLines, line)
	}

	flush()
	return sections
}

// isHeading returns true if the line starts with ## or ###.
func isHeading(line string) bool {
	return strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "### ")
}

// stripHeadingPrefix removes the leading # characters and whitespace.
func stripHeadingPrefix(line string) string {
	return strings.TrimSpace(strings.TrimLeft(line, "#"))
}

// parsePageMarker extracts the page number from an HTML comment like <!-- page 3 -->.
func parsePageMarker(line string) (int, bool) {
	if !strings.HasPrefix(line, "<!-- page ") || !strings.HasSuffix(line, " -->") {
		return 0, false
	}
	inner := strings.TrimPrefix(line, "<!-- page ")
	inner = strings.TrimSuffix(inner, " -->")
	var page int
	if _, err := fmt.Sscanf(inner, "%d", &page); err != nil {
		return 0, false
	}
	return page, true
}

// formatChunk prepares a section for the AI backend by combining heading and body.
func formatChunk(sec section) string {
	if sec.heading == "" {
		return sec.body
	}
	return fmt.Sprintf("## %s\n\n%s", sec.heading, sec.body)
}

// backoffBase controls the base duration for exponential backoff. Tests
// override this to avoid real sleeps.
var backoffBase = time.Second

// callWithRetry calls the AI backend with exponential backoff (R5.5).
func callWithRetry(ctx context.Context, backend AIBackend, chunk string, maxRetries int) (AIResponse, error) {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(math.Pow(2, float64(attempt-1))) * backoffBase
			select {
			case <-ctx.Done():
				return AIResponse{}, ctx.Err()
			case <-time.After(backoff):
			}
		}

		resp, err := backend.Extract(ctx, chunk)
		if err == nil {
			return resp, nil
		}
		lastErr = err
	}
	return AIResponse{}, fmt.Errorf("after %d retries: %w", maxRetries, lastErr)
}

// convertItems validates AI response items and converts them to KnowledgeItems (R5.4).
func convertItems(items []AIResponseItem, paperID, sectionHeading string) ([]types.KnowledgeItem, []string) {
	var result []types.KnowledgeItem
	var errors []string

	for i, item := range items {
		itemType := types.KnowledgeItemType(item.Type)
		if !validItemTypes[itemType] {
			errors = append(errors, fmt.Sprintf("item %d: invalid type %q", i, item.Type))
			continue
		}
		if item.Content == "" {
			errors = append(errors, fmt.Sprintf("item %d: empty content", i))
			continue
		}
		if item.Confidence < 0.0 || item.Confidence > 1.0 {
			errors = append(errors, fmt.Sprintf("item %d: confidence %f out of range [0,1]", i, item.Confidence))
			continue
		}

		sec := sectionHeading
		if item.Section != "" {
			sec = item.Section
		}

		ki := types.KnowledgeItem{
			ID:         stableID(paperID, sec, item.Content),
			Type:       itemType,
			Content:    item.Content,
			PaperID:    paperID,
			Section:    sec,
			Page:       item.Page,
			Confidence: item.Confidence,
			Tags:       item.Tags,
		}
		result = append(result, ki)
	}

	return result, errors
}

// stableID generates a deterministic ID from paper ID, section, and content (R2.5).
// The ID is the first 12 hex characters of SHA-256(paperID + section + content).
func stableID(paperID, section, content string) string {
	h := sha256.New()
	h.Write([]byte(paperID))
	h.Write([]byte(section))
	h.Write([]byte(content))
	return fmt.Sprintf("%x", h.Sum(nil))[:12]
}

// hasChanged reports whether the Markdown file is newer than the output file (R6.1).
// Returns true if the output does not exist or the Markdown is more recent.
func hasChanged(mdPath, outPath string) (bool, error) {
	mdInfo, err := os.Stat(mdPath)
	if err != nil {
		return false, fmt.Errorf("stat markdown %s: %w", mdPath, err)
	}

	outInfo, err := os.Stat(outPath)
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}
		return false, fmt.Errorf("stat output %s: %w", outPath, err)
	}

	return mdInfo.ModTime().After(outInfo.ModTime()), nil
}

// writeResult marshals the ExtractionResult to a YAML file (R5.6).
func writeResult(path string, result *types.ExtractionResult) error {
	data, err := yaml.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshaling result: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}
