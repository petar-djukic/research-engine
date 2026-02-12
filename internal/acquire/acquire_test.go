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

func TestClassify(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantType IdentifierType
		wantNorm string
	}{
		{"arxiv bare", "2301.07041", TypeArxiv, "2301.07041"},
		{"arxiv prefixed", "arXiv:2301.07041", TypeArxiv, "2301.07041"},
		{"arxiv versioned", "2301.07041v2", TypeArxiv, "2301.07041v2"},
		{"arxiv five digit", "2301.12345", TypeArxiv, "2301.12345"},
		{"doi simple", "10.1145/1234567.1234568", TypeDOI, "10.1145/1234567.1234568"},
		{"doi nature", "10.1038/s41586-024-07487-w", TypeDOI, "10.1038/s41586-024-07487-w"},
		{"url https", "https://example.com/paper.pdf", TypeURL, "https://example.com/paper.pdf"},
		{"url http", "http://example.com/paper.pdf", TypeURL, "http://example.com/paper.pdf"},
		{"unknown bare word", "not-an-id", TypeUnknown, "not-an-id"},
		{"unknown empty", "", TypeUnknown, ""},
		{"whitespace trimmed", "  2301.07041  ", TypeArxiv, "2301.07041"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotNorm := Classify(tt.input)
			if gotType != tt.wantType {
				t.Errorf("Classify(%q) type = %v, want %v", tt.input, gotType, tt.wantType)
			}
			if gotNorm != tt.wantNorm {
				t.Errorf("Classify(%q) norm = %q, want %q", tt.input, gotNorm, tt.wantNorm)
			}
		})
	}
}

func TestSlug(t *testing.T) {
	tests := []struct {
		name     string
		idType   IdentifierType
		norm     string
		wantSlug string
	}{
		{"arxiv", TypeArxiv, "2301.07041", "2301.07041"},
		{"doi", TypeDOI, "10.1145/1234567.1234568", "10.1145-1234567.1234568"},
		{"url with filename", TypeURL, "https://example.com/my-paper.pdf", "my-paper"},
		{"url no filename", TypeURL, "https://example.com/", "url-" + urlHashSlug("https://example.com/")[4:]},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Slug(tt.idType, tt.norm)
			if got != tt.wantSlug {
				t.Errorf("Slug(%v, %q) = %q, want %q", tt.idType, tt.norm, got, tt.wantSlug)
			}
		})
	}
}

func TestPDFURL(t *testing.T) {
	tests := []struct {
		name    string
		idType  IdentifierType
		norm    string
		wantURL string
	}{
		{"arxiv", TypeArxiv, "2301.07041", arxivPDFBase + "2301.07041"},
		{"doi", TypeDOI, "10.1145/1234567", doiBase + "10.1145/1234567"},
		{"url passthrough", TypeURL, "https://example.com/paper.pdf", "https://example.com/paper.pdf"},
		{"unknown empty", TypeUnknown, "foo", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PDFURL(tt.idType, tt.norm)
			if got != tt.wantURL {
				t.Errorf("PDFURL(%v, %q) = %q, want %q", tt.idType, tt.norm, got, tt.wantURL)
			}
		})
	}
}

const sampleArxivXML = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <title>Test Paper Title</title>
    <summary>This is the abstract of the test paper.</summary>
    <published>2023-01-17T18:58:28Z</published>
    <author><name>Alice Smith</name></author>
    <author><name>Bob Jones</name></author>
  </entry>
