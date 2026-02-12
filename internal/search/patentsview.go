// Copyright Mesh Intelligence Inc., 2026. All rights reserved.

// Implements: prd008-patent-search (R1-R3, R5);
//
//	docs/ARCHITECTURE § Search.
package search

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pdiddy/research-engine/pkg/types"
)

// patentsViewSearchBase is the PatentsView patent search endpoint. Declared
// as a var so tests can substitute an httptest server.
var patentsViewSearchBase = "https://search.patentsview.org/api/v1/patent/"

// patentsViewFields lists the fields requested from the API (R2.6).
const patentsViewFields = `["patent_id","patent_title","patent_abstract","patent_date","patent_type","patent_num_claims","inventors.inventor_name_last"]`

// PatentsViewBackend queries the PatentsView API (prd008 R1.1).
type PatentsViewBackend struct {
	Client *http.Client
	APIKey string
}

// Name returns the backend identifier (R1.6).
func (b *PatentsViewBackend) Name() string { return "patentsview" }

// Search queries the PatentsView API and returns results (R1.2).
func (b *PatentsViewBackend) Search(ctx context.Context, query Query, cfg types.SearchConfig) ([]types.SearchResult, error) {
	q := buildPatentsViewQuery(query)
	if q == "" {
		return nil, fmt.Errorf("empty PatentsView query")
	}

	maxResults := cfg.MaxResults
	if maxResults <= 0 {
		maxResults = 20
	}
	if maxResults > 1000 {
		maxResults = 1000
	}

	params := url.Values{
		"q": {q},
		"f": {patentsViewFields},
		"o": {fmt.Sprintf(`{"per_page":%d}`, maxResults)},
	}

	reqURL := patentsViewSearchBase + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", cfg.UserAgent)
	if b.APIKey != "" {
		req.Header.Set("X-Api-Key", b.APIKey)
	}

	resp, err := b.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("PatentsView API request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		retryAfter := resp.Header.Get("Retry-After")
		if retryAfter != "" {
			return nil, fmt.Errorf("PatentsView rate limit exceeded, retry after %s seconds", retryAfter)
		}
		return nil, fmt.Errorf("PatentsView rate limit exceeded (HTTP 429)")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("PatentsView API returned HTTP %d", resp.StatusCode)
	}

	var pvr patentsViewResponse
	if err := json.NewDecoder(resp.Body).Decode(&pvr); err != nil {
		return nil, fmt.Errorf("parsing PatentsView response: %w", err)
	}

	total := len(pvr.Patents)
	var results []types.SearchResult
	for i, patent := range pvr.Patents {
		r := types.SearchResult{
			Title:    patent.PatentTitle,
			Abstract: patent.PatentAbstract,
			Source:   "patentsview",
		}

		// Build identifier with US prefix (R3.2).
		patentID := "US" + patent.PatentID
		r.Identifier = patentID
		r.PreferredAcquisitionID = patentID

		// Authors from inventors (R3.4).
		for _, inv := range patent.Inventors {
			if inv.InventorNameLast != "" {
				r.Authors = append(r.Authors, inv.InventorNameLast)
			}
		}

		// Date parsing (R3.4).
		if patent.PatentDate != "" {
			if t, parseErr := time.Parse("2006-01-02", patent.PatentDate); parseErr == nil {
				r.Date = t
			}
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

// buildPatentsViewQuery constructs the JSON query parameter from structured
// fields using PatentsView operators (R2.1-R2.5).
func buildPatentsViewQuery(q Query) string {
	var conditions []string

	// Free text: _text_any on patent_title and patent_abstract (R2.1).
	if q.FreeText != "" {
		conditions = append(conditions,
			fmt.Sprintf(`{"_or":[{"_text_any":{"patent_title":"%s"}},{"_text_any":{"patent_abstract":"%s"}}]}`,
				escapeJSON(q.FreeText), escapeJSON(q.FreeText)))
	}

	// Author (inventor): _contains on inventors.inventor_name_last (R2.2).
	if q.Author != "" {
		conditions = append(conditions,
			fmt.Sprintf(`{"_contains":{"inventors.inventor_name_last":"%s"}}`,
				escapeJSON(q.Author)))
	}

	// Keywords: _text_all on patent_title and patent_abstract (R2.3).
	if len(q.Keywords) > 0 {
		combined := strings.Join(q.Keywords, " ")
		conditions = append(conditions,
			fmt.Sprintf(`{"_or":[{"_text_all":{"patent_title":"%s"}},{"_text_all":{"patent_abstract":"%s"}}]}`,
				escapeJSON(combined), escapeJSON(combined)))
	}

	// Date range: _gte and _lte on patent_date (R2.4).
	if !q.DateFrom.IsZero() {
		conditions = append(conditions,
			fmt.Sprintf(`{"_gte":{"patent_date":"%s"}}`, q.DateFrom.Format("2006-01-02")))
	}
	if !q.DateTo.IsZero() {
		conditions = append(conditions,
			fmt.Sprintf(`{"_lte":{"patent_date":"%s"}}`, q.DateTo.Format("2006-01-02")))
	}

	if len(conditions) == 0 {
		return ""
	}

	// Combine with _and when multiple conditions exist (R2.5).
	if len(conditions) == 1 {
		return conditions[0]
	}
	return fmt.Sprintf(`{"_and":[%s]}`, strings.Join(conditions, ","))
}

// escapeJSON escapes a string for safe inclusion in a JSON string value.
func escapeJSON(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}

// PatentsView API JSON structures.
type patentsViewResponse struct {
	Patents []patentsViewPatent `json:"patents"`
	Count   int                 `json:"count"`
	Total   int                 `json:"total_patent_count"`
}

type patentsViewPatent struct {
	PatentID       string                   `json:"patent_id"`
	PatentTitle    string                   `json:"patent_title"`
	PatentAbstract string                   `json:"patent_abstract"`
	PatentDate     string                   `json:"patent_date"`
	PatentType     string                   `json:"patent_type"`
	NumClaims      int                      `json:"patent_num_claims"`
	Inventors      []patentsViewInventor    `json:"inventors"`
}

type patentsViewInventor struct {
	InventorNameLast string `json:"inventor_name_last"`
}
