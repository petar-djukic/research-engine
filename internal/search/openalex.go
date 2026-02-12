// Copyright Mesh Intelligence Inc., 2026. All rights reserved.

package search

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/pdiddy/research-engine/pkg/types"
)

// openAlexSearchBase is the OpenAlex Works search endpoint. Declared as a
// var so tests can substitute an httptest server.
var openAlexSearchBase = "https://api.openalex.org/works"

// OpenAlexBackend queries the OpenAlex API (R2.3).
type OpenAlexBackend struct {
	Client *http.Client
	// Email is sent as mailto parameter for polite pool access.
	Email string
}

// Name returns the backend identifier.
func (b *OpenAlexBackend) Name() string { return "openalex" }

// Search queries the OpenAlex API and returns results.
func (b *OpenAlexBackend) Search(ctx context.Context, query Query, cfg types.SearchConfig) ([]types.SearchResult, error) {
	searchText := buildOpenAlexQuery(query)
	if searchText == "" {
		return nil, fmt.Errorf("empty OpenAlex query")
	}

	maxResults := cfg.MaxResults
	if maxResults <= 0 {
		maxResults = 20
	}
	if maxResults > 200 {
		maxResults = 200
	}

	params := url.Values{
		"search":   {searchText},
		"per_page": {fmt.Sprintf("%d", maxResults)},
		"page":     {"1"},
	}

	// Build filters for date range.
	var filters []string
	if !query.DateFrom.IsZero() {
		filters = append(filters, "from_publication_date:"+query.DateFrom.Format("2006-01-02"))
	}
	if !query.DateTo.IsZero() {
		filters = append(filters, "to_publication_date:"+query.DateTo.Format("2006-01-02"))
	}
	if len(filters) > 0 {
		params.Set("filter", strings.Join(filters, ","))
	}

	if b.Email != "" {
		params.Set("mailto", b.Email)
	}

	reqURL := openAlexSearchBase + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", cfg.UserAgent)

	resp, err := b.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OpenAlex API request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenAlex API returned HTTP %d", resp.StatusCode)
	}

	var oar openAlexResponse
	if err := json.NewDecoder(resp.Body).Decode(&oar); err != nil {
		return nil, fmt.Errorf("parsing OpenAlex response: %w", err)
	}

	total := len(oar.Results)
	var results []types.SearchResult
	for i, work := range oar.Results {
		r := types.SearchResult{
			Title:    work.Title,
			Abstract: reconstructAbstract(work.AbstractInvertedIndex),
			Source:   "openalex",
		}

		for _, authorship := range work.Authorships {
			if authorship.Author.DisplayName != "" {
				r.Authors = append(r.Authors, authorship.Author.DisplayName)
			}
		}

		if work.PublicationDate != "" {
			if t, parseErr := time.Parse("2006-01-02", work.PublicationDate); parseErr == nil {
				r.Date = t
			}
		} else if work.PublicationYear > 0 {
			r.Date = time.Date(work.PublicationYear, 1, 1, 0, 0, 0, 0, time.UTC)
		}

		// Prefer DOI as identifier since OpenAlex is DOI-centric.
		// Strip the https://doi.org/ prefix to get the bare DOI.
		if work.DOI != "" {
			doi := strings.TrimPrefix(work.DOI, "https://doi.org/")
			r.Identifier = doi
			r.PreferredAcquisitionID = doi
		} else if work.ID != "" {
			r.Identifier = work.ID
			r.PreferredAcquisitionID = work.ID
		}

		// Position-based relevance score. OpenAlex returns results
		// sorted by relevance by default.
		if total > 1 {
			r.RelevanceScore = 1.0 - float64(i)/float64(total-1)*0.9
		} else {
			r.RelevanceScore = 1.0
		}

		results = append(results, r)
	}
	return results, nil
}

// buildOpenAlexQuery combines query fields into a search string.
func buildOpenAlexQuery(q Query) string {
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

// reconstructAbstract converts OpenAlex's abstract_inverted_index back to
// plain text. The inverted index maps each word to a list of positions
// where that word appears.
func reconstructAbstract(invertedIndex map[string][]int) string {
	if len(invertedIndex) == 0 {
		return ""
	}

	// Build position→word map.
	type posWord struct {
		pos  int
		word string
	}
	var pairs []posWord
	for word, positions := range invertedIndex {
		for _, pos := range positions {
			pairs = append(pairs, posWord{pos: pos, word: word})
		}
	}

	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].pos < pairs[j].pos
	})

	words := make([]string, len(pairs))
	for i, p := range pairs {
		words[i] = p.word
	}
	return strings.Join(words, " ")
}

// OpenAlex API JSON structures.
type openAlexResponse struct {
	Meta    openAlexMeta   `json:"meta"`
	Results []openAlexWork `json:"results"`
}

type openAlexMeta struct {
	Count   int `json:"count"`
	PerPage int `json:"per_page"`
	Page    int `json:"page"`
}

type openAlexWork struct {
	ID                    string                 `json:"id"`
	Title                 string                 `json:"title"`
	DOI                   string                 `json:"doi"`
	PublicationDate       string                 `json:"publication_date"`
	PublicationYear       int                    `json:"publication_year"`
	Authorships           []openAlexAuthorship   `json:"authorships"`
	AbstractInvertedIndex map[string][]int       `json:"abstract_inverted_index"`
	OpenAccess            openAlexOpenAccess     `json:"open_access"`
}

type openAlexAuthorship struct {
	Author openAlexAuthor `json:"author"`
}

type openAlexAuthor struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

type openAlexOpenAccess struct {
	IsOA     bool   `json:"is_oa"`
	OAStatus string `json:"oa_status"`
	OAURL    string `json:"oa_url"`
}