</feed>`

const sampleCrossRefJSON = `{
  "status": "ok",
  "message": {
    "title": ["CrossRef Paper Title"],
    "abstract": "Abstract from CrossRef.",
    "author": [
      {"given": "Carol", "family": "White"},
      {"given": "Dave", "family": "Brown"}
    ],
    "created": {
      "date-parts": [[2023, 6, 15]]
    }
  }
}`

const samplePatentsViewJSON = `{
  "patents": [{
    "patent_title": "Method for testing patents",
    "patent_abstract": "A method for testing patent acquisition.",
    "patent_date": "2023-03-14",
    "inventors": [
      {"inventor_name_last": "Edison"},
      {"inventor_name_last": "Tesla"}
    ]
  }],
  "count": 1,
  "total_patent_count": 1
}`

const fakePDFContent = "%PDF-1.4 fake"

// newTestServer creates an httptest server that serves fake PDF downloads,
// arXiv API responses, and CrossRef API responses based on URL path.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/pdf/"):
			w.Header().Set("Content-Type", "application/pdf")
			fmt.Fprint(w, fakePDFContent)
		case r.URL.Path == "/api/query":
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprint(w, sampleArxivXML)
		case strings.HasPrefix(r.URL.Path, "/works/"):
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, sampleCrossRefJSON)
		case strings.HasPrefix(r.URL.Path, "/openalex/"):
			// Default: no OA location available so DOI falls back to doi.org.
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"best_oa_location": null}`)
		case strings.HasPrefix(r.URL.Path, "/doi/"):
			// Simulate DOI redirect to PDF.
			w.Header().Set("Content-Type", "application/pdf")
			fmt.Fprint(w, fakePDFContent)
		case strings.HasPrefix(r.URL.Path, "/patent-pdf/"):
			w.Header().Set("Content-Type", "application/pdf")
			fmt.Fprint(w, fakePDFContent)
		case strings.HasPrefix(r.URL.Path, "/patentsview-api/"):
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, samplePatentsViewJSON)
		case strings.HasPrefix(r.URL.Path, "/google-patents/"):
			w.Header().Set("Content-Type", "application/pdf")
			fmt.Fprint(w, fakePDFContent)
		default:
			http.NotFound(w, r)
		}
	}))
}

// overrideBaseURLs sets package-level base URLs to point at the test server
// and returns a cleanup function that restores the originals.
func overrideBaseURLs(tsURL string) func() {
	origPDF := arxivPDFBase
	origAPI := arxivAPIBase
	origDOI := doiBase
	origCR := crossrefAPIBase
	origOA := openAlexAPIBase
	origPatent := googlePatentsPDFBase
	origPVAPI := patentsViewAPIBase
	origGPatents := googlePatentsHTMLBase

	arxivPDFBase = tsURL + "/pdf/"
	arxivAPIBase = tsURL + "/api/query"
	doiBase = tsURL + "/doi/"
	crossrefAPIBase = tsURL + "/works/"
	openAlexAPIBase = tsURL + "/openalex/"
	googlePatentsPDFBase = tsURL + "/patent-pdf/"
	patentsViewAPIBase = tsURL + "/patentsview-api/"
	googlePatentsHTMLBase = tsURL + "/google-patents/"

	return func() {
		arxivPDFBase = origPDF
		arxivAPIBase = origAPI
		doiBase = origDOI
		crossrefAPIBase = origCR
		openAlexAPIBase = origOA
		googlePatentsPDFBase = origPatent
		patentsViewAPIBase = origPVAPI
		googlePatentsHTMLBase = origGPatents
	}
}

func testConfig(dir string) types.AcquisitionConfig {
	return types.AcquisitionConfig{
		HTTPConfig: types.HTTPConfig{
			Timeout:   10 * time.Second,
			UserAgent: "research-engine-test/0.1",
		},
		DownloadDelay: 0,
		PapersDir:     dir,
	}
}

func TestAcquirePaperArxiv(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	restore := overrideBaseURLs(ts.URL)
	defer restore()

	dir := t.TempDir()
	cfg := testConfig(dir)
	var buf bytes.Buffer

	paper, skipped, err := AcquirePaper(ts.Client(), "2301.07041", cfg, &buf)
	if err != nil {
		t.Fatalf("AcquirePaper: %v", err)
	}
	if skipped {
		t.Error("expected download, got skipped")
	}
	if paper.ID != "2301.07041" {
		t.Errorf("paper.ID = %q, want %q", paper.ID, "2301.07041")
	}
	if paper.Title != "Test Paper Title" {
		t.Errorf("paper.Title = %q, want %q", paper.Title, "Test Paper Title")
	}
	if len(paper.Authors) != 2 {
		t.Errorf("len(paper.Authors) = %d, want 2", len(paper.Authors))
	}
	if paper.Abstract != "This is the abstract of the test paper." {
		t.Errorf("paper.Abstract = %q, want %q", paper.Abstract, "This is the abstract of the test paper.")
	}

	// Verify PDF file exists.
	pdfPath := filepath.Join(dir, "raw", "2301.07041.pdf")
	data, err := os.ReadFile(pdfPath)
	if err != nil {
		t.Fatalf("reading PDF: %v", err)
	}
	if string(data) != fakePDFContent {
		t.Errorf("PDF content = %q, want %q", string(data), fakePDFContent)
	}

	// Verify metadata YAML exists.
	metaPath := filepath.Join(dir, "metadata", "2301.07041.yaml")
	if _, err := os.Stat(metaPath); err != nil {
		t.Fatalf("metadata file missing: %v", err)
	}

	// Verify output mentions downloading.
	if !strings.Contains(buf.String(), "downloading:") {
		t.Error("output should contain 'downloading:'")
	}
}

