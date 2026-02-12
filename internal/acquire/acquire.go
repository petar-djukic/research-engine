// Package acquire downloads papers and creates metadata records.
// Implements: prd001-acquisition (R1-R5);
//
//	docs/ARCHITECTURE § Acquisition.
package acquire

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/pdiddy/research-engine/pkg/types"
)

const (
	rawDir      = "raw"
	metadataDir = "metadata"
)

// BatchResult holds the outcome of a batch acquisition run.
type BatchResult struct {
	Downloaded int
	Skipped    int
	Failed     int
	Papers     []*types.Paper
}

// Total returns the total number of identifiers processed.
func (r BatchResult) Total() int {
	return r.Downloaded + r.Skipped + r.Failed
}

// HasFailures reports whether any papers failed.
func (r BatchResult) HasFailures() bool {
	return r.Failed > 0
}

// AcquirePaper resolves a single identifier, downloads the PDF, and writes
// metadata. If the PDF already exists on disk, it skips the download.
// The skipped return value indicates whether the download was skipped.
func AcquirePaper(client *http.Client, identifier string, cfg types.AcquisitionConfig, w io.Writer) (paper *types.Paper, skipped bool, err error) {
	idType, normalized := Classify(identifier)
	if idType == TypeUnknown {
		return nil, false, fmt.Errorf("unrecognized identifier format: %q", identifier)
	}

	slug := Slug(idType, normalized)
	pdfPath := filepath.Join(cfg.PapersDir, rawDir, slug+".pdf")
	metaPath := filepath.Join(cfg.PapersDir, metadataDir, slug+".yaml")

	// Skip if PDF already exists (R2.4).
	if _, err := os.Stat(pdfPath); err == nil {
		fmt.Fprintf(w, "skipped: %s (already exists)\n", slug)
		p, readErr := readMetadata(metaPath)
		if readErr != nil {
			p = &types.Paper{ID: slug, PDFPath: pdfPath}
		}
		return p, true, nil
	}

	// For DOI identifiers, try OpenAlex first for open-access PDF.
	var source string
	pdfURL := PDFURL(idType, normalized)
	if idType == TypeDOI {
		if oaURL, err := resolveOpenAlex(client, normalized, cfg); err == nil && oaURL != "" {
			pdfURL = oaURL
			source = "openalex"
		}
	}
	if pdfURL == "" {
		return nil, false, fmt.Errorf("cannot resolve PDF URL for %q", identifier)
	}

	// Create directories (R2.3).
	for _, dir := range []string{
		filepath.Join(cfg.PapersDir, rawDir),
		filepath.Join(cfg.PapersDir, metadataDir),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, false, fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}

	fmt.Fprintf(w, "downloading: %s (%s)\n", slug, idType)

	// Download PDF to temp file, rename on success (R2.5).
	if err := downloadFile(client, pdfURL, pdfPath, cfg); err != nil {
		return nil, false, fmt.Errorf("downloading %s: %w", slug, err)
	}

	// Build Paper record (R3.1, R3.2).
	if source == "" {
		source = idType.String()
	}
	p := &types.Paper{
		ID:               slug,
		SourceURL:        pdfURL,
		PDFPath:          pdfPath,
		Source:           source,
		ConversionStatus: types.ConversionNone,
	}

	// Fetch metadata from APIs (R3.3, R3.4, R3.5).
	switch idType {
	case TypeArxiv:
		if err := fetchArxivMetadata(client, normalized, p, cfg); err != nil {
			fmt.Fprintf(w, "  warning: arXiv metadata fetch failed: %v\n", err)
		}
	case TypeDOI:
		if err := fetchCrossRefMetadata(client, normalized, p, cfg); err != nil {
			fmt.Fprintf(w, "  warning: CrossRef metadata fetch failed: %v\n", err)
		}
	}

	// Write metadata YAML (R3.6).
	if err := writeMetadata(p, metaPath); err != nil {
		return nil, false, fmt.Errorf("writing metadata for %s: %w", slug, err)
	}

	return p, false, nil
}

// AcquireBatch processes multiple identifiers, printing per-item status
// and returning a summary. It continues after individual failures (R4.2)
// and applies a delay between consecutive downloads (R5.1).
func AcquireBatch(client *http.Client, identifiers []string, cfg types.AcquisitionConfig, w io.Writer) BatchResult {
	var result BatchResult
	for i, id := range identifiers {
		if i > 0 && cfg.DownloadDelay > 0 {
			time.Sleep(cfg.DownloadDelay)
		}
		paper, wasSkipped, err := AcquirePaper(client, id, cfg, w)
		if err != nil {
			fmt.Fprintf(w, "failed:  %s (%v)\n", id, err)
			result.Failed++
			continue
		}
		if wasSkipped {
			result.Skipped++
		} else {
			result.Downloaded++
		}
		result.Papers = append(result.Papers, paper)
	}
	fmt.Fprintf(w, "\nBatch summary: %d downloaded, %d skipped, %d failed (total: %d)\n",
		result.Downloaded, result.Skipped, result.Failed, result.Total())
	return result
}

