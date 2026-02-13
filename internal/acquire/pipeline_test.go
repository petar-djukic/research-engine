// Copyright Mesh Intelligence Inc., 2026. All rights reserved.

// Integration test: patent search → acquire pipeline (research-0op.11, prd008).
// Exercises the end-to-end flow using mock servers for PatentsView (search +
// metadata), Google Patents PDF storage, and arXiv endpoints.

package acquire

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/pdiddy/research-engine/pkg/types"
)

// patentsViewSearchResponse mirrors the PatentsView search API response
// structure for test JSON parsing.
type patentsViewSearchResponse struct {
	Patents []struct {
		PatentID       string `json:"patent_id"`
		PatentTitle    string `json:"patent_title"`
		PatentAbstract string `json:"patent_abstract"`
		PatentDate     string `json:"patent_date"`
		Inventors      []struct {
			InventorNameLast string `json:"inventor_name_last"`
		} `json:"inventors"`
	} `json:"patents"`
	Count int `json:"count"`
	Total int `json:"total_patent_count"`
}

const pipelinePatentsViewSearchJSON = `{
  "patents": [
    {
      "patent_id": "7654321",
      "patent_title": "Method for testing patents",
      "patent_abstract": "A method for testing patent acquisition.",
      "patent_date": "2023-03-14",
      "patent_type": "utility",
      "patent_num_claims": 10,
      "inventors": [
        {"inventor_name_last": "Edison"},
        {"inventor_name_last": "Tesla"}
      ]
    },
    {
      "patent_id": "9876543",
      "patent_title": "System for processing data",
      "patent_abstract": "A system for efficiently processing data streams.",
      "patent_date": "2024-01-15",
      "patent_type": "utility",
      "patent_num_claims": 15,
      "inventors": [
        {"inventor_name_last": "Curie"}
      ]
    }
  ],
  "count": 2,
  "total_patent_count": 2
}`