func TestAcquirePaperURL(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	dir := t.TempDir()
	cfg := testConfig(dir)
	var buf bytes.Buffer

	pdfURL := ts.URL + "/pdf/direct-paper.pdf"
	paper, skipped, err := AcquirePaper(ts.Client(), pdfURL, cfg, &buf)
	if err != nil {
		t.Fatalf("AcquirePaper: %v", err)
	}
	if skipped {
		t.Error("expected download, got skipped")
	}
	if paper.Title != "" {
		t.Errorf("URL paper should have empty title, got %q", paper.Title)
	}
	if paper.SourceURL != pdfURL {
		t.Errorf("paper.SourceURL = %q, want %q", paper.SourceURL, pdfURL)
	}
}

func TestAcquirePaperSkipExisting(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	restore := overrideBaseURLs(ts.URL)
	defer restore()

	dir := t.TempDir()
	cfg := testConfig(dir)

	// Pre-create the PDF file.
	rawPath := filepath.Join(dir, "raw")
	if err := os.MkdirAll(rawPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rawPath, "2301.07041.pdf"), []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	paper, skipped, err := AcquirePaper(ts.Client(), "2301.07041", cfg, &buf)
	if err != nil {
		t.Fatalf("AcquirePaper: %v", err)
	}
	if !skipped {
		t.Error("expected skipped, got download")
	}
	if paper.ID != "2301.07041" {
		t.Errorf("paper.ID = %q, want %q", paper.ID, "2301.07041")
	}
	if !strings.Contains(buf.String(), "skipped:") {
		t.Error("output should contain 'skipped:'")
	}
}