// downloadFile fetches url to destPath using a temporary file (R2.5).
// It sets User-Agent (R5.2) and requests PDF via Accept header.
// The HTTP client handles redirect following (R5.3).
func downloadFile(client *http.Client, url, destPath string, cfg types.AcquisitionConfig) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", cfg.UserAgent)
	req.Header.Set("Accept", "application/pdf")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(destPath), ".acquire-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	_, copyErr := io.Copy(tmpFile, resp.Body)
	closeErr := tmpFile.Close()
	if copyErr != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("writing download: %w", copyErr)
	}
	if closeErr != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("closing temp file: %w", closeErr)
	}

	if err := os.Rename(tmpPath, destPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renaming temp file: %w", err)
	}
	return nil
}

// arXiv Atom feed XML structures.
type arxivFeed struct {
	Entries []arxivEntry `xml:"entry"`
}

type arxivEntry struct {
	Title     string        `xml:"title"`
	Summary   string        `xml:"summary"`
	Published string        `xml:"published"`
	Authors   []arxivAuthor `xml:"author"`
}

type arxivAuthor struct {
	Name string `xml:"name"`
}

// fetchArxivMetadata retrieves metadata from the arXiv API (R3.3).
func fetchArxivMetadata(client *http.Client, arxivID string, paper *types.Paper, cfg types.AcquisitionConfig) error {
	apiURL := fmt.Sprintf("%s?id_list=%s", arxivAPIBase, arxivID)

	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", cfg.UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("arXiv API request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("arXiv API returned HTTP %d", resp.StatusCode)
	}

	var feed arxivFeed
	if err := xml.NewDecoder(resp.Body).Decode(&feed); err != nil {
		return fmt.Errorf("parsing arXiv response: %w", err)
	}

	if len(feed.Entries) == 0 {
		return fmt.Errorf("no entries found for arXiv ID %s", arxivID)
	}

	entry := feed.Entries[0]
	paper.Title = strings.TrimSpace(entry.Title)
	paper.Abstract = strings.TrimSpace(entry.Summary)

	for _, a := range entry.Authors {
		paper.Authors = append(paper.Authors, strings.TrimSpace(a.Name))
	}

	if t, parseErr := time.Parse(time.RFC3339, entry.Published); parseErr == nil {
		paper.Date = t
	}
	return nil
}

// CrossRef API JSON structures.
type crossrefResponse struct {
	Message crossrefWork `json:"message"`
}

type crossrefWork struct {
	Title    []string         `json:"title"`
	Abstract string           `json:"abstract"`
	Author   []crossrefAuthor `json:"author"`
	Created  crossrefDate     `json:"created"`
}

type crossrefAuthor struct {
	Given  string `json:"given"`
	Family string `json:"family"`
}

type crossrefDate struct {
	DateParts [][]int `json:"date-parts"`
}

// fetchCrossRefMetadata retrieves metadata from the CrossRef API (R3.4).
func fetchCrossRefMetadata(client *http.Client, doi string, paper *types.Paper, cfg types.AcquisitionConfig) error {
	apiURL := crossrefAPIBase + doi

	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", cfg.UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("CrossRef API request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("CrossRef API returned HTTP %d", resp.StatusCode)
	}

	var cr crossrefResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return fmt.Errorf("parsing CrossRef response: %w", err)
	}

	if len(cr.Message.Title) > 0 {
		paper.Title = cr.Message.Title[0]
	}
	paper.Abstract = cr.Message.Abstract

	for _, a := range cr.Message.Author {
		name := strings.TrimSpace(a.Given + " " + a.Family)
		paper.Authors = append(paper.Authors, name)
	}

	if len(cr.Message.Created.DateParts) > 0 && len(cr.Message.Created.DateParts[0]) >= 3 {
		parts := cr.Message.Created.DateParts[0]
		paper.Date = time.Date(parts[0], time.Month(parts[1]), parts[2], 0, 0, 0, 0, time.UTC)
	}
	return nil
}

// writeMetadata writes a Paper record to a YAML file (R3.6).
func writeMetadata(paper *types.Paper, path string) error {
	data, err := yaml.Marshal(paper)
	if err != nil {
		return fmt.Errorf("marshaling metadata: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

// readMetadata reads a Paper record from a YAML file.
func readMetadata(path string) (*types.Paper, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var paper types.Paper
	if err := yaml.Unmarshal(data, &paper); err != nil {
		return nil, err
	}
	return &paper, nil
}
