// Copyright Mesh Intelligence Inc., 2026. All rights reserved.

package acquire

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pdiddy/research-engine/pkg/types"
)

// newPatentTestServer creates a server handling patent PDF, Google Patents
// fallback, PatentsView metadata API, and arXiv endpoints for mixed-batch tests.
func newPatentTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		// Patent PDF at the Google Patents storage path.
		case strings.HasPrefix(r.URL.Path, "/patent-pdf/"):
			w.Header().Set("Content-Type", "application/pdf")
			fmt.Fprint(w, fakePDFContent)

		// Google Patents HTML fallback.
		case strings.HasPrefix(r.URL.Path, "/google-patents/"):
			w.Header().Set("Content-Type", "application/pdf")
			fmt.Fprint(w, fakePDFContent)

		// PatentsView metadata API.
		case strings.HasPrefix(r.URL.Path, "/patentsview-api/"):
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, samplePatentsViewJSON)

		// arXiv PDF (for mixed batch tests).
		case strings.HasPrefix(r.URL.Path, "/pdf/"):
			w.Header().Set("Content-Type", "application/pdf")
			fmt.Fprint(w, fakePDFContent)

		// arXiv API (for mixed batch tests).
		case r.URL.Path == "/api/query":
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprint(w, sampleArxivXML)

		// OpenAlex (returns no OA for simplicity).
		case strings.HasPrefix(r.URL.Path, "/openalex/"):
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"best_oa_location": null}`)

		default:
			http.NotFound(w, r)
		}
	}))
}

func TestAcquirePatentSuccess(t *testing.T) {
	ts := newPatentTestServer(t)
	defer ts.Close()
	restore := overrideBaseURLs(ts.URL)
	defer restore()

	dir := t.TempDir()
	cfg := testConfig(dir)
	var buf bytes.Buffer

	paper, skipped, err := AcquirePaper(ts.Client(), "US7654321B2", cfg, &buf)
	if err != nil {
		t.Fatalf("AcquirePaper: %v", err)
	}
	if skipped {
		t.Error("expected download, got skipped")
	}
	if paper.ID != "US7654321B2" {
		t.Errorf("paper.ID = %q, want %q", paper.ID, "US7654321B2")
	}
	if paper.Source != "patentsview" {
		t.Errorf("paper.Source = %q, want %q", paper.Source, "patentsview")
	}
	if paper.Title != "Method for testing patents" {
		t.Errorf("paper.Title = %q, want %q", paper.Title, "Method for testing patents")
	}
	if len(paper.Authors) != 2 {
		t.Fatalf("len(Authors) = %d, want 2", len(paper.Authors))
	}
	if paper.Authors[0] != "Edison" {
		t.Errorf("Authors[0] = %q, want %q", paper.Authors[0], "Edison")
	}

	expectedDate := time.Date(2023, 3, 14, 0, 0, 0, 0, time.UTC)
	if !paper.Date.Equal(expectedDate) {
		t.Errorf("Date = %v, want %v", paper.Date, expectedDate)
	}

	// Verify PDF file exists.
	pdfPath := filepath.Join(dir, "raw", "US7654321B2.pdf")
	data, err := os.ReadFile(pdfPath)
	if err != nil {
		t.Fatalf("reading PDF: %v", err)
	}
	if string(data) != fakePDFContent {
		t.Errorf("PDF content = %q, want %q", string(data), fakePDFContent)
	}

	// Verify metadata YAML exists.
	metaPath := filepath.Join(dir, "metadata", "US7654321B2.yaml")
	if _, err := os.Stat(metaPath); err != nil {
		t.Fatalf("metadata file missing: %v", err)
	}

	// Verify output mentions downloading.
	if !strings.Contains(buf.String(), "downloading:") {
		t.Error("output should contain 'downloading:'")
	}
}

func TestAcquirePatentSkipExisting(t *testing.T) {
	ts := newPatentTestServer(t)
	defer ts.Close()
	restore := overrideBaseURLs(ts.URL)
	defer restore()

	dir := t.TempDir()
	cfg := testConfig(dir)

	// Pre-create the patent PDF file.
	rawPath := filepath.Join(dir, "raw")
	if err := os.MkdirAll(rawPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rawPath, "US7654321B2.pdf"), []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	paper, skipped, err := AcquirePaper(ts.Client(), "US7654321B2", cfg, &buf)
	if err != nil {
		t.Fatalf("AcquirePaper: %v", err)
	}
	if !skipped {
		t.Error("expected skipped, got download")
	}
	if paper.ID != "US7654321B2" {
		t.Errorf("paper.ID = %q, want %q", paper.ID, "US7654321B2")
	}
	if !strings.Contains(buf.String(), "skipped:") {
		t.Error("output should contain 'skipped:'")
	}
}

func TestAcquirePatentFallback(t *testing.T) {
	// Server that returns 404 for patent-pdf/ (primary) but serves fallback.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/patent-pdf/"):
			// Primary Google Patents storage returns 404.
			http.NotFound(w, r)
		case strings.HasPrefix(r.URL.Path, "/google-patents/"):
			// Fallback Google Patents HTML serves PDF.
			w.Header().Set("Content-Type", "application/pdf")
			fmt.Fprint(w, fakePDFContent)
		case strings.HasPrefix(r.URL.Path, "/patentsview-api/"):
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, samplePatentsViewJSON)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()
	restore := overrideBaseURLs(ts.URL)
	defer restore()

	dir := t.TempDir()
	cfg := testConfig(dir)
	var buf bytes.Buffer

	paper, skipped, err := AcquirePaper(ts.Client(), "US7654321B2", cfg, &buf)
	if err != nil {
		t.Fatalf("AcquirePaper: %v", err)
	}
	if skipped {
		t.Error("expected download, got skipped")
	}
	if paper.Title != "Method for testing patents" {
		t.Errorf("paper.Title = %q, want %q", paper.Title, "Method for testing patents")
	}

	// Verify PDF was saved via fallback.
	pdfPath := filepath.Join(dir, "raw", "US7654321B2.pdf")
	if _, err := os.Stat(pdfPath); err != nil {
		t.Fatalf("PDF file missing after fallback: %v", err)
	}

	// Verify the fallback warning was logged.
	if !strings.Contains(buf.String(), "warning:") || !strings.Contains(buf.String(), "fallback") {
		t.Error("output should contain fallback warning")
	}
}

func TestFetchPatentMetadata(t *testing.T) {
	ts := newPatentTestServer(t)
	defer ts.Close()
	restore := overrideBaseURLs(ts.URL)
	defer restore()

	cfg := testConfig(t.TempDir())
	paper := &types.Paper{}

	err := fetchPatentMetadata(ts.Client(), "US7654321B2", paper, cfg)
	if err != nil {
		t.Fatalf("fetchPatentMetadata: %v", err)
	}

	if paper.Title != "Method for testing patents" {
		t.Errorf("Title = %q, want %q", paper.Title, "Method for testing patents")
	}
	if paper.Abstract != "A method for testing patent acquisition." {
		t.Errorf("Abstract = %q", paper.Abstract)
	}
	if len(paper.Authors) != 2 {
		t.Fatalf("len(Authors) = %d, want 2", len(paper.Authors))
	}
	if paper.Authors[0] != "Edison" {
		t.Errorf("Authors[0] = %q, want %q", paper.Authors[0], "Edison")
	}
	if paper.Authors[1] != "Tesla" {
		t.Errorf("Authors[1] = %q, want %q", paper.Authors[1], "Tesla")
	}

	expectedDate := time.Date(2023, 3, 14, 0, 0, 0, 0, time.UTC)
	if !paper.Date.Equal(expectedDate) {
		t.Errorf("Date = %v, want %v", paper.Date, expectedDate)
	}
}

func TestFetchPatentMetadataAPIFailure(t *testing.T) {
	// Server that returns 500 for PatentsView API.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/patentsview-api/"):
			w.WriteHeader(http.StatusInternalServerError)
		case strings.HasPrefix(r.URL.Path, "/patent-pdf/"):
			w.Header().Set("Content-Type", "application/pdf")
			fmt.Fprint(w, fakePDFContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()
	restore := overrideBaseURLs(ts.URL)
	defer restore()

	dir := t.TempDir()
	cfg := testConfig(dir)
	var buf bytes.Buffer

	// The acquisition should succeed with a warning, not fail entirely.
	paper, skipped, err := AcquirePaper(ts.Client(), "US7654321B2", cfg, &buf)
	if err != nil {
		t.Fatalf("AcquirePaper should not fail when metadata fetch fails: %v", err)
	}
	if skipped {
		t.Error("expected download, got skipped")
	}

	// Metadata fields should be empty since the API failed.
	if paper.Title != "" {
		t.Errorf("Title should be empty when metadata fails, got %q", paper.Title)
	}
	if len(paper.Authors) != 0 {
		t.Errorf("Authors should be empty, got %v", paper.Authors)
	}

	// Warning should be logged.
	if !strings.Contains(buf.String(), "warning:") {
		t.Error("output should contain metadata failure warning")
	}
}

func TestStripKindCode(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"7654321B2", "7654321"},
		{"7654321B1", "7654321"},
		{"20230012345A1", "20230012345"},
		{"7654321", "7654321"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := stripKindCode(tt.input)
			if got != tt.want {
				t.Errorf("stripKindCode(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestAcquireBatchMixedPapersAndPatents(t *testing.T) {
	ts := newPatentTestServer(t)
	defer ts.Close()
	restore := overrideBaseURLs(ts.URL)
	defer restore()

	dir := t.TempDir()
	cfg := testConfig(dir)
	var buf bytes.Buffer

	identifiers := []string{
		"2301.07041",    // arXiv paper
		"US7654321B2",   // Patent
	}

	result := AcquireBatch(ts.Client(), identifiers, cfg, &buf)

	if result.Downloaded != 2 {
		t.Errorf("Downloaded = %d, want 2", result.Downloaded)
	}
	if result.Failed != 0 {
		t.Errorf("Failed = %d, want 0", result.Failed)
	}
	if result.Total() != 2 {
		t.Errorf("Total = %d, want 2", result.Total())
	}
	if len(result.Papers) != 2 {
		t.Fatalf("len(Papers) = %d, want 2", len(result.Papers))
	}

	// Verify arXiv paper.
	arxivPaper := result.Papers[0]
	if arxivPaper.ID != "2301.07041" {
		t.Errorf("Papers[0].ID = %q, want %q", arxivPaper.ID, "2301.07041")
	}
	if arxivPaper.Source != "arxiv" {
		t.Errorf("Papers[0].Source = %q, want %q", arxivPaper.Source, "arxiv")
	}

	// Verify patent.
	patentPaper := result.Papers[1]
	if patentPaper.ID != "US7654321B2" {
		t.Errorf("Papers[1].ID = %q, want %q", patentPaper.ID, "US7654321B2")
	}
	if patentPaper.Source != "patentsview" {
		t.Errorf("Papers[1].Source = %q, want %q", patentPaper.Source, "patentsview")
	}

	// Verify both PDF files exist.
	for _, slug := range []string{"2301.07041", "US7654321B2"} {
		pdfPath := filepath.Join(dir, "raw", slug+".pdf")
		if _, err := os.Stat(pdfPath); err != nil {
			t.Errorf("PDF missing for %s: %v", slug, err)
		}
	}

	// Verify batch summary.
	if !strings.Contains(buf.String(), "Batch summary:") {
		t.Error("output should contain batch summary")
	}
}
