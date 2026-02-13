// Copyright Mesh Intelligence Inc., 2026. All rights reserved.

// Package convert implements PDF-to-Markdown conversion with pluggable backends.
// Implements: prd002-conversion (R1, R2, R3);
//
//	docs/ARCHITECTURE § Conversion.
package convert

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pdiddy/research-engine/pkg/types"
)

const (
	// markdownDir is the subdirectory under the papers base for Markdown output.
	markdownDir = "markdown"
	// rawDir is the subdirectory under the papers base for raw PDFs.
	rawDir = "raw"
)

// Converter transforms a PDF file into Markdown text. Different backends
// (markitdown, GROBID, pdftotext) implement this interface.
type Converter interface {
	// Convert reads a PDF at pdfPath and returns the Markdown content.
	Convert(pdfPath string) (string, error)
}

// BatchResult holds the outcome of a batch conversion run.
type BatchResult struct {
	Converted int
	Skipped   int
	Failed    int
}

// Total returns the total number of papers processed.
func (r BatchResult) Total() int {
	return r.Converted + r.Skipped + r.Failed
}

// HasFailures reports whether any papers failed conversion.
func (r BatchResult) HasFailures() bool {
	return r.Failed > 0
}

// ConvertPaper converts a single PDF to Markdown, writing the result to the
// output directory. It returns the status of the conversion. If the Markdown
// output already exists, it skips conversion and returns ConversionNone.
func ConvertPaper(c Converter, paper types.Paper, papersDir string, w io.Writer) types.ConversionStatus {
	outDir := filepath.Join(papersDir, markdownDir)
	base := strings.TrimSuffix(filepath.Base(paper.PDFPath), filepath.Ext(paper.PDFPath))
	mdPath := filepath.Join(outDir, base+".md")

	if _, err := os.Stat(mdPath); err == nil {
		fmt.Fprintf(w, "skipped: %s (already exists)\n", base)
		return ConversionNone
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintf(w, "failed:  %s (%v)\n", base, err)
		return types.ConversionFailed
	}

	raw, err := c.Convert(paper.PDFPath)
	if err != nil {
		fmt.Fprintf(w, "failed:  %s (%v)\n", base, err)
		return types.ConversionFailed
	}

	content := addFrontmatter(paper, raw)

	if err := os.WriteFile(mdPath, []byte(content), 0o644); err != nil {
		fmt.Fprintf(w, "failed:  %s (%v)\n", base, err)
		return types.ConversionFailed
	}

	fmt.Fprintf(w, "converted: %s\n", base)
	return types.ConversionDone
}

// ConvertBatch processes a list of papers through the converter, printing
// per-file status to w and returning a summary.
func ConvertBatch(c Converter, papers []types.Paper, papersDir string, w io.Writer) BatchResult {
	var result BatchResult
	for _, p := range papers {
		status := ConvertPaper(c, p, papersDir, w)
		switch status {
		case types.ConversionDone:
			result.Converted++
		case ConversionNone:
			result.Skipped++
		case types.ConversionFailed:
			result.Failed++
		}
	}
	fmt.Fprintf(w, "\nBatch summary: %d converted, %d skipped, %d failed (total: %d)\n",
		result.Converted, result.Skipped, result.Failed, result.Total())
	return result
}

// ConvertPaths builds Paper records from raw PDF paths and delegates to
// ConvertBatch. Each path is turned into a minimal Paper with ID derived
// from the filename.
func ConvertPaths(c Converter, pdfPaths []string, papersDir string, w io.Writer) BatchResult {
	papers := make([]types.Paper, len(pdfPaths))
	for i, p := range pdfPaths {
		base := strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
		papers[i] = types.Paper{
			ID:      base,
			PDFPath: p,
		}
	}
	return ConvertBatch(c, papers, papersDir, w)
}

// ConversionNone is a local alias for "skip" status (markdown already exists).
const ConversionNone = types.ConversionNone

// addFrontmatter prepends YAML frontmatter to the converted Markdown content.
func addFrontmatter(paper types.Paper, body string) string {
	ts := time.Now().UTC().Format(time.RFC3339)
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "paper_id: %q\n", paper.ID)
	fmt.Fprintf(&b, "source_pdf: %q\n", paper.PDFPath)
	fmt.Fprintf(&b, "converted_at: %q\n", ts)
	b.WriteString("---\n\n")
	b.WriteString(body)
	return b.String()
}
