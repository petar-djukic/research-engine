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

// --- Query translation ---

func TestBuildPatentsViewQuery(t *testing.T) {
	tests := []struct {
		name  string
		query Query
		want  string
	}{
		{
			name:  "free text only",
			query: Query{FreeText: "transformer architecture"},
			want:  `{"_or":[{"_text_any":{"patent_title":"transformer architecture"}},{"_text_any":{"patent_abstract":"transformer architecture"}}]}`,
		},
		{
			name:  "inventor filter only",
			query: Query{Author: "Smith"},
			want:  `{"_contains":{"inventors.inventor_name_last":"Smith"}}`,
		},
		{
			name:  "date range only",
			query: Query{FreeText: "test", DateFrom: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), DateTo: time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC)},
			want:  `{"_and":[{"_or":[{"_text_any":{"patent_title":"test"}},{"_text_any":{"patent_abstract":"test"}}]},{"_gte":{"patent_date":"2020-01-01"}},{"_lte":{"patent_date":"2023-12-31"}}]}`,
		},
		{
			name:  "keywords",
			query: Query{Keywords: []string{"neural", "network"}},
			want:  `{"_or":[{"_text_all":{"patent_title":"neural network"}},{"_text_all":{"patent_abstract":"neural network"}}]}`,
		},
		{
			name:  "combined free text and inventor",
			query: Query{FreeText: "attention", Author: "Vaswani"},
			want:  `{"_and":[{"_or":[{"_text_any":{"patent_title":"attention"}},{"_text_any":{"patent_abstract":"attention"}}]},{"_contains":{"inventors.inventor_name_last":"Vaswani"}}]}`,
		},
		{
			name:  "empty query",
			query: Query{},
			want:  "",
		},
		{
			name:  "date-only query (no text or author)",
			query: Query{DateFrom: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)},
			want:  `{"_gte":{"patent_date":"2020-01-01"}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildPatentsViewQuery(tt.query)
			if got != tt.want {
				t.Errorf("buildPatentsViewQuery() =\n  %s\nwant\n  %s", got, tt.want)
			}
		})
	}
}

func TestEscapeJSON(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`normal text`, `normal text`},
		{`text with "quotes"`, `text with \"quotes\"`},
		{`text with \backslash`, `text with \\backslash`},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := escapeJSON(tt.input)
			if got != tt.want {
				t.Errorf("escapeJSON(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --- Mock PatentsView server ---

const samplePatentsViewJSON = `{
  "patents": [
    {
      "patent_id": "7654321",
      "patent_title": "Neural Network Architecture for Data Processing",
      "patent_abstract": "A method for processing data using neural networks.",
      "patent_date": "2020-03-15",
      "patent_type": "utility",
      "patent_num_claims": 20,
      "inventors": [
        {"inventor_name_last": "Smith"},
        {"inventor_name_last": "Jones"}
      ]
    },
    {
      "patent_id": "9876543",
      "patent_title": "Transformer-Based Language Model",
      "patent_abstract": "An improved language model using transformer architecture.",
      "patent_date": "2022-07-01",
      "patent_type": "utility",
      "patent_num_claims": 15,
      "inventors": [
        {"inventor_name_last": "Brown"}
      ]
    }
  ],
  "count": 2,
  "total_patent_count": 2
}`

func patentsViewTestServer(statusCode int, body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		fmt.Fprint(w, body)
	}))
}

// --- PatentsViewBackend.Search ---

func TestPatentsViewBackendSearch(t *testing.T) {
	ts := patentsViewTestServer(http.StatusOK, samplePatentsViewJSON)
	defer ts.Close()

	old := patentsViewSearchBase
	patentsViewSearchBase = ts.URL + "/"
	defer func() { patentsViewSearchBase = old }()

	b := &PatentsViewBackend{Client: ts.Client(), APIKey: "test-key"}
	results, err := b.Search(context.Background(), Query{FreeText: "neural network"}, testCfg())
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}

	r0 := results[0]
	if r0.Identifier != "US7654321" {
		t.Errorf("Identifier = %q, want %q", r0.Identifier, "US7654321")
	}
	if r0.PreferredAcquisitionID != "US7654321" {
		t.Errorf("PreferredAcquisitionID = %q, want %q", r0.PreferredAcquisitionID, "US7654321")
	}
	if r0.Title != "Neural Network Architecture for Data Processing" {
		t.Errorf("Title = %q", r0.Title)
	}
	if r0.Abstract != "A method for processing data using neural networks." {
		t.Errorf("Abstract = %q", r0.Abstract)
	}
	if r0.Source != "patentsview" {
		t.Errorf("Source = %q, want %q", r0.Source, "patentsview")
	}
	if len(r0.Authors) != 2 || r0.Authors[0] != "Smith" || r0.Authors[1] != "Jones" {
		t.Errorf("Authors = %v, want [Smith Jones]", r0.Authors)
	}
	if r0.Date.Year() != 2020 || r0.Date.Month() != 3 || r0.Date.Day() != 15 {
		t.Errorf("Date = %v, want 2020-03-15", r0.Date)
	}

	r1 := results[1]
	if r1.Identifier != "US9876543" {
		t.Errorf("Identifier = %q, want %q", r1.Identifier, "US9876543")
	}
	if len(r1.Authors) != 1 || r1.Authors[0] != "Brown" {
		t.Errorf("Authors = %v, want [Brown]", r1.Authors)
	}
}