func TestAcquirePaperDOIViaOpenAlex(t *testing.T) {
	// Use a variable so the handler can reference the server URL after assignment.
	var tsURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/openalex/"):
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"best_oa_location":{"pdf_url":"%s/pdf/oa-paper.pdf","landing_page_url":"https://example.com"}}`, tsURL)
		case strings.HasPrefix(r.URL.Path, "/pdf/"):
			w.Header().Set("Content-Type", "application/pdf")
			fmt.Fprint(w, fakePDFContent)
		case strings.HasPrefix(r.URL.Path, "/works/"):
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, sampleCrossRefJSON)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()
	tsURL = ts.URL

	origOA := openAlexAPIBase
	origCR := crossrefAPIBase
	origDOI := doiBase
	openAlexAPIBase = ts.URL + "/openalex/"
	crossrefAPIBase = ts.URL + "/works/"
	doiBase = ts.URL + "/doi/"
	defer func() {
		openAlexAPIBase = origOA
		crossrefAPIBase = origCR
		doiBase = origDOI
	}()

	dir := t.TempDir()
	cfg := testConfig(dir)
	var buf bytes.Buffer

	paper, skipped, err := AcquirePaper(ts.Client(), "10.1145/1234567.1234568", cfg, &buf)
	if err != nil {
		t.Fatalf("AcquirePaper: %v", err)
	}
	if skipped {
		t.Error("expected download, got skipped")
	}
	if paper.Source != "openalex" {
		t.Errorf("paper.Source = %q, want %q", paper.Source, "openalex")
	}
	if paper.Title != "CrossRef Paper Title" {
		t.Errorf("paper.Title = %q, want %q", paper.Title, "CrossRef Paper Title")
	}

	// Verify PDF was downloaded.
	pdfPath := filepath.Join(dir, "raw", "10.1145-1234567.1234568.pdf")
	if _, err := os.Stat(pdfPath); err != nil {
		t.Fatalf("PDF file missing: %v", err)
	}
}

func TestAcquirePaperDOIFallbackWhenNoOA(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	restore := overrideBaseURLs(ts.URL)
	defer restore()

	dir := t.TempDir()
	cfg := testConfig(dir)
	var buf bytes.Buffer

	paper, skipped, err := AcquirePaper(ts.Client(), "10.1145/1234567.1234568", cfg, &buf)
	if err != nil {
		t.Fatalf("AcquirePaper: %v", err)
	}
	if skipped {
		t.Error("expected download, got skipped")
	}
	// When OpenAlex has no OA, source should be "doi".
	if paper.Source != "doi" {
		t.Errorf("paper.Source = %q, want %q", paper.Source, "doi")
	}
}

func TestAcquirePaperArxivBypassesOpenAlex(t *testing.T) {
	openAlexCalled := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/openalex/"):
			openAlexCalled = true
			http.NotFound(w, r)
		case strings.HasPrefix(r.URL.Path, "/pdf/"):
			w.Header().Set("Content-Type", "application/pdf")
			fmt.Fprint(w, fakePDFContent)
		case r.URL.Path == "/api/query":
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprint(w, sampleArxivXML)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	origPDF := arxivPDFBase
	origAPI := arxivAPIBase
	origOA := openAlexAPIBase
	arxivPDFBase = ts.URL + "/pdf/"
	arxivAPIBase = ts.URL + "/api/query"
	openAlexAPIBase = ts.URL + "/openalex/"
	defer func() {
		arxivPDFBase = origPDF
		arxivAPIBase = origAPI
		openAlexAPIBase = origOA
	}()

	dir := t.TempDir()
	cfg := testConfig(dir)
	var buf bytes.Buffer

	paper, _, err := AcquirePaper(ts.Client(), "2301.07041", cfg, &buf)
	if err != nil {
		t.Fatalf("AcquirePaper: %v", err)
	}
	if openAlexCalled {
		t.Error("OpenAlex should not be called for arXiv identifiers")
	}
	if paper.Source != "arxiv" {
		t.Errorf("paper.Source = %q, want %q", paper.Source, "arxiv")
	}
}

func TestAcquirePaperUnknownIdentifier(t *testing.T) {
	var buf bytes.Buffer
	cfg := testConfig(t.TempDir())

	_, _, err := AcquirePaper(http.DefaultClient, "not-a-valid-id", cfg, &buf)
	if err == nil {
		t.Fatal("expected error for unknown identifier")
	}
	if !strings.Contains(err.Error(), "unrecognized identifier format") {
		t.Errorf("error = %q, want 'unrecognized identifier format'", err.Error())
	}
}

func TestAcquireBatch(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	restore := overrideBaseURLs(ts.URL)
	defer restore()

	dir := t.TempDir()
	cfg := testConfig(dir)
	var buf bytes.Buffer

	identifiers := []string{
		"2301.07041",      // arXiv: should download
		"bad-identifier",  // unknown: should fail
		ts.URL + "/pdf/direct.pdf", // URL: should download
	}

	result := AcquireBatch(ts.Client(), identifiers, cfg, &buf)

	if result.Downloaded != 2 {
		t.Errorf("Downloaded = %d, want 2", result.Downloaded)
	}
	if result.Failed != 1 {
		t.Errorf("Failed = %d, want 1", result.Failed)
	}
	if result.Total() != 3 {
		t.Errorf("Total = %d, want 3", result.Total())
	}
	if !result.HasFailures() {
		t.Error("HasFailures should be true")
	}
	if len(result.Papers) != 2 {
		t.Errorf("len(Papers) = %d, want 2", len(result.Papers))
	}
	if !strings.Contains(buf.String(), "Batch summary:") {
		t.Error("output should contain batch summary")
	}
}

func TestAcquireBatchSkipExisting(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	restore := overrideBaseURLs(ts.URL)
	defer restore()

	dir := t.TempDir()
	cfg := testConfig(dir)

	// Pre-create one PDF so it gets skipped.
	rawPath := filepath.Join(dir, "raw")
	if err := os.MkdirAll(rawPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rawPath, "2301.07041.pdf"), []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	result := AcquireBatch(ts.Client(), []string{"2301.07041"}, cfg, &buf)
	if result.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", result.Skipped)
	}
	if result.Downloaded != 0 {
		t.Errorf("Downloaded = %d, want 0", result.Downloaded)
	}
}

func TestFetchArxivMetadata(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	restore := overrideBaseURLs(ts.URL)
	defer restore()

	cfg := testConfig(t.TempDir())
	paper := &types.Paper{}

	err := fetchArxivMetadata(ts.Client(), "2301.07041", paper, cfg)
	if err != nil {
		t.Fatalf("fetchArxivMetadata: %v", err)
	}

	if paper.Title != "Test Paper Title" {
		t.Errorf("Title = %q, want %q", paper.Title, "Test Paper Title")
	}
	if paper.Abstract != "This is the abstract of the test paper." {
		t.Errorf("Abstract = %q", paper.Abstract)
	}
	if len(paper.Authors) != 2 {
		t.Fatalf("len(Authors) = %d, want 2", len(paper.Authors))
	}
	if paper.Authors[0] != "Alice Smith" {
		t.Errorf("Authors[0] = %q, want %q", paper.Authors[0], "Alice Smith")
	}
	if paper.Authors[1] != "Bob Jones" {
		t.Errorf("Authors[1] = %q, want %q", paper.Authors[1], "Bob Jones")
	}
	expectedDate := time.Date(2023, 1, 17, 18, 58, 28, 0, time.UTC)
	if !paper.Date.Equal(expectedDate) {
		t.Errorf("Date = %v, want %v", paper.Date, expectedDate)
	}
}

func TestFetchCrossRefMetadata(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	restore := overrideBaseURLs(ts.URL)
	defer restore()

	cfg := testConfig(t.TempDir())
	paper := &types.Paper{}

	err := fetchCrossRefMetadata(ts.Client(), "10.1145/1234567", paper, cfg)
	if err != nil {
		t.Fatalf("fetchCrossRefMetadata: %v", err)
	}

	if paper.Title != "CrossRef Paper Title" {
		t.Errorf("Title = %q, want %q", paper.Title, "CrossRef Paper Title")
	}
	if paper.Abstract != "Abstract from CrossRef." {
		t.Errorf("Abstract = %q", paper.Abstract)
	}
	if len(paper.Authors) != 2 {
		t.Fatalf("len(Authors) = %d, want 2", len(paper.Authors))
	}
	if paper.Authors[0] != "Carol White" {
		t.Errorf("Authors[0] = %q, want %q", paper.Authors[0], "Carol White")
	}
	expectedDate := time.Date(2023, 6, 15, 0, 0, 0, 0, time.UTC)
	if !paper.Date.Equal(expectedDate) {
		t.Errorf("Date = %v, want %v", paper.Date, expectedDate)
	}
}

func TestWriteAndReadMetadata(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")

	paper := &types.Paper{
		ID:        "2301.07041",
		SourceURL: "https://arxiv.org/pdf/2301.07041",
		PDFPath:   "/papers/raw/2301.07041.pdf",
		Title:     "Test Paper",
		Authors:   []string{"Alice", "Bob"},
		Date:      time.Date(2023, 1, 17, 0, 0, 0, 0, time.UTC),
		Abstract:  "An abstract.",
	}

	if err := writeMetadata(paper, path); err != nil {
		t.Fatalf("writeMetadata: %v", err)
	}

	got, err := readMetadata(path)
	if err != nil {
		t.Fatalf("readMetadata: %v", err)
	}

	if got.ID != paper.ID {
		t.Errorf("ID = %q, want %q", got.ID, paper.ID)
	}
	if got.Title != paper.Title {
		t.Errorf("Title = %q, want %q", got.Title, paper.Title)
	}
	if len(got.Authors) != 2 {
		t.Errorf("len(Authors) = %d, want 2", len(got.Authors))
	}
	if got.SourceURL != paper.SourceURL {
		t.Errorf("SourceURL = %q, want %q", got.SourceURL, paper.SourceURL)
	}
}
