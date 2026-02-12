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

// --- buildOpenAlexQuery ---

func TestBuildOpenAlexQuery(t *testing.T) {
	tests := []struct {
		name  string
		query Query
		want  string
	}{
		{"free text only", Query{FreeText: "attention mechanisms"}, "attention mechanisms"},
		{"author only", Query{Author: "Vaswani"}, "Vaswani"},
		{"keywords only", Query{Keywords: []string{"transformers", "nlp"}}, "transformers nlp"},
		{"combined all fields", Query{FreeText: "attention", Author: "Vaswani", Keywords: []string{"transformer"}}, "attention Vaswani transformer"},
		{"free text and author", Query{FreeText: "attention", Author: "Vaswani"}, "attention Vaswani"},
		{"empty", Query{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildOpenAlexQuery(tt.query)
			if got != tt.want {
				t.Errorf("buildOpenAlexQuery() = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- reconstructAbstract ---

func TestReconstructAbstract(t *testing.T) {
	tests := []struct {
		name  string
		index map[string][]int
		want  string
	}{
		{
			name:  "empty map",
			index: map[string][]int{},
			want:  "",
		},
		{
			name:  "nil map",
			index: nil,
			want:  "",
		},
		{
			name:  "single word",
			index: map[string][]int{"hello": {0}},
			want:  "hello",
		},
		{
			name: "multi-word ordered",
			index: map[string][]int{
				"We":      {0},
				"propose": {1},
				"a":       {2},
				"new":     {3},
				"method":  {4},
			},
			want: "We propose a new method",
		},
		{
			name: "words with shared positions (word appearing multiple times)",
			index: map[string][]int{
				"the": {0, 4},
				"cat": {1},
				"sat": {2},
				"on":  {3},
				"mat": {5},
			},
			want: "the cat sat on the mat",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := reconstructAbstract(tt.index)
			if got != tt.want {
				t.Errorf("reconstructAbstract() = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- Mock OpenAlex server ---

const sampleOpenAlexJSON = `{
  "meta": {"count": 2, "per_page": 20, "page": 1},
  "results": [
    {
      "id": "https://openalex.org/W2741809807",
      "title": "Attention Is All You Need",
      "doi": "https://doi.org/10.5555/3295222.3295349",
      "publication_date": "2017-06-12",
      "publication_year": 2017,
      "authorships": [
        {"author": {"id": "A1", "display_name": "Ashish Vaswani"}},
        {"author": {"id": "A2", "display_name": "Noam Shazeer"}}
      ],
      "abstract_inverted_index": {
        "We": [0],
        "propose": [1],
        "a": [2, 5],
        "new": [3],
        "architecture": [4],
        "based": [6],
        "on": [7],
        "attention": [8]
      },
      "open_access": {"is_oa": true, "oa_status": "green", "oa_url": "https://arxiv.org/pdf/1706.03762"}
    },
    {
      "id": "https://openalex.org/W3210812345",
      "title": "BERT: Pre-training of Deep Bidirectional Transformers",
      "doi": "",
      "publication_date": "",
      "publication_year": 2018,
      "authorships": [
        {"author": {"id": "A3", "display_name": "Jacob Devlin"}}
      ],
      "abstract_inverted_index": {},
      "open_access": {"is_oa": false, "oa_status": "closed", "oa_url": ""}
    }
  ]
}`

func openAlexTestServer(statusCode int, body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		fmt.Fprint(w, body)
	}))
}

// --- OpenAlexBackend.Search ---

func TestOpenAlexBackendSearch(t *testing.T) {
	ts := openAlexTestServer(http.StatusOK, sampleOpenAlexJSON)
	defer ts.Close()

	old := openAlexSearchBase
	openAlexSearchBase = ts.URL
	defer func() { openAlexSearchBase = old }()

	b := &OpenAlexBackend{Client: ts.Client(), Email: "test@example.com"}
	results, err := b.Search(context.Background(), Query{FreeText: "attention"}, testCfg())
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}

	r0 := results[0]
	// DOI should be stripped of https://doi.org/ prefix.
	if r0.Identifier != "10.5555/3295222.3295349" {
		t.Errorf("Identifier = %q, want DOI without prefix", r0.Identifier)
	}
	if r0.PreferredAcquisitionID != "10.5555/3295222.3295349" {
		t.Errorf("PreferredAcquisitionID = %q, want DOI", r0.PreferredAcquisitionID)
	}
	if r0.Title != "Attention Is All You Need" {
		t.Errorf("Title = %q", r0.Title)
	}
	if r0.Source != "openalex" {
		t.Errorf("Source = %q, want %q", r0.Source, "openalex")
	}
	if len(r0.Authors) != 2 || r0.Authors[0] != "Ashish Vaswani" || r0.Authors[1] != "Noam Shazeer" {
		t.Errorf("Authors = %v, want [Ashish Vaswani, Noam Shazeer]", r0.Authors)
	}
	if r0.Date.Year() != 2017 || r0.Date.Month() != 6 || r0.Date.Day() != 12 {
		t.Errorf("Date = %v, want 2017-06-12", r0.Date)
	}
	// Abstract should be reconstructed from inverted index.
	if !strings.Contains(r0.Abstract, "We") || !strings.Contains(r0.Abstract, "attention") {
		t.Errorf("Abstract = %q, should contain reconstructed text", r0.Abstract)
	}

	// Second result has no DOI → should use OpenAlex ID.
	r1 := results[1]
	if r1.Identifier != "https://openalex.org/W3210812345" {
		t.Errorf("Identifier = %q, want OpenAlex ID", r1.Identifier)
	}
	// No publication_date but has publication_year → date should be Jan 1 of that year.
	if r1.Date.Year() != 2018 || r1.Date.Month() != 1 || r1.Date.Day() != 1 {
		t.Errorf("Date = %v, want 2018-01-01", r1.Date)
	}
	if len(r1.Authors) != 1 || r1.Authors[0] != "Jacob Devlin" {
		t.Errorf("Authors = %v, want [Jacob Devlin]", r1.Authors)
	}
	// Empty abstract inverted index → empty abstract.
	if r1.Abstract != "" {
		t.Errorf("Abstract = %q, want empty for empty inverted index", r1.Abstract)
	}
}

// --- DOI preference and identifier stripping ---

func TestOpenAlexBackendDOIPreference(t *testing.T) {
	// Result with DOI prefixed by https://doi.org/ should be stripped.
	jsonWithDOI := `{
		"meta": {"count": 1, "per_page": 20, "page": 1},
		"results": [{
			"id": "https://openalex.org/W123",
			"title": "Test Paper",
			"doi": "https://doi.org/10.1234/test.5678",
			"publication_date": "2023-01-01",
			"authorships": [],
			"abstract_inverted_index": {}
		}]
	}`

	ts := openAlexTestServer(http.StatusOK, jsonWithDOI)
	defer ts.Close()

	old := openAlexSearchBase
	openAlexSearchBase = ts.URL
	defer func() { openAlexSearchBase = old }()

	b := &OpenAlexBackend{Client: ts.Client()}
	results, err := b.Search(context.Background(), Query{FreeText: "test"}, testCfg())
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].Identifier != "10.1234/test.5678" {
		t.Errorf("Identifier = %q, want DOI without https://doi.org/ prefix", results[0].Identifier)
	}
	if results[0].PreferredAcquisitionID != "10.1234/test.5678" {
		t.Errorf("PreferredAcquisitionID = %q, want DOI", results[0].PreferredAcquisitionID)
	}
}

func TestOpenAlexBackendNoDOIFallsBackToID(t *testing.T) {
	jsonNoDOI := `{
		"meta": {"count": 1, "per_page": 20, "page": 1},
		"results": [{
			"id": "https://openalex.org/W999",
			"title": "No DOI Paper",
			"doi": "",
			"publication_date": "2023-06-01",
			"authorships": [],
			"abstract_inverted_index": {}
		}]
	}`

	ts := openAlexTestServer(http.StatusOK, jsonNoDOI)
	defer ts.Close()

	old := openAlexSearchBase
	openAlexSearchBase = ts.URL
	defer func() { openAlexSearchBase = old }()

	b := &OpenAlexBackend{Client: ts.Client()}
	results, err := b.Search(context.Background(), Query{FreeText: "test"}, testCfg())
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if results[0].Identifier != "https://openalex.org/W999" {
		t.Errorf("Identifier = %q, want OpenAlex ID fallback", results[0].Identifier)
	}
}

// --- Position-based scoring ---

func TestOpenAlexBackendPositionScoring(t *testing.T) {
	ts := openAlexTestServer(http.StatusOK, sampleOpenAlexJSON)
	defer ts.Close()

	old := openAlexSearchBase
	openAlexSearchBase = ts.URL
	defer func() { openAlexSearchBase = old }()

	b := &OpenAlexBackend{Client: ts.Client()}
	results, err := b.Search(context.Background(), Query{FreeText: "test"}, testCfg())
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}

	// First result should have highest score.
	if results[0].RelevanceScore != 1.0 {
		t.Errorf("first result score = %f, want 1.0", results[0].RelevanceScore)
	}
	// Last result should have lowest score (1.0 - 1/1*0.9 = 0.1).
	if math.Abs(results[1].RelevanceScore-0.1) > 0.001 {
		t.Errorf("last result score = %f, want ~0.1", results[1].RelevanceScore)
	}
	if results[0].RelevanceScore <= results[1].RelevanceScore {
		t.Error("first result should have higher score than last")
	}
}

func TestOpenAlexBackendSingleResultScoring(t *testing.T) {
	singleJSON := `{
		"meta": {"count": 1, "per_page": 20, "page": 1},
		"results": [{
			"id": "https://openalex.org/W111",
			"title": "Solo Paper",
			"doi": "",
			"publication_date": "2023-01-01",
			"authorships": [],
			"abstract_inverted_index": {}
		}]
	}`

	ts := openAlexTestServer(http.StatusOK, singleJSON)
	defer ts.Close()

	old := openAlexSearchBase
	openAlexSearchBase = ts.URL
	defer func() { openAlexSearchBase = old }()

	b := &OpenAlexBackend{Client: ts.Client()}
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

// --- Date range filtering ---

func TestOpenAlexBackendDateRangeFiltering(t *testing.T) {
	var receivedFilter string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedFilter = r.URL.Query().Get("filter")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"meta":{"count":0,"per_page":20,"page":1},"results":[]}`)
	}))
	defer ts.Close()

	old := openAlexSearchBase
	openAlexSearchBase = ts.URL
	defer func() { openAlexSearchBase = old }()

	b := &OpenAlexBackend{Client: ts.Client()}

	// Both dates set.
	q := Query{
		FreeText: "test",
		DateFrom: time.Date(2020, 1, 15, 0, 0, 0, 0, time.UTC),
		DateTo:   time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC),
	}
	_, err := b.Search(context.Background(), q, testCfg())
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if !strings.Contains(receivedFilter, "from_publication_date:2020-01-15") {
		t.Errorf("filter = %q, should contain from_publication_date:2020-01-15", receivedFilter)
	}
	if !strings.Contains(receivedFilter, "to_publication_date:2023-12-31") {
		t.Errorf("filter = %q, should contain to_publication_date:2023-12-31", receivedFilter)
	}

	// Only from date.
	q = Query{FreeText: "test", DateFrom: time.Date(2021, 6, 1, 0, 0, 0, 0, time.UTC)}
	_, _ = b.Search(context.Background(), q, testCfg())
	if !strings.Contains(receivedFilter, "from_publication_date:2021-06-01") {
		t.Errorf("filter = %q, should contain from_publication_date:2021-06-01", receivedFilter)
	}
	if strings.Contains(receivedFilter, "to_publication_date") {
		t.Errorf("filter = %q, should not contain to_publication_date", receivedFilter)
	}

	// No dates → no filter param.
	q = Query{FreeText: "test"}
	_, _ = b.Search(context.Background(), q, testCfg())
	if receivedFilter != "" {
		t.Errorf("filter = %q, should be empty when no dates set", receivedFilter)
	}
}

// --- Email (mailto) parameter ---

func TestOpenAlexBackendEmailParameter(t *testing.T) {
	var receivedMailto string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMailto = r.URL.Query().Get("mailto")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"meta":{"count":0,"per_page":20,"page":1},"results":[]}`)
	}))
	defer ts.Close()

	old := openAlexSearchBase
	openAlexSearchBase = ts.URL
	defer func() { openAlexSearchBase = old }()

	// With email.
	b := &OpenAlexBackend{Client: ts.Client(), Email: "researcher@example.com"}
	_, _ = b.Search(context.Background(), Query{FreeText: "test"}, testCfg())
	if receivedMailto != "researcher@example.com" {
		t.Errorf("mailto = %q, want %q", receivedMailto, "researcher@example.com")
	}

	// Without email.
	b = &OpenAlexBackend{Client: ts.Client()}
	_, _ = b.Search(context.Background(), Query{FreeText: "test"}, testCfg())
	if receivedMailto != "" {
		t.Errorf("mailto = %q, should be empty when no email set", receivedMailto)
	}
}

// --- Empty query ---

func TestOpenAlexBackendEmptyQuery(t *testing.T) {
	b := &OpenAlexBackend{Client: &http.Client{}}
	_, err := b.Search(context.Background(), Query{}, testCfg())
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Errorf("expected empty query error, got: %v", err)
	}
}

