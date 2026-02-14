// Copyright Mesh Intelligence Inc., 2026. All rights reserved.

package search

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pdiddy/research-engine/internal/httputil"
	"github.com/pdiddy/research-engine/pkg/types"
)

// semanticAPIBase is the Semantic Scholar paper search endpoint. Declared
// as a var so tests can substitute an httptest server.
var semanticAPIBase = "https://api.semanticscholar.org/graph/v1/paper/search"

const semanticFields = "title,abstract,authors,externalIds,year,publicationDate"

// SemanticScholarBackend queries the Semantic Scholar API (R2.2).
type SemanticScholarBackend struct {
	Client *http.Client
	APIKey string
}

// Name returns the backend identifier.
func (b *SemanticScholarBackend) Name() string { return "semantic_scholar" }

// Search queries the Semantic Scholar API and returns results (R2.2).
func (b *SemanticScholarBackend) Search(ctx context.Context, query Query, cfg types.SearchConfig) ([]types.SearchResult, error) {
	q := buildSemanticQuery(query)
	if q == "" {
		return nil, fmt.Errorf("empty Semantic Scholar query")
	}

	maxResults := cfg.MaxResults
	if maxResults <= 0 {
		maxResults = 20
	}

	params := url.Values{
		"query":  {q},
		"limit":  {fmt.Sprintf("%d", maxResults)},
		"fields": {semanticFields},
	}

	// Date filtering via year range.
	if !query.DateFrom.IsZero() || !query.DateTo.IsZero() {
		yearRange := buildYearRange(query.DateFrom, query.DateTo)
		if yearRange != "" {
			params.Set("year", yearRange)
		}
	}

	reqURL := semanticAPIBase + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", cfg.UserAgent)
	if b.APIKey != "" {
		req.Header.Set("x-api-key", b.APIKey)
	}

	resp, err := httputil.DoWithRetry(ctx, b.Client, req, 0)
	if err != nil {
		return nil, fmt.Errorf("Semantic Scholar API request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Semantic Scholar API returned HTTP %d", resp.StatusCode)
	}

	var sr semanticResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, fmt.Errorf("parsing Semantic Scholar response: %w", err)
	}

	total := len(sr.Data)
	var results []types.SearchResult
	for i, paper := range sr.Data {
		r := types.SearchResult{
			Title:    paper.Title,
			Abstract: paper.Abstract,
			Source:   "semantic_scholar",
		}

		for _, a := range paper.Authors {
			r.Authors = append(r.Authors, a.Name)
		}

		if paper.PublicationDate != "" {
			if t, parseErr := time.Parse("2006-01-02", paper.PublicationDate); parseErr == nil {
				r.Date = t
			}
		} else if paper.Year > 0 {
			r.Date = time.Date(paper.Year, 1, 1, 0, 0, 0, 0, time.UTC)
		}

		// Set identifiers: prefer arXiv ID, then DOI (R4.4).
		if paper.ExternalIDs.ArXiv != "" {
			r.Identifier = paper.ExternalIDs.ArXiv
			r.PreferredAcquisitionID = paper.ExternalIDs.ArXiv
		} else if paper.ExternalIDs.DOI != "" {
			r.Identifier = paper.ExternalIDs.DOI
			r.PreferredAcquisitionID = paper.ExternalIDs.DOI
		} else {
			r.Identifier = paper.PaperID
			r.PreferredAcquisitionID = paper.PaperID
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

// buildSemanticQuery combines query fields into a search string.
func buildSemanticQuery(q Query) string {
	var parts []string
	if q.FreeText != "" {
		parts = append(parts, q.FreeText)
	}
	if q.Author != "" {
		parts = append(parts, q.Author)
	}
	for _, kw := range q.Keywords {
		parts = append(parts, kw)
	}
	return strings.Join(parts, " ")
}

// buildYearRange returns a Semantic Scholar year filter string (e.g. "2020-2023").
func buildYearRange(from, to time.Time) string {
	switch {
	case !from.IsZero() && !to.IsZero():
		return fmt.Sprintf("%d-%d", from.Year(), to.Year())
	case !from.IsZero():
		return fmt.Sprintf("%d-", from.Year())
	case !to.IsZero():
		return fmt.Sprintf("-%d", to.Year())
	default:
		return ""
	}
}

// Semantic Scholar API JSON structures.
type semanticResponse struct {
	Total  int             `json:"total"`
	Offset int             `json:"offset"`
	Data   []semanticPaper `json:"data"`
}

type semanticPaper struct {
	PaperID         string            `json:"paperId"`
	Title           string            `json:"title"`
	Abstract        string            `json:"abstract"`
	Year            int               `json:"year"`
	PublicationDate string            `json:"publicationDate"`
	Authors         []semanticAuthor  `json:"authors"`
	ExternalIDs     semanticExternalIDs `json:"externalIds"`
}

type semanticAuthor struct {
	AuthorID string `json:"authorId"`
	Name     string `json:"name"`
}

type semanticExternalIDs struct {
	DOI      string `json:"DOI"`
	ArXiv    string `json:"ArXiv"`
	CorpusID int    `json:"CorpusId"`
}