func TestPatentsViewBackendPositionScoring(t *testing.T) {
	ts := patentsViewTestServer(http.StatusOK, samplePatentsViewJSON)
	defer ts.Close()

	old := patentsViewSearchBase
	patentsViewSearchBase = ts.URL + "/"
	defer func() { patentsViewSearchBase = old }()

	b := &PatentsViewBackend{Client: ts.Client()}
	results, err := b.Search(context.Background(), Query{FreeText: "test"}, testCfg())
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	// First result should have highest score, last should have lowest.
	if results[0].RelevanceScore != 1.0 {
		t.Errorf("first result score = %f, want 1.0", results[0].RelevanceScore)
	}
	if math.Abs(results[1].RelevanceScore-0.1) > 0.001 {
		t.Errorf("last result score = %f, want ~0.1", results[1].RelevanceScore)
	}
	if results[0].RelevanceScore <= results[1].RelevanceScore {
		t.Error("first result should have higher score than last")
	}
}

func TestPatentsViewBackendSingleResult(t *testing.T) {
	singleJSON := `{"patents":[{"patent_id":"1234567","patent_title":"Solo Patent","patent_abstract":"","patent_date":"2021-01-01","patent_type":"utility","patent_num_claims":5,"inventors":[]}],"count":1,"total_patent_count":1}`

	ts := patentsViewTestServer(http.StatusOK, singleJSON)
	defer ts.Close()

	old := patentsViewSearchBase
	patentsViewSearchBase = ts.URL + "/"
	defer func() { patentsViewSearchBase = old }()

	b := &PatentsViewBackend{Client: ts.Client()}
	results, err := b.Search(context.Background(), Query{FreeText: "test"}, testCfg())
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	// Single result gets score 1.0.
	if results[0].RelevanceScore != 1.0 {
		t.Errorf("single result score = %f, want 1.0", results[0].RelevanceScore)
	}
}

func TestPatentsViewBackendEmptyResults(t *testing.T) {
	emptyJSON := `{"patents":[],"count":0,"total_patent_count":0}`

	ts := patentsViewTestServer(http.StatusOK, emptyJSON)
	defer ts.Close()

	old := patentsViewSearchBase
	patentsViewSearchBase = ts.URL + "/"
	defer func() { patentsViewSearchBase = old }()

	b := &PatentsViewBackend{Client: ts.Client()}
	results, err := b.Search(context.Background(), Query{FreeText: "nonexistent"}, testCfg())
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("len(results) = %d, want 0", len(results))
	}
}

func TestPatentsViewBackendEmptyQuery(t *testing.T) {
	b := &PatentsViewBackend{Client: &http.Client{}}
	_, err := b.Search(context.Background(), Query{}, testCfg())
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Errorf("expected empty query error, got: %v", err)
	}
}

// --- API key header ---

func TestPatentsViewBackendAPIKeyHeader(t *testing.T) {
	var receivedKey string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedKey = r.Header.Get("X-Api-Key")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"patents":[],"count":0,"total_patent_count":0}`)
	}))
	defer ts.Close()

	old := patentsViewSearchBase
	patentsViewSearchBase = ts.URL + "/"
	defer func() { patentsViewSearchBase = old }()

	b := &PatentsViewBackend{Client: ts.Client(), APIKey: "my-secret-key"}
	_, _ = b.Search(context.Background(), Query{FreeText: "test"}, testCfg())

	if receivedKey != "my-secret-key" {
		t.Errorf("X-Api-Key = %q, want %q", receivedKey, "my-secret-key")
	}
}

func TestPatentsViewBackendNoAPIKey(t *testing.T) {
	var receivedKey string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedKey = r.Header.Get("X-Api-Key")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"patents":[],"count":0,"total_patent_count":0}`)
	}))
	defer ts.Close()

	old := patentsViewSearchBase
	patentsViewSearchBase = ts.URL + "/"
	defer func() { patentsViewSearchBase = old }()

	b := &PatentsViewBackend{Client: ts.Client()}
	_, _ = b.Search(context.Background(), Query{FreeText: "test"}, testCfg())

	if receivedKey != "" {
		t.Errorf("X-Api-Key should be empty when no key configured, got %q", receivedKey)
	}
}