// --- Error cases ---

func TestOpenAlexBackendHTTPNon200(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantSubstr string
	}{
		{"server error", http.StatusInternalServerError, "HTTP 500"},
		{"forbidden", http.StatusForbidden, "HTTP 403"},
		{"bad gateway", http.StatusBadGateway, "HTTP 502"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := openAlexTestServer(tt.statusCode, "")
			defer ts.Close()

			old := openAlexSearchBase
			openAlexSearchBase = ts.URL
			defer func() { openAlexSearchBase = old }()

			b := &OpenAlexBackend{Client: ts.Client()}
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

func TestOpenAlexBackendMalformedJSON(t *testing.T) {
	ts := openAlexTestServer(http.StatusOK, `{not valid json`)
	defer ts.Close()

	old := openAlexSearchBase
	openAlexSearchBase = ts.URL
	defer func() { openAlexSearchBase = old }()

	b := &OpenAlexBackend{Client: ts.Client()}
	_, err := b.Search(context.Background(), Query{FreeText: "test"}, testCfg())
	if err == nil {
		t.Fatal("expected JSON parse error")
	}
	if !strings.Contains(err.Error(), "parsing") {
		t.Errorf("error = %q, should mention parsing", err.Error())
	}
}

func TestOpenAlexBackendEmptyResults(t *testing.T) {
	emptyJSON := `{"meta":{"count":0,"per_page":20,"page":1},"results":[]}`

	ts := openAlexTestServer(http.StatusOK, emptyJSON)
	defer ts.Close()

	old := openAlexSearchBase
	openAlexSearchBase = ts.URL
	defer func() { openAlexSearchBase = old }()

	b := &OpenAlexBackend{Client: ts.Client()}
	results, err := b.Search(context.Background(), Query{FreeText: "nonexistent"}, testCfg())
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("len(results) = %d, want 0", len(results))
	}
}

// --- Backend name ---

func TestOpenAlexBackendName(t *testing.T) {
	b := &OpenAlexBackend{}
	if b.Name() != "openalex" {
		t.Errorf("Name() = %q, want %q", b.Name(), "openalex")
	}
}
