// Copyright Mesh Intelligence Inc., 2026. All rights reserved.

package search

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/pdiddy/research-engine/pkg/types"
)

// arxivAPIBase is the arXiv search endpoint. Declared as a var so tests
// can substitute an httptest server.
var arxivAPIBase = "https://export.arxiv.org/api/query"

// ArxivBackend queries the arXiv API (R2.1).
type ArxivBackend struct {
	Client *http.Client
}

// Name returns the backend identifier.
func (b *ArxivBackend) Name() string { return "arxiv" }

// Search queries the arXiv API and returns results (R2.1).
func (b *ArxivBackend) Search(ctx context.Context, query Query, cfg types.SearchConfig) ([]types.SearchResult, error) {
	q := buildArxivQuery(query)
	if q == "" {
		return nil, fmt.Errorf("empty arXiv query")
	}

	maxResults := cfg.MaxResults
	if maxResults <= 0 {
		maxResults = 20
	}

	url := fmt.Sprintf("%s?search_query=%s&start=0&max_results=%d&sortBy=relevance&sortOrder=descending",
		arxivAPIBase, q, maxResults)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", cfg.UserAgent)

	resp, err := b.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("arXiv API request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("arXiv API returned HTTP %d", resp.StatusCode)
	}

	var feed arxivFeed
	if err := xml.NewDecoder(resp.Body).Decode(&feed); err != nil {
		return nil, fmt.Errorf("parsing arXiv response: %w", err)
	}

	total := len(feed.Entries)
	var results []types.SearchResult
	for i, entry := range feed.Entries {
		arxivID := extractArxivID(entry.ID)
		if arxivID == "" {
			continue
		}

		r := types.SearchResult{
			Identifier:             arxivID,
			Title:                  strings.TrimSpace(entry.Title),
			Abstract:               strings.TrimSpace(entry.Summary),
			Source:                 "arxiv",
			PreferredAcquisitionID: arxivID,
		}

		for _, a := range entry.Authors {
			r.Authors = append(r.Authors, strings.TrimSpace(a.Name))
		}

		if t, parseErr := time.Parse(time.RFC3339, entry.Published); parseErr == nil {
			r.Date = t
		}

		// Position-based relevance score (R3.5).
		if total > 1 {
			r.RelevanceScore = 1.0 - float64(i)/float64(total-1)*0.9
		} else {
			r.RelevanceScore = 1.0
		}

		results = append(results, r)
	}
	return results, nil
}

// buildArxivQuery constructs the search_query parameter from structured fields.
func buildArxivQuery(q Query) string {
	var parts []string

	if q.FreeText != "" {
		terms := strings.Fields(q.FreeText)
		parts = append(parts, "all:"+strings.Join(terms, "+"))
	}
	if q.Author != "" {
		terms := strings.Fields(q.Author)
		parts = append(parts, "au:"+strings.Join(terms, "+"))
	}
	for _, kw := range q.Keywords {
		terms := strings.Fields(kw)
		parts = append(parts, "all:"+strings.Join(terms, "+"))
	}

	return strings.Join(parts, "+AND+")
}

// arXiv Atom feed XML structures.
type arxivFeed struct {
	Entries []arxivEntry `xml:"entry"`
}

type arxivEntry struct {
	ID        string        `xml:"id"`
	Title     string        `xml:"title"`
	Summary   string        `xml:"summary"`
	Published string        `xml:"published"`
	Authors   []arxivAuthor `xml:"author"`
}

type arxivAuthor struct {
	Name string `xml:"name"`
}

// extractArxivID pulls the arXiv ID from the entry's <id> URL
// (e.g. "http://arxiv.org/abs/2301.07041v1" → "2301.07041").
func extractArxivID(idURL string) string {
	const prefix = "/abs/"
	idx := strings.Index(idURL, prefix)
	if idx < 0 {
		return ""
	}
	id := idURL[idx+len(prefix):]

	// Strip version suffix (e.g. "v1", "v2").
	if vIdx := strings.LastIndex(id, "v"); vIdx > 0 {
		if _, err := strconv.Atoi(id[vIdx+1:]); err == nil {
			id = id[:vIdx]
		}
	}
	return id
}
