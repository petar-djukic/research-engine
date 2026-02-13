// Copyright Mesh Intelligence Inc., 2026. All rights reserved.

package convert

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pdiddy/research-engine/pkg/types"
)

// fakeConverter implements Converter for testing. It returns canned Markdown
// or an error, depending on configuration.
type fakeConverter struct {
	output string
	err    error
}

func (f *fakeConverter) Convert(pdfPath string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.output, nil
}

// setupPDF creates a temporary PDF file and returns its path and the temp dir.
func setupPDF(t *testing.T) (pdfPath, tmpDir string) {
	t.Helper()
	tmpDir = t.TempDir()
	rawDir := filepath.Join(tmpDir, "raw")
	if err := os.MkdirAll(rawDir, 0o755); err != nil {
		t.Fatal(err)
	}
	pdfPath = filepath.Join(rawDir, "2301.07041.pdf")
	if err := os.WriteFile(pdfPath, []byte("fake pdf"), 0o644); err != nil {
		t.Fatal(err)
	}
	return pdfPath, tmpDir
}

func TestConvertPaper(t *testing.T) {
	tests := []struct {
		name       string
		converter  *fakeConverter
		preCreate  bool // create output MD before running
		wantStatus types.ConversionStatus
		wantLog    string
	}{
		{
			name:       "successful conversion",
			converter:  &fakeConverter{output: "# Title\n\nContent here."},
			wantStatus: types.ConversionDone,
			wantLog:    "converted:",
		},
		{
			name:       "skip existing markdown",
			converter:  &fakeConverter{output: "should not be called"},
			preCreate:  true,
			wantStatus: ConversionNone,
			wantLog:    "skipped:",
		},
		{
			name:       "conversion failure",
			converter:  &fakeConverter{err: errors.New("container crashed")},
			wantStatus: types.ConversionFailed,
			wantLog:    "failed:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pdfPath, tmpDir := setupPDF(t)

			if tt.preCreate {
				mdDir := filepath.Join(tmpDir, "markdown")
				if err := os.MkdirAll(mdDir, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(mdDir, "2301.07041.md"), []byte("existing"), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			paper := types.Paper{ID: "2301.07041", PDFPath: pdfPath}
			var log bytes.Buffer

			status := ConvertPaper(tt.converter, paper, tmpDir, &log)

			if status != tt.wantStatus {
				t.Errorf("status = %q, want %q", status, tt.wantStatus)
			}
			if !strings.Contains(log.String(), tt.wantLog) {
				t.Errorf("log output %q does not contain %q", log.String(), tt.wantLog)
			}
		})
	}
}

func TestConvertPaper_Frontmatter(t *testing.T) {
	pdfPath, tmpDir := setupPDF(t)
	conv := &fakeConverter{output: "# Paper Title\n\nSome content."}
	paper := types.Paper{ID: "2301.07041", PDFPath: pdfPath}

	var log bytes.Buffer
	status := ConvertPaper(conv, paper, tmpDir, &log)
	if status != types.ConversionDone {
		t.Fatalf("expected ConversionDone, got %q", status)
	}

	mdPath := filepath.Join(tmpDir, "markdown", "2301.07041.md")
	data, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatalf("reading output: %v", err)
	}
	content := string(data)

	if !strings.HasPrefix(content, "---\n") {
		t.Error("output should start with YAML frontmatter delimiter")
	}
	if !strings.Contains(content, `paper_id: "2301.07041"`) {
		t.Error("frontmatter should contain paper_id")
	}
	if !strings.Contains(content, `source_pdf:`) {
		t.Error("frontmatter should contain source_pdf")
	}
	if !strings.Contains(content, `converted_at:`) {
		t.Error("frontmatter should contain converted_at")
	}
	if !strings.Contains(content, "# Paper Title") {
		t.Error("output should contain the original Markdown body")
	}
}

func TestConvertBatch(t *testing.T) {
	tmpDir := t.TempDir()
	rawDir := filepath.Join(tmpDir, "raw")
	if err := os.MkdirAll(rawDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create 3 PDFs: one will succeed, one will be pre-existing, one will fail.
	for _, name := range []string{"a.pdf", "b.pdf", "c.pdf"} {
		if err := os.WriteFile(filepath.Join(rawDir, name), []byte("pdf"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Pre-create output for "b" to trigger skip.
	mdDir := filepath.Join(tmpDir, "markdown")
	if err := os.MkdirAll(mdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mdDir, "b.md"), []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Converter that fails for "c.pdf".
	conv := &selectiveConverter{
		outputs: map[string]string{
			filepath.Join(rawDir, "a.pdf"): "# Paper A",
			filepath.Join(rawDir, "b.pdf"): "# Paper B",
		},
		errors: map[string]error{
			filepath.Join(rawDir, "c.pdf"): errors.New("bad pdf"),
		},
	}

	papers := []types.Paper{
		{ID: "a", PDFPath: filepath.Join(rawDir, "a.pdf")},
		{ID: "b", PDFPath: filepath.Join(rawDir, "b.pdf")},
		{ID: "c", PDFPath: filepath.Join(rawDir, "c.pdf")},
	}

	var log bytes.Buffer
	result := ConvertBatch(conv, papers, tmpDir, &log)

	if result.Converted != 1 {
		t.Errorf("converted = %d, want 1", result.Converted)
	}
	if result.Skipped != 1 {
		t.Errorf("skipped = %d, want 1", result.Skipped)
	}
	if result.Failed != 1 {
		t.Errorf("failed = %d, want 1", result.Failed)
	}
	if !result.HasFailures() {
		t.Error("HasFailures should be true")
	}
	if result.Total() != 3 {
		t.Errorf("total = %d, want 3", result.Total())
	}

	output := log.String()
	if !strings.Contains(output, "Batch summary:") {
		t.Error("batch output should contain summary line")
	}
}

func TestConvertPaths(t *testing.T) {
	tmpDir := t.TempDir()
	rawDir := filepath.Join(tmpDir, "raw")
	if err := os.MkdirAll(rawDir, 0o755); err != nil {
		t.Fatal(err)
	}

	pdfPath := filepath.Join(rawDir, "test.pdf")
	if err := os.WriteFile(pdfPath, []byte("pdf"), 0o644); err != nil {
		t.Fatal(err)
	}

	conv := &fakeConverter{output: "# Test"}
	var log bytes.Buffer
	result := ConvertPaths(conv, []string{pdfPath}, tmpDir, &log)

	if result.Converted != 1 {
		t.Errorf("converted = %d, want 1", result.Converted)
	}

	mdPath := filepath.Join(tmpDir, "markdown", "test.md")
	if _, err := os.Stat(mdPath); err != nil {
		t.Errorf("expected output file at %s", mdPath)
	}
}

// selectiveConverter returns different results per file path.
type selectiveConverter struct {
	outputs map[string]string
	errors  map[string]error
}

func (s *selectiveConverter) Convert(pdfPath string) (string, error) {
	if err, ok := s.errors[pdfPath]; ok {
		return "", err
	}
	if out, ok := s.outputs[pdfPath]; ok {
		return out, nil
	}
	return "", errors.New("unexpected path: " + pdfPath)
}
