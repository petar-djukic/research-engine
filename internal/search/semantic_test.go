// Copyright Mesh Intelligence Inc., 2026. All rights reserved.

package search

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// --- Query building ---

func TestBuildSemanticQueryCombinations(t *testing.T) {
	tests := []struct {
		name  string
		query Query
		want  string
	}{
		{"free text only", Query{FreeText: "transformer models"}, "transformer models"},
		{"author only", Query{Author: "Vaswani"}, "Vaswani"},
		{"keywords only", Query{Keywords: []string{"attention", "nlp"}}, "attention nlp"},
		{"free text and author", Query{FreeText: "attention", Author: "Vaswani"}, "attention Vaswani"},
		{"free text and keywords", Query{FreeText: "attention", Keywords: []string{"transformers"}}, "attention transformers"},
		{"all fields", Query{FreeText: "attention", Author: "Vaswani", Keywords: []string{"transformers", "nlp"}}, "attention Vaswani transformers nlp"},
		{"empty query", Query{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildSemanticQuery(tt.query)
			if got != tt.want {
				t.Errorf("buildSemanticQuery() = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- Request construction (URL params, headers) ---

func TestSemanticSearchRequestParams(t *testing.T) {
	var capturedReq *http.Request
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedReq = r
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"total":0,"offset":0,"data":[]}`)
	}))
	defer ts.Close()

	old := semanticAPIBase
	semanticAPIBase = ts.URL
	defer func() { semanticAPIBase = old }()

	cfg := testCfg()
	cfg.MaxResults = 15

	b := &SemanticScholarBackend{Client: ts.Client()}
	_, err := b.Search(context.Background(), Query{
		FreeText: "attention",
		DateFrom: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
		DateTo:   time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC),
	}, cfg)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	q := capturedReq.URL.Query()

	// Verify query parameter.
	if got := q.Get("query"); got != "attention" {
		t.Errorf("query param = %q, want %q", got, "attention")
	}

	// Verify limit parameter.
	if got := q.Get("limit"); got != "15" {
		t.Errorf("limit param = %q, want %q", got, "15")
	}

	// Verify fields parameter contains expected fields.
	fields := q.Get("fields")
	for _, f := range []string{"title", "abstract", "authors", "externalIds", "year", "publicationDate"} {
		if !strings.Contains(fields, f) {
			t.Errorf("fields param %q missing %q", fields, f)
		}
	}

	// Verify year range parameter.
	if got := q.Get("year"); got != "2020-2023" {
		t.Errorf("year param = %q, want %q", got, "2020-2023")
	}
}

func TestSemanticSearchAPIKeyHeader(t *testing.T) {
	tests := []struct {
		name      string
		apiKey    string
		wantKey   bool
		wantValue string
	}{
		{"with API key", "test-key-123", true, "test-key-123"},
		{"without API key", "", false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedReq *http.Request
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedReq = r
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"total":0,"offset":0,"data":[]}`)
			}))
			defer ts.Close()

			old := semanticAPIBase
			semanticAPIBase = ts.URL
			defer func() { semanticAPIBase = old }()

			b := &SemanticScholarBackend{Client: ts.Client(), APIKey: tt.apiKey}
			_, err := b.Search(context.Background(), Query{FreeText: "test"}, testCfg())
			if err != nil {
				t.Fatalf("Search: %v", err)
			}

			got := capturedReq.Header.Get("x-api-key")
			if tt.wantKey && got != tt.wantValue {
				t.Errorf("x-api-key header = %q, want %q", got, tt.wantValue)
			}
			if !tt.wantKey && got != "" {
				t.Errorf("x-api-key header should be absent, got %q", got)
			}
		})
	}
}

// --- Identifier preference ---

func TestSemanticSearchIdentifierPreference(t *testing.T) {
	tests := []struct {
		name       string
		paper      string // JSON for a single paper
		wantID     string
		wantAcqID  string
	}{
		{
			"arXiv preferred over DOI",
			`{"paperId":"abc","title":"P","authors":[],"externalIds":{"ArXiv":"1706.03762","DOI":"10.555/test"}}`,
			"1706.03762",
			"1706.03762",
		},
		{
			"DOI when no arXiv",
			`{"paperId":"def","title":"P","authors":[],"externalIds":{"DOI":"10.555/test"}}`,
			"10.555/test",
			"10.555/test",
		},
		{
			"PaperID when no arXiv or DOI",
			`{"paperId":"ghi789","title":"P","authors":[],"externalIds":{}}`,
			"ghi789",
			"ghi789",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := fmt.Sprintf(`{"total":1,"offset":0,"data":[%s]}`, tt.paper)
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, resp)
			}))
			defer ts.Close()

			old := semanticAPIBase
			semanticAPIBase = ts.URL
			defer func() { semanticAPIBase = old }()

			b := &SemanticScholarBackend{Client: ts.Client()}
			results, err := b.Search(context.Background(), Query{FreeText: "test"}, testCfg())
			if err != nil {
				t.Fatalf("Search: %v", err)
			}
			if len(results) != 1 {
				t.Fatalf("len(results) = %d, want 1", len(results))
			}
			if results[0].Identifier != tt.wantID {
				t.Errorf("Identifier = %q, want %q", results[0].Identifier, tt.wantID)
			}
			if results[0].PreferredAcquisitionID != tt.wantAcqID {
				t.Errorf("PreferredAcquisitionID = %q, want %q", results[0].PreferredAcquisitionID, tt.wantAcqID)
			}
		})
	}
}