// --- MaxResults capping ---

func TestPatentsViewBackendMaxResultsCapping(t *testing.T) {
	var receivedOptions string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedOptions = r.URL.Query().Get("o")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"patents":[],"count":0,"total_patent_count":0}`)
	}))
	defer ts.Close()

	old := patentsViewSearchBase
	patentsViewSearchBase = ts.URL + "/"
	defer func() { patentsViewSearchBase = old }()

	b := &PatentsViewBackend{Client: ts.Client()}

	// MaxResults over 1000 should be capped.
	cfg := testCfg()
	cfg.MaxResults = 5000
	_, _ = b.Search(context.Background(), Query{FreeText: "test"}, cfg)

	if receivedOptions != `{"per_page":1000}` {
		t.Errorf("options = %q, want per_page capped to 1000", receivedOptions)
	}

	// MaxResults of 0 should default to 20.
	cfg.MaxResults = 0
	_, _ = b.Search(context.Background(), Query{FreeText: "test"}, cfg)

	if receivedOptions != `{"per_page":20}` {
		t.Errorf("options = %q, want per_page default 20", receivedOptions)
	}
}

// --- Error cases ---

func TestPatentsViewBackendRateLimit(t *testing.T) {
	// DoWithRetry handles 429 retries. After exhausting retries the backend
	// sees a 429 status and returns "HTTP 429" in the error.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer ts.Close()

	old := patentsViewSearchBase
	patentsViewSearchBase = ts.URL + "/"
	defer func() { patentsViewSearchBase = old }()

	b := &PatentsViewBackend{Client: ts.Client()}
	_, err := b.Search(context.Background(), Query{FreeText: "test"}, testCfg())
	if err == nil {
		t.Fatal("expected rate limit error")
	}
	if !strings.Contains(err.Error(), "429") {
		t.Errorf("error = %q, should mention 429", err.Error())
	}
}

func TestPatentsViewBackendHTTPErrors(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantSubstr string
	}{
		{"forbidden", http.StatusForbidden, "HTTP 403"},
		{"server error", http.StatusInternalServerError, "HTTP 500"},
		{"bad gateway", http.StatusBadGateway, "HTTP 502"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := patentsViewTestServer(tt.statusCode, "")
			defer ts.Close()

			old := patentsViewSearchBase
			patentsViewSearchBase = ts.URL + "/"
			defer func() { patentsViewSearchBase = old }()

			b := &PatentsViewBackend{Client: ts.Client()}
			_, err := b.Search(context.Background(), Query{FreeText: "test"}, testCfg())
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantSubstr) {
				t.Errorf("error = %q, should contain %q", err.Error(), tt.wantSubstr)
			}
		})
	}
}

func TestPatentsViewBackendMalformedJSON(t *testing.T) {
	ts := patentsViewTestServer(http.StatusOK, `{not valid json`)
	defer ts.Close()

	old := patentsViewSearchBase
	patentsViewSearchBase = ts.URL + "/"
	defer func() { patentsViewSearchBase = old }()

	b := &PatentsViewBackend{Client: ts.Client()}
	_, err := b.Search(context.Background(), Query{FreeText: "test"}, testCfg())
	if err == nil {
		t.Fatal("expected JSON parse error")
	}
	if !strings.Contains(err.Error(), "parsing") {
		t.Errorf("error = %q, should mention parsing", err.Error())
	}
}

// --- Date parsing edge case ---

func TestPatentsViewBackendInvalidDate(t *testing.T) {
	jsonWithBadDate := `{"patents":[{"patent_id":"1111111","patent_title":"Test","patent_abstract":"","patent_date":"not-a-date","patent_type":"utility","patent_num_claims":1,"inventors":[]}],"count":1,"total_patent_count":1}`

	ts := patentsViewTestServer(http.StatusOK, jsonWithBadDate)
	defer ts.Close()

	old := patentsViewSearchBase
	patentsViewSearchBase = ts.URL + "/"
	defer func() { patentsViewSearchBase = old }()

	b := &PatentsViewBackend{Client: ts.Client()}
	results, err := b.Search(context.Background(), Query{FreeText: "test"}, testCfg())
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	// Invalid date should result in zero time, not an error.
	if !results[0].Date.IsZero() {
		t.Errorf("Date should be zero for invalid date string, got %v", results[0].Date)
	}
}

// --- Backend name ---

func TestPatentsViewBackendName(t *testing.T) {
	b := &PatentsViewBackend{}
	if b.Name() != "patentsview" {
		t.Errorf("Name() = %q, want %q", b.Name(), "patentsview")
	}
}