// newPipelineTestServer creates a server for integration tests covering both
// search and acquire endpoints.
func newPipelineTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		// PatentsView search API.
		case r.URL.Path == "/patentsview-search/":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, pipelinePatentsViewSearchJSON)

		// PatentsView metadata API (for acquire).
		case strings.HasPrefix(r.URL.Path, "/patentsview-api/"):
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, samplePatentsViewJSON)

		// Google Patents PDF storage (primary download path).
		case strings.HasPrefix(r.URL.Path, "/patent-pdf/"):
			w.Header().Set("Content-Type", "application/pdf")
			fmt.Fprint(w, fakePDFContent)

		// Google Patents HTML fallback.
		case strings.HasPrefix(r.URL.Path, "/google-patents/"):
			w.Header().Set("Content-Type", "application/pdf")
			fmt.Fprint(w, fakePDFContent)

		// arXiv PDF endpoint (for mixed batch).
		case strings.HasPrefix(r.URL.Path, "/pdf/"):
			w.Header().Set("Content-Type", "application/pdf")
			fmt.Fprint(w, fakePDFContent)

		// arXiv API (for mixed batch metadata).
		case r.URL.Path == "/api/query":
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprint(w, sampleArxivXML)

		// OpenAlex (returns no OA).
		case strings.HasPrefix(r.URL.Path, "/openalex/"):
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"best_oa_location": null}`)

		default:
			http.NotFound(w, r)
		}
	}))
}

func TestPipelineSearchThenAcquire(t *testing.T) {
	ts := newPipelineTestServer(t)
	defer ts.Close()
	restore := overrideBaseURLs(ts.URL)
	defer restore()

	// Step 1: Simulate PatentsView search by hitting the mock endpoint.
	searchURL := ts.URL + "/patentsview-search/"
	resp, err := ts.Client().Get(searchURL)
	if err != nil {
		t.Fatalf("search request: %v", err)
	}
	defer resp.Body.Close()

	var searchResp patentsViewSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		t.Fatalf("parsing search response: %v", err)
	}

	if len(searchResp.Patents) != 2 {
		t.Fatalf("search returned %d patents, want 2", len(searchResp.Patents))
	}

	// Step 2: Extract identifiers with US prefix (as PatentsViewBackend does).
	var identifiers []string
	for _, p := range searchResp.Patents {
		identifiers = append(identifiers, "US"+p.PatentID)
	}

	// Step 3: Acquire the patents.
	dir := t.TempDir()
	cfg := testConfig(dir)
	var buf bytes.Buffer

	result := AcquireBatch(ts.Client(), identifiers, cfg, &buf)

	if result.Downloaded != 2 {
		t.Errorf("Downloaded = %d, want 2", result.Downloaded)
	}
	if result.Failed != 0 {
		t.Errorf("Failed = %d, want 0", result.Failed)
	}

	// Step 4: Verify file naming.
	for _, id := range []string{"US7654321", "US9876543"} {
		pdfPath := filepath.Join(dir, "raw", id+".pdf")
		if _, err := os.Stat(pdfPath); err != nil {
			t.Errorf("PDF missing for %s: %v", id, err)
		}

		metaPath := filepath.Join(dir, "metadata", id+".yaml")
		if _, err := os.Stat(metaPath); err != nil {
			t.Errorf("metadata YAML missing for %s: %v", id, err)
		}
	}

	// Step 5: Verify metadata content for first patent.
	metaPath := filepath.Join(dir, "metadata", "US7654321.yaml")
	metaData, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("reading metadata: %v", err)
	}

	var paper types.Paper
	if err := yaml.Unmarshal(metaData, &paper); err != nil {
		t.Fatalf("parsing metadata YAML: %v", err)
	}

	if paper.ID != "US7654321" {
		t.Errorf("metadata ID = %q, want %q", paper.ID, "US7654321")
	}
	if paper.Title != "Method for testing patents" {
		t.Errorf("metadata Title = %q, want %q", paper.Title, "Method for testing patents")
	}
	if paper.Source != "patentsview" {
		t.Errorf("metadata Source = %q, want %q", paper.Source, "patentsview")
	}
	if len(paper.Authors) < 1 {
		t.Error("metadata should have at least one inventor")
	}
	expectedDate := time.Date(2023, 3, 14, 0, 0, 0, 0, time.UTC)
	if !paper.Date.Equal(expectedDate) {
		t.Errorf("metadata Date = %v, want %v", paper.Date, expectedDate)
	}
}

func TestPipelineIdempotentReacquire(t *testing.T) {
	ts := newPipelineTestServer(t)
	defer ts.Close()
	restore := overrideBaseURLs(ts.URL)
	defer restore()

	dir := t.TempDir()
	cfg := testConfig(dir)

	// First acquisition.
	var buf1 bytes.Buffer
	paper1, skipped1, err := AcquirePaper(ts.Client(), "US7654321", cfg, &buf1)
	if err != nil {
		t.Fatalf("first AcquirePaper: %v", err)
	}
	if skipped1 {
		t.Error("first call should download, not skip")
	}
	if paper1.ID != "US7654321" {
		t.Errorf("paper.ID = %q, want %q", paper1.ID, "US7654321")
	}

	// Second acquisition of the same patent should be skipped.
	var buf2 bytes.Buffer
	paper2, skipped2, err := AcquirePaper(ts.Client(), "US7654321", cfg, &buf2)
	if err != nil {
		t.Fatalf("second AcquirePaper: %v", err)
	}
	if !skipped2 {
		t.Error("second call should skip, not download")
	}
	if paper2.ID != "US7654321" {
		t.Errorf("paper.ID = %q, want %q", paper2.ID, "US7654321")
	}
	if !strings.Contains(buf2.String(), "skipped:") {
		t.Error("second call output should contain 'skipped:'")
	}

	// Verify the PDF file was NOT overwritten (still has original content).
	pdfPath := filepath.Join(dir, "raw", "US7654321.pdf")
	data, err := os.ReadFile(pdfPath)
	if err != nil {
		t.Fatalf("reading PDF: %v", err)
	}
	if string(data) != fakePDFContent {
		t.Errorf("PDF content should be unchanged after skip")
	}
}

func TestPipelineMixedBatchPaperAndPatent(t *testing.T) {
	ts := newPipelineTestServer(t)
	defer ts.Close()
	restore := overrideBaseURLs(ts.URL)
	defer restore()

	dir := t.TempDir()
	cfg := testConfig(dir)
	var buf bytes.Buffer

	identifiers := []string{
		"2301.07041",  // arXiv paper
		"US7654321",   // Patent
	}

	result := AcquireBatch(ts.Client(), identifiers, cfg, &buf)

	if result.Downloaded != 2 {
		t.Errorf("Downloaded = %d, want 2", result.Downloaded)
	}
	if result.Failed != 0 {
		t.Errorf("Failed = %d, want 0", result.Failed)
	}
	if len(result.Papers) != 2 {
		t.Fatalf("len(Papers) = %d, want 2", len(result.Papers))
	}

	// Verify arXiv paper.
	arxivPaper := result.Papers[0]
	if arxivPaper.Source != "arxiv" {
		t.Errorf("arXiv paper source = %q, want %q", arxivPaper.Source, "arxiv")
	}
	arxivPDF := filepath.Join(dir, "raw", "2301.07041.pdf")
	if _, err := os.Stat(arxivPDF); err != nil {
		t.Errorf("arXiv PDF missing: %v", err)
	}
	arxivMeta := filepath.Join(dir, "metadata", "2301.07041.yaml")
	if _, err := os.Stat(arxivMeta); err != nil {
		t.Errorf("arXiv metadata missing: %v", err)
	}

	// Verify patent.
	patentPaper := result.Papers[1]
	if patentPaper.Source != "patentsview" {
		t.Errorf("patent source = %q, want %q", patentPaper.Source, "patentsview")
	}
	patentPDF := filepath.Join(dir, "raw", "US7654321.pdf")
	if _, err := os.Stat(patentPDF); err != nil {
		t.Errorf("patent PDF missing: %v", err)
	}
	patentMeta := filepath.Join(dir, "metadata", "US7654321.yaml")
	if _, err := os.Stat(patentMeta); err != nil {
		t.Errorf("patent metadata missing: %v", err)
	}

	// Verify patent metadata content.
	metaData, err := os.ReadFile(patentMeta)
	if err != nil {
		t.Fatalf("reading patent metadata: %v", err)
	}
	var paper types.Paper
	if err := yaml.Unmarshal(metaData, &paper); err != nil {
		t.Fatalf("parsing metadata YAML: %v", err)
	}
	if paper.Source != "patentsview" {
		t.Errorf("patent metadata source = %q, want %q", paper.Source, "patentsview")
	}

	// Verify batch summary.
	if !strings.Contains(buf.String(), "Batch summary:") {
		t.Error("output should contain batch summary")
	}
}