// --- Error cases ---

func TestSemanticSearchHTTPErrors(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErr    string
	}{
		{"429 rate limit", http.StatusTooManyRequests, "HTTP 429"},
		{"500 server error", http.StatusInternalServerError, "HTTP 500"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer ts.Close()

			old := semanticAPIBase
			semanticAPIBase = ts.URL
			defer func() { semanticAPIBase = old }()

			b := &SemanticScholarBackend{Client: ts.Client()}
			_, err := b.Search(context.Background(), Query{FreeText: "test"}, testCfg())
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestSemanticSearchMalformedJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{invalid json`)
	}))
	defer ts.Close()

	old := semanticAPIBase
	semanticAPIBase = ts.URL
	defer func() { semanticAPIBase = old }()

	b := &SemanticScholarBackend{Client: ts.Client()}
	_, err := b.Search(context.Background(), Query{FreeText: "test"}, testCfg())
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if !strings.Contains(err.Error(), "parsing") {
		t.Errorf("error = %q, want substring 'parsing'", err.Error())
	}
}

func TestSemanticSearchEmptyQuery(t *testing.T) {
	b := &SemanticScholarBackend{Client: http.DefaultClient}
	_, err := b.Search(context.Background(), Query{}, testCfg())
	if err == nil {
		t.Fatal("expected error for empty query")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("error = %q, want substring 'empty'", err.Error())
	}
}

func TestSemanticSearchZeroResults(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"total":0,"offset":0,"data":[]}`)
	}))
	defer ts.Close()

	old := semanticAPIBase
	semanticAPIBase = ts.URL
	defer func() { semanticAPIBase = old }()

	b := &SemanticScholarBackend{Client: ts.Client()}
	results, err := b.Search(context.Background(), Query{FreeText: "obscure topic xyz"}, testCfg())
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("len(results) = %d, want 0", len(results))
	}
}

// --- Position-based scoring ---

func TestSemanticSearchPositionScoring(t *testing.T) {
	// Build a response with 5 papers to verify scoring formula.
	var papers []string
	for i := 0; i < 5; i++ {
		papers = append(papers, fmt.Sprintf(
			`{"paperId":"p%d","title":"Paper %d","authors":[],"externalIds":{"DOI":"10.%d/test"}}`,
			i, i, i,
		))
	}
	resp := fmt.Sprintf(`{"total":5,"offset":0,"data":[%s]}`, strings.Join(papers, ","))

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, resp)
	}))
	defer ts.Close()

	old := semanticAPIBase
	semanticAPIBase = ts.URL
	defer func() { semanticAPIBase = old }()

	b := &SemanticScholarBackend{Client: ts.Client()}
	results, err := b.Search(context.Background(), Query{FreeText: "test"}, testCfg())
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 5 {
		t.Fatalf("len(results) = %d, want 5", len(results))
	}

	// First result should have score 1.0.
	if math.Abs(results[0].RelevanceScore-1.0) > 0.001 {
		t.Errorf("results[0].RelevanceScore = %f, want 1.0", results[0].RelevanceScore)
	}

	// Last result: 1.0 - (4/4)*0.9 = 0.1.
	if math.Abs(results[4].RelevanceScore-0.1) > 0.001 {
		t.Errorf("results[4].RelevanceScore = %f, want 0.1", results[4].RelevanceScore)
	}

	// Scores should be monotonically decreasing.
	for i := 1; i < len(results); i++ {
		if results[i].RelevanceScore >= results[i-1].RelevanceScore {
			t.Errorf("scores not decreasing: [%d]=%f >= [%d]=%f",
				i, results[i].RelevanceScore, i-1, results[i-1].RelevanceScore)
		}
	}
}

func TestSemanticSearchSingleResultScoring(t *testing.T) {
	resp := `{"total":1,"offset":0,"data":[{"paperId":"p0","title":"Solo","authors":[],"externalIds":{}}]}`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, resp)
	}))
	defer ts.Close()

	old := semanticAPIBase
	semanticAPIBase = ts.URL
	defer func() { semanticAPIBase = old }()

	b := &SemanticScholarBackend{Client: ts.Client()}
	results, err := b.Search(context.Background(), Query{FreeText: "test"}, testCfg())
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].RelevanceScore != 1.0 {
		t.Errorf("single result score = %f, want 1.0", results[0].RelevanceScore)
	}
}

// --- Date parsing ---

func TestSemanticSearchDateParsing(t *testing.T) {
	tests := []struct {
		name     string
		paper    string
		wantYear int
		wantMonth time.Month
		wantDay  int
	}{
		{
			"publicationDate preferred",
			`{"paperId":"a","title":"P","authors":[],"year":2017,"publicationDate":"2017-06-12","externalIds":{}}`,
			2017, time.June, 12,
		},
		{
			"year fallback when no publicationDate",
			`{"paperId":"b","title":"P","authors":[],"year":2023,"publicationDate":"","externalIds":{}}`,
			2023, time.January, 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := fmt.Sprintf(`{"total":1,"offset":0,"data":[%s]}`, tt.paper)
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, resp)
			}))
			defer ts.Close()

			old := semanticAPIBase
			semanticAPIBase = ts.URL
			defer func() { semanticAPIBase = old }()

			b := &SemanticScholarBackend{Client: ts.Client()}
			results, err := b.Search(context.Background(), Query{FreeText: "test"}, testCfg())
			if err != nil {
				t.Fatalf("Search: %v", err)
			}
			if len(results) != 1 {
				t.Fatalf("len(results) = %d, want 1", len(results))
			}
			d := results[0].Date
			if d.Year() != tt.wantYear || d.Month() != tt.wantMonth || d.Day() != tt.wantDay {
				t.Errorf("Date = %v, want %d-%02d-%02d", d, tt.wantYear, tt.wantMonth, tt.wantDay)
			}
		})
	}
}

// --- Backend name ---

func TestSemanticScholarBackendName(t *testing.T) {
	b := &SemanticScholarBackend{}
	if got := b.Name(); got != "semantic_scholar" {
		t.Errorf("Name() = %q, want %q", got, "semantic_scholar")
	}
}

// --- Default max results ---

func TestSemanticSearchDefaultMaxResults(t *testing.T) {
	var capturedReq *http.Request
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedReq = r
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"total":0,"offset":0,"data":[]}`)
	}))
	defer ts.Close()

	old := semanticAPIBase
	semanticAPIBase = ts.URL
	defer func() { semanticAPIBase = old }()

	cfg := testCfg()
	cfg.MaxResults = 0 // Should default to 20.

	b := &SemanticScholarBackend{Client: ts.Client()}
	_, err := b.Search(context.Background(), Query{FreeText: "test"}, cfg)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	if got := capturedReq.URL.Query().Get("limit"); got != "20" {
		t.Errorf("limit param = %q, want %q (default)", got, "20")
	}
}

// --- Author parsing ---

func TestSemanticSearchAuthorParsing(t *testing.T) {
	resp := `{"total":1,"offset":0,"data":[{
		"paperId":"x","title":"P",
		"authors":[{"authorId":"1","name":"Alice Smith"},{"authorId":"2","name":"Bob Jones"}],
		"externalIds":{}}]}`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, resp)
	}))
	defer ts.Close()

	old := semanticAPIBase
	semanticAPIBase = ts.URL
	defer func() { semanticAPIBase = old }()

	b := &SemanticScholarBackend{Client: ts.Client()}
	results, err := b.Search(context.Background(), Query{FreeText: "test"}, testCfg())
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if len(results[0].Authors) != 2 {
		t.Fatalf("len(Authors) = %d, want 2", len(results[0].Authors))
	}
	if results[0].Authors[0] != "Alice Smith" {
		t.Errorf("Authors[0] = %q, want %q", results[0].Authors[0], "Alice Smith")
	}
	if results[0].Authors[1] != "Bob Jones" {
		t.Errorf("Authors[1] = %q, want %q", results[0].Authors[1], "Bob Jones")
	}
}

// --- Source field ---

func TestSemanticSearchSourceField(t *testing.T) {
	resp := `{"total":1,"offset":0,"data":[{"paperId":"x","title":"P","authors":[],"externalIds":{}}]}`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, resp)
	}))
	defer ts.Close()

	old := semanticAPIBase
	semanticAPIBase = ts.URL
	defer func() { semanticAPIBase = old }()

	b := &SemanticScholarBackend{Client: ts.Client()}
	results, err := b.Search(context.Background(), Query{FreeText: "test"}, testCfg())
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if results[0].Source != "semantic_scholar" {
		t.Errorf("Source = %q, want %q", results[0].Source, "semantic_scholar")
	}
}
