// Copyright Mesh Intelligence Inc., 2026. All rights reserved.

package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pdiddy/research-engine/internal/httputil"
	"github.com/pdiddy/research-engine/pkg/types"
)

func init() {
	// Use a tiny retry delay so 429 tests finish quickly.
	httputil.RetryBaseDelay = 1 * time.Millisecond
}

// --- mock backend ---

type mockBackend struct {
	name    string
	results []types.SearchResult
	err     error
}

func (m *mockBackend) Name() string { return m.name }

func (m *mockBackend) Search(_ context.Context, _ Query, _ types.SearchConfig) ([]types.SearchResult, error) {
	return m.results, m.err
}

func testCfg() types.SearchConfig {
	return types.SearchConfig{
		HTTPConfig: types.HTTPConfig{
			Timeout:   10 * time.Second,
			UserAgent: "test/0.1",
		},
		MaxResults:        20,
		InterBackendDelay: 0,
		RecencyBiasWindow: 2 * 365 * 24 * time.Hour,
	}
}

// --- Query ---

func TestQueryIsEmpty(t *testing.T) {
	tests := []struct {
		name  string
		query Query
		want  bool
	}{
		{"empty", Query{}, true},
		{"free text", Query{FreeText: "attention"}, false},
		{"author only", Query{Author: "Smith"}, false},
		{"keywords only", Query{Keywords: []string{"ml"}}, false},
		{"date only is empty", Query{DateFrom: time.Now()}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.query.IsEmpty(); got != tt.want {
				t.Errorf("IsEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}

// --- Deduplication ---

func TestDeduplicateByIdentifier(t *testing.T) {
	results := []types.SearchResult{
		{Identifier: "2301.07041", Title: "Paper A", Source: "arxiv", RelevanceScore: 0.9},
		{Identifier: "2301.07041", Title: "Paper A (from S2)", Source: "semantic_scholar", RelevanceScore: 0.8},
		{Identifier: "2301.99999", Title: "Paper B", Source: "arxiv", RelevanceScore: 0.7},
	}

	deduped, removed := deduplicate(results)
	if removed != 1 {
		t.Errorf("removed = %d, want 1", removed)
	}
	if len(deduped) != 2 {
		t.Fatalf("len(deduped) = %d, want 2", len(deduped))
	}
	// Merged result should keep higher score and combine sources.
	if deduped[0].RelevanceScore != 0.9 {
		t.Errorf("merged score = %f, want 0.9", deduped[0].RelevanceScore)
	}
	if !strings.Contains(deduped[0].Source, "semantic_scholar") {
		t.Errorf("merged source = %q, should contain both backends", deduped[0].Source)
	}
}

func TestDeduplicateByTitle(t *testing.T) {
	results := []types.SearchResult{
		{Identifier: "arxiv-id-1", Title: "Attention Is All You Need", Source: "arxiv"},
		{Identifier: "doi-10.123", Title: "attention is all you need!", Source: "semantic_scholar"},
	}

	deduped, removed := deduplicate(results)
	if removed != 1 {
		t.Errorf("removed = %d, want 1", removed)
	}
	if len(deduped) != 1 {
		t.Fatalf("len(deduped) = %d, want 1", len(deduped))
	}
}

func TestDeduplicateNoDuplicates(t *testing.T) {
	results := []types.SearchResult{
		{Identifier: "2301.07041", Title: "Paper A", Source: "arxiv"},
		{Identifier: "2301.99999", Title: "Paper B", Source: "arxiv"},
	}

	deduped, removed := deduplicate(results)
	if removed != 0 {
		t.Errorf("removed = %d, want 0", removed)
	}
	if len(deduped) != 2 {
		t.Errorf("len(deduped) = %d, want 2", len(deduped))
	}
}

// --- Patent deduplication ---

func TestDeduplicatePatentByKindCode(t *testing.T) {
	// Same patent from two queries with different kind codes should be deduped.
	results := []types.SearchResult{
		{Identifier: "US7654321B2", Title: "Patent A", Source: "patentsview", RelevanceScore: 0.9},
		{Identifier: "US7654321", Title: "Patent A", Source: "patentsview", RelevanceScore: 0.8},
	}

	deduped, removed := deduplicate(results)
	if removed != 1 {
		t.Errorf("removed = %d, want 1", removed)
	}
	if len(deduped) != 1 {
		t.Fatalf("len(deduped) = %d, want 1", len(deduped))
	}
	// Merged result should keep higher score.
	if deduped[0].RelevanceScore != 0.9 {
		t.Errorf("merged score = %f, want 0.9", deduped[0].RelevanceScore)
	}
}

func TestDeduplicatePatentSameID(t *testing.T) {
	// Exact same patent ID from two backends should be deduped.
	results := []types.SearchResult{
		{Identifier: "US7654321B2", Title: "Patent A", Source: "backend1", RelevanceScore: 0.9},
		{Identifier: "US7654321B2", Title: "Patent A", Source: "backend2", RelevanceScore: 0.7},
	}

	deduped, removed := deduplicate(results)
	if removed != 1 {
		t.Errorf("removed = %d, want 1", removed)
	}
	if len(deduped) != 1 {
		t.Fatalf("len(deduped) = %d, want 1", len(deduped))
	}
	if !strings.Contains(deduped[0].Source, "backend2") {
		t.Errorf("merged source should contain both backends, got %q", deduped[0].Source)
	}
}

func TestDeduplicateNoCrossTypeMerge(t *testing.T) {
	// A paper and a patent with the same title should NOT be deduped.
	results := []types.SearchResult{
		{Identifier: "2301.07041", Title: "Attention Is All You Need", Source: "arxiv", RelevanceScore: 0.9},
		{Identifier: "US7654321B2", Title: "Attention Is All You Need", Source: "patentsview", RelevanceScore: 0.8},
	}

	deduped, removed := deduplicate(results)
	if removed != 0 {
		t.Errorf("removed = %d, want 0 (paper and patent should not be cross-deduped)", removed)
	}
	if len(deduped) != 2 {
		t.Errorf("len(deduped) = %d, want 2", len(deduped))
	}
}

func TestDeduplicateMixedResults(t *testing.T) {
	// Mix of papers and patents with some duplicates within each type.
	results := []types.SearchResult{
		{Identifier: "2301.07041", Title: "Paper A", Source: "arxiv", RelevanceScore: 0.9},
		{Identifier: "2301.07041", Title: "Paper A", Source: "semantic_scholar", RelevanceScore: 0.8},
		{Identifier: "US7654321B2", Title: "Patent X", Source: "patentsview", RelevanceScore: 0.7},
		{Identifier: "US7654321", Title: "Patent X", Source: "patentsview", RelevanceScore: 0.6},
		{Identifier: "2302.00001", Title: "Paper B", Source: "arxiv", RelevanceScore: 0.5},
	}

	deduped, removed := deduplicate(results)
	if removed != 2 {
		t.Errorf("removed = %d, want 2 (one paper dup, one patent dup)", removed)
	}
	if len(deduped) != 3 {
		t.Errorf("len(deduped) = %d, want 3", len(deduped))
	}
}

func TestStripKindCodeSearch(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"US7654321B2", "US7654321"},
		{"US7654321B1", "US7654321"},
		{"US20230012345A1", "US20230012345"},
		{"US7654321", "US7654321"},
		{"2301.07041", "2301.07041"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := stripKindCode(tt.input)
			if got != tt.want {
				t.Errorf("stripKindCode(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestDedupKey(t *testing.T) {
	tests := []struct {
		name   string
		result types.SearchResult
		want   string
	}{
		{"arxiv paper", types.SearchResult{Identifier: "2301.07041", Source: "arxiv"}, "id:2301.07041"},
		{"doi paper", types.SearchResult{Identifier: "10.1234/test", Source: "semantic_scholar"}, "id:10.1234/test"},
		{"patent with kind code", types.SearchResult{Identifier: "US7654321B2", Source: "patentsview"}, "patent:US7654321"},
		{"patent without kind code", types.SearchResult{Identifier: "US7654321", Source: "patentsview"}, "patent:US7654321"},
		{"empty identifier", types.SearchResult{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dedupKey(tt.result)
			if got != tt.want {
				t.Errorf("dedupKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- Ranking ---

func TestApplyRecencyBias(t *testing.T) {
	window := 2 * 365 * 24 * time.Hour
	results := []types.SearchResult{
		{Title: "Recent", Date: time.Now().Add(-30 * 24 * time.Hour), RelevanceScore: 0.5},
		{Title: "Old", Date: time.Now().Add(-5 * 365 * 24 * time.Hour), RelevanceScore: 0.5},
		{Title: "No date", RelevanceScore: 0.5},
	}

	applyRecencyBias(results, window)

	if results[0].RelevanceScore <= 0.5 {
		t.Errorf("recent paper should be boosted, got %f", results[0].RelevanceScore)
	}
	if results[1].RelevanceScore != 0.5 {
		t.Errorf("old paper should not be boosted, got %f", results[1].RelevanceScore)
	}
	if results[2].RelevanceScore != 0.5 {
		t.Errorf("no-date paper should not be boosted, got %f", results[2].RelevanceScore)
	}
	if results[0].RelevanceScore > 1.0 {
		t.Errorf("score should not exceed 1.0, got %f", results[0].RelevanceScore)
	}
}

// --- Search integration ---

func TestSearchEmptyQuery(t *testing.T) {
	var buf bytes.Buffer
	_, err := Search(context.Background(), Query{}, []Backend{&mockBackend{name: "mock"}}, testCfg(), false, &buf)
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Errorf("expected empty query error, got: %v", err)
	}
}

func TestSearchNoBackends(t *testing.T) {
	var buf bytes.Buffer
	_, err := Search(context.Background(), Query{FreeText: "test"}, nil, testCfg(), false, &buf)
	if err == nil || !strings.Contains(err.Error(), "no search backends") {
		t.Errorf("expected no backends error, got: %v", err)
	}
}

func TestSearchContinuesAfterBackendFailure(t *testing.T) {
	failing := &mockBackend{name: "failing", err: fmt.Errorf("network error")}
	working := &mockBackend{
		name: "working",
		results: []types.SearchResult{
			{Identifier: "2301.07041", Title: "Paper A", Source: "working", RelevanceScore: 0.9},
		},
	}

	var buf bytes.Buffer
	out, err := Search(context.Background(), Query{FreeText: "test"}, []Backend{failing, working}, testCfg(), false, &buf)
	if err != nil {
		t.Fatalf("Search should not fail entirely: %v", err)
	}
	if len(out.Results) != 1 {
		t.Errorf("len(Results) = %d, want 1", len(out.Results))
	}
	if len(out.BackendErrors) != 1 {
		t.Errorf("len(BackendErrors) = %d, want 1", len(out.BackendErrors))
	}
	if !strings.Contains(buf.String(), "warning:") {
		t.Error("output should contain warning about failed backend")
	}
}

func TestSearchDedupAndRank(t *testing.T) {
	backend1 := &mockBackend{
		name: "b1",
		results: []types.SearchResult{
			{Identifier: "2301.07041", Title: "Paper A", Source: "b1", RelevanceScore: 0.9},
			{Identifier: "2301.99999", Title: "Paper C", Source: "b1", RelevanceScore: 0.6},
		},
	}
	backend2 := &mockBackend{
		name: "b2",
		results: []types.SearchResult{
			{Identifier: "2301.07041", Title: "Paper A (dup)", Source: "b2", RelevanceScore: 0.8},
			{Identifier: "2302.00001", Title: "Paper B", Source: "b2", RelevanceScore: 0.95},
		},
	}

	var buf bytes.Buffer
	out, err := Search(context.Background(), Query{FreeText: "test"}, []Backend{backend1, backend2}, testCfg(), false, &buf)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if out.DupsRemoved != 1 {
		t.Errorf("DupsRemoved = %d, want 1", out.DupsRemoved)
	}
	if len(out.Results) != 3 {
		t.Errorf("len(Results) = %d, want 3", len(out.Results))
	}
	// Results should be sorted by score descending.
	for i := 1; i < len(out.Results); i++ {
		if out.Results[i].RelevanceScore > out.Results[i-1].RelevanceScore {
			t.Errorf("results not sorted: [%d].Score=%f > [%d].Score=%f",
				i, out.Results[i].RelevanceScore, i-1, out.Results[i-1].RelevanceScore)
		}
	}
}

func TestSearchMaxResults(t *testing.T) {
	var results []types.SearchResult
	for i := 0; i < 30; i++ {
		results = append(results, types.SearchResult{
			Identifier:     fmt.Sprintf("id-%d", i),
			Title:          fmt.Sprintf("Paper %d", i),
			Source:         "mock",
			RelevanceScore: 1.0 - float64(i)/30.0,
		})
	}

	cfg := testCfg()
	cfg.MaxResults = 10
	var buf bytes.Buffer
	out, err := Search(context.Background(), Query{FreeText: "test"}, []Backend{&mockBackend{name: "mock", results: results}}, cfg, false, &buf)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(out.Results) != 10 {
		t.Errorf("len(Results) = %d, want 10", len(out.Results))
	}
}

// --- arXiv backend ---

const sampleArxivSearchXML = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <id>http://arxiv.org/abs/1706.03762v1</id>
    <title>Attention Is All You Need</title>
    <summary>We propose a new architecture based solely on attention mechanisms.</summary>
    <published>2017-06-12T17:57:34Z</published>
    <author><name>Ashish Vaswani</name></author>
    <author><name>Noam Shazeer</name></author>
  </entry>
  <entry>
    <id>http://arxiv.org/abs/1810.04805v2</id>
    <title>BERT: Pre-training of Deep Bidirectional Transformers</title>
    <summary>We introduce BERT.</summary>
    <published>2018-10-11T00:00:00Z</published>
    <author><name>Jacob Devlin</name></author>
  </entry>
</feed>`

func TestArxivBackendSearch(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprint(w, sampleArxivSearchXML)
	}))
	defer ts.Close()

	old := arxivAPIBase
	arxivAPIBase = ts.URL
	defer func() { arxivAPIBase = old }()

	b := &ArxivBackend{Client: ts.Client()}
	results, err := b.Search(context.Background(), Query{FreeText: "attention"}, testCfg())
	if err != nil {
		t.Fatalf("ArxivBackend.Search: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}

	r := results[0]
	if r.Identifier != "1706.03762" {
		t.Errorf("Identifier = %q, want %q", r.Identifier, "1706.03762")
	}
	if r.Title != "Attention Is All You Need" {
		t.Errorf("Title = %q", r.Title)
	}
	if len(r.Authors) != 2 {
		t.Errorf("len(Authors) = %d, want 2", len(r.Authors))
	}
	if r.Source != "arxiv" {
		t.Errorf("Source = %q, want %q", r.Source, "arxiv")
	}
	if r.PreferredAcquisitionID != "1706.03762" {
		t.Errorf("PreferredAcquisitionID = %q", r.PreferredAcquisitionID)
	}
	if r.RelevanceScore < 0.0 || r.RelevanceScore > 1.0 {
		t.Errorf("RelevanceScore = %f, out of range", r.RelevanceScore)
	}
}

func TestExtractArxivID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"http://arxiv.org/abs/2301.07041v1", "2301.07041"},
		{"http://arxiv.org/abs/1706.03762v5", "1706.03762"},
		{"http://arxiv.org/abs/2301.12345", "2301.12345"},
		{"https://arxiv.org/abs/2301.07041v2", "2301.07041"},
		{"not a url", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := extractArxivID(tt.input)
			if got != tt.want {
				t.Errorf("extractArxivID(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestBuildArxivQuery(t *testing.T) {
	tests := []struct {
		name  string
		query Query
		want  string
	}{
		{"free text", Query{FreeText: "attention mechanisms"}, "all:attention+mechanisms"},
		{"author", Query{Author: "Vaswani"}, "au:Vaswani"},
		{"combined", Query{FreeText: "attention", Author: "Vaswani"}, "all:attention+AND+au:Vaswani"},
		{"keywords", Query{Keywords: []string{"transformers", "nlp"}}, "all:transformers+AND+all:nlp"},
		{"empty", Query{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildArxivQuery(tt.query)
			if got != tt.want {
				t.Errorf("buildArxivQuery = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- Semantic Scholar backend ---

const sampleSemanticJSON = `{
  "total": 2,
  "offset": 0,
  "data": [
    {
      "paperId": "abc123",
      "title": "Attention Is All You Need",
      "abstract": "We propose a new architecture.",
      "year": 2017,
      "publicationDate": "2017-06-12",
      "authors": [
        {"authorId": "1", "name": "Ashish Vaswani"},
        {"authorId": "2", "name": "Noam Shazeer"}
      ],
      "externalIds": {"ArXiv": "1706.03762", "DOI": "10.5555/3295222.3295349"}
    },
    {
      "paperId": "def456",
      "title": "GPT-4 Technical Report",
      "abstract": "We report the development of GPT-4.",
      "year": 2023,
      "publicationDate": "2023-03-15",
      "authors": [{"authorId": "3", "name": "OpenAI"}],
      "externalIds": {"DOI": "10.48550/arXiv.2303.08774"}
    }
  ]
}`

func TestSemanticScholarBackendSearch(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, sampleSemanticJSON)
	}))
	defer ts.Close()

	old := semanticAPIBase
	semanticAPIBase = ts.URL
	defer func() { semanticAPIBase = old }()

	b := &SemanticScholarBackend{Client: ts.Client()}
	results, err := b.Search(context.Background(), Query{FreeText: "attention"}, testCfg())
	if err != nil {
		t.Fatalf("SemanticScholarBackend.Search: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}

	// First result has arXiv ID → should be preferred.
	r0 := results[0]
	if r0.Identifier != "1706.03762" {
		t.Errorf("Identifier = %q, want arXiv ID", r0.Identifier)
	}
	if r0.PreferredAcquisitionID != "1706.03762" {
		t.Errorf("PreferredAcquisitionID = %q, want arXiv ID", r0.PreferredAcquisitionID)
	}

	// Second result has no arXiv → DOI should be used.
	r1 := results[1]
	if r1.Identifier != "10.48550/arXiv.2303.08774" {
		t.Errorf("Identifier = %q, want DOI", r1.Identifier)
	}
	if r1.Source != "semantic_scholar" {
		t.Errorf("Source = %q", r1.Source)
	}
}

func TestBuildSemanticQuery(t *testing.T) {
	tests := []struct {
		name  string
		query Query
		want  string
	}{
		{"free text", Query{FreeText: "attention"}, "attention"},
		{"combined", Query{FreeText: "attention", Author: "Vaswani"}, "attention Vaswani"},
		{"keywords", Query{Keywords: []string{"transformers"}}, "transformers"},
		{"empty", Query{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildSemanticQuery(tt.query)
			if got != tt.want {
				t.Errorf("buildSemanticQuery = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildYearRange(t *testing.T) {
	tests := []struct {
		name     string
		from, to time.Time
		want     string
	}{
		{"both", time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC), "2020-2023"},
		{"from only", time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), time.Time{}, "2020-"},
		{"to only", time.Time{}, time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC), "-2023"},
		{"neither", time.Time{}, time.Time{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildYearRange(tt.from, tt.to)
			if got != tt.want {
				t.Errorf("buildYearRange = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- Output formatting ---

func TestFormatTable(t *testing.T) {
	out := SearchOutput{
		Results: []types.SearchResult{
			{Title: "Paper A", Authors: []string{"Smith"}, Date: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC), Source: "arxiv", RelevanceScore: 0.95},
			{Title: "Paper B", Authors: []string{"Jones", "Doe"}, Date: time.Date(2022, 6, 1, 0, 0, 0, 0, time.UTC), Source: "semantic_scholar", RelevanceScore: 0.80},
		},
		DupsRemoved: 1,
	}

	var buf bytes.Buffer
	FormatTable(out, &buf)
	s := buf.String()

	if !strings.Contains(s, "Paper A") {
		t.Error("table should contain 'Paper A'")
	}
	if !strings.Contains(s, "Paper B") {
		t.Error("table should contain 'Paper B'")
	}
	if !strings.Contains(s, "1 duplicates removed") {
		t.Error("table should mention duplicates removed")
	}
}

func TestFormatTableEmpty(t *testing.T) {
	var buf bytes.Buffer
	FormatTable(SearchOutput{}, &buf)
	if !strings.Contains(buf.String(), "No results") {
		t.Error("empty output should say 'No results'")
	}
}

func TestFormatJSON(t *testing.T) {
	out := SearchOutput{
		Results: []types.SearchResult{
			{Identifier: "2301.07041", Title: "Paper A", Source: "arxiv", RelevanceScore: 0.9},
		},
	}

	var buf bytes.Buffer
	if err := FormatJSON(out, &buf); err != nil {
		t.Fatalf("FormatJSON: %v", err)
	}

	var parsed []types.SearchResult
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if len(parsed) != 1 {
		t.Errorf("len(parsed) = %d, want 1", len(parsed))
	}
	if parsed[0].Identifier != "2301.07041" {
		t.Errorf("Identifier = %q", parsed[0].Identifier)
	}
}

// --- Helper functions ---

func TestNormalizeTitle(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Attention Is All You Need", "attention is all you need"},
		{"attention is all you need!", "attention is all you need"},
		{"  BERT:  Pre-training  ", "bert pretraining"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeTitle(tt.input)
			if got != tt.want {
				t.Errorf("normalizeTitle(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsArxivID(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"2301.07041", true},
		{"1706.03762", true},
		{"10.1234/foo", false},
		{"short", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := isArxivID(tt.input); got != tt.want {
				t.Errorf("isArxivID(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// --- CSL output ---

func TestFormatCSL(t *testing.T) {
	out := SearchOutput{
		Results: []types.SearchResult{
			{
				Identifier: "1706.03762",
				Title:      "Attention Is All You Need",
				Authors:    []string{"Ashish Vaswani", "Noam Shazeer"},
				Abstract:   "We propose a new architecture.",
				Date:       time.Date(2017, 6, 12, 0, 0, 0, 0, time.UTC),
				Source:     "arxiv",
			},
			{
				Identifier: "10.1234/example",
				Title:      "A DOI Paper",
				Authors:    []string{"Jane Smith"},
				Date:       time.Date(2023, 3, 15, 0, 0, 0, 0, time.UTC),
				Source:     "semantic_scholar",
			},
		},
	}

	var buf bytes.Buffer
	if err := FormatCSL(out, &buf); err != nil {
		t.Fatalf("FormatCSL: %v", err)
	}

	s := buf.String()
	if !strings.Contains(s, "family: Vaswani") {
		t.Error("CSL output should contain parsed family name 'Vaswani'")
	}
	if !strings.Contains(s, "given: Ashish") {
		t.Error("CSL output should contain parsed given name 'Ashish'")
	}
	if !strings.Contains(s, "type: article") {
		t.Error("CSL output should contain type: article")
	}
	if !strings.Contains(s, "DOI: \"10.1234/example\"") && !strings.Contains(s, "DOI: 10.1234/example") {
		t.Error("CSL output should contain DOI for DOI-identified papers")
	}
	if !strings.Contains(s, "date-parts") {
		t.Error("CSL output should contain date-parts")
	}
}

func TestFormatCSLEmpty(t *testing.T) {
	var buf bytes.Buffer
	if err := FormatCSL(SearchOutput{}, &buf); err != nil {
		t.Fatalf("FormatCSL: %v", err)
	}
	s := buf.String()
	if !strings.Contains(s, "[]") {
		t.Errorf("empty CSL output should be empty list, got: %s", s)
	}
}

func TestParseAuthorName(t *testing.T) {
	tests := []struct {
		input      string
		wantFamily string
		wantGiven  string
		wantLit    string
	}{
		{"Ashish Vaswani", "Vaswani", "Ashish", ""},
		{"Jean-Pierre Dupont", "Dupont", "Jean-Pierre", ""},
		{"OpenAI", "", "", "OpenAI"},
		{"", "", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			n := parseAuthorName(tt.input)
			if n.Family != tt.wantFamily {
				t.Errorf("Family = %q, want %q", n.Family, tt.wantFamily)
			}
			if n.Given != tt.wantGiven {
				t.Errorf("Given = %q, want %q", n.Given, tt.wantGiven)
			}
			if n.Literal != tt.wantLit {
				t.Errorf("Literal = %q, want %q", n.Literal, tt.wantLit)
			}
		})
	}
}

func TestToCSLItem(t *testing.T) {
	r := types.SearchResult{
		Identifier: "10.1234/test",
		Title:      "Test Paper",
		Authors:    []string{"John Smith"},
		Abstract:   "An abstract.",
		Date:       time.Date(2023, 6, 15, 0, 0, 0, 0, time.UTC),
	}

	item := toCSLItem(r)
	if item.Type != "article" {
		t.Errorf("Type = %q, want article", item.Type)
	}
	if item.DOI != "10.1234/test" {
		t.Errorf("DOI = %q, want the identifier", item.DOI)
	}
	if item.Issued == nil || len(item.Issued.DateParts) != 1 {
		t.Fatal("Issued date-parts should have one entry")
	}
	if item.Issued.DateParts[0][0] != 2023 || item.Issued.DateParts[0][1] != 6 || item.Issued.DateParts[0][2] != 15 {
		t.Errorf("date-parts = %v, want [2023 6 15]", item.Issued.DateParts[0])
	}

	// arXiv ID should not set DOI.
	r2 := types.SearchResult{Identifier: "2301.07041", Title: "ArXiv Paper"}
	item2 := toCSLItem(r2)
	if item2.DOI != "" {
		t.Errorf("arXiv ID should not set DOI, got %q", item2.DOI)
	}
}

// --- Query file persistence ---

func TestQueryFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/query.yaml"

	query := Query{
		FreeText: "attention mechanisms",
		Author:   "Vaswani",
		Keywords: []string{"transformers"},
		DateFrom: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	cfg := types.SearchConfig{MaxResults: 10}
	out := SearchOutput{
		Results: []types.SearchResult{
			{Identifier: "1706.03762", Title: "Attention Is All You Need", Authors: []string{"Vaswani"}, RelevanceScore: 0.9},
			{Identifier: "2301.99999", Title: "Another Paper", RelevanceScore: 0.7},
		},
		DupsRemoved:   1,
		BackendErrors: []string{"s2: timeout"},
	}

	if err := WriteQueryFile(path, query, cfg, true, out); err != nil {
		t.Fatalf("WriteQueryFile: %v", err)
	}

	loaded, err := ReadQueryFile(path)
	if err != nil {
		t.Fatalf("ReadQueryFile: %v", err)
	}

	if loaded.Query.FreeText != "attention mechanisms" {
		t.Errorf("FreeText = %q", loaded.Query.FreeText)
	}
	if loaded.Query.Author != "Vaswani" {
		t.Errorf("Author = %q", loaded.Query.Author)
	}
	if len(loaded.Query.Keywords) != 1 || loaded.Query.Keywords[0] != "transformers" {
		t.Errorf("Keywords = %v", loaded.Query.Keywords)
	}
	if loaded.Query.DateFrom != "2020-01-01" {
		t.Errorf("DateFrom = %q", loaded.Query.DateFrom)
	}
	if loaded.Config.MaxResults != 10 {
		t.Errorf("MaxResults = %d", loaded.Config.MaxResults)
	}
	if !loaded.Config.RecencyBias {
		t.Error("RecencyBias should be true")
	}
	if len(loaded.Results) != 2 {
		t.Errorf("len(Results) = %d, want 2", len(loaded.Results))
	}
	if loaded.Results[0].Identifier != "1706.03762" {
		t.Errorf("Results[0].Identifier = %q", loaded.Results[0].Identifier)
	}
	if loaded.Summary.DuplicatesRemoved != 1 {
		t.Errorf("DuplicatesRemoved = %d", loaded.Summary.DuplicatesRemoved)
	}
	if len(loaded.Summary.BackendErrors) != 1 {
		t.Errorf("BackendErrors = %v", loaded.Summary.BackendErrors)
	}
	if loaded.Summary.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
}

func TestQueryFileReadNotFound(t *testing.T) {
	_, err := ReadQueryFile("/nonexistent/query.yaml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestQueryParamsToQuery(t *testing.T) {
	p := QueryParams{
		FreeText: "attention",
		Author:   "Smith",
		Keywords: []string{"ml"},
		DateFrom: "2020-01-15",
		DateTo:   "2023-12-31",
	}
	q, err := p.ToQuery()
	if err != nil {
		t.Fatalf("ToQuery: %v", err)
	}
	if q.FreeText != "attention" {
		t.Errorf("FreeText = %q", q.FreeText)
	}
	if q.DateFrom.Year() != 2020 || q.DateFrom.Month() != 1 || q.DateFrom.Day() != 15 {
		t.Errorf("DateFrom = %v", q.DateFrom)
	}
	if q.DateTo.Year() != 2023 || q.DateTo.Month() != 12 || q.DateTo.Day() != 31 {
		t.Errorf("DateTo = %v", q.DateTo)
	}
}

func TestQueryParamsToQueryInvalidDate(t *testing.T) {
	p := QueryParams{DateFrom: "not-a-date"}
	_, err := p.ToQuery()
	if err == nil {
		t.Error("expected error for invalid date")
	}
}

func TestMergeInto(t *testing.T) {
	dst := types.SearchResult{
		Identifier:             "2301.07041",
		Title:                  "Paper A",
		Source:                 "arxiv",
		RelevanceScore:         0.8,
		PreferredAcquisitionID: "2301.07041",
	}
	src := types.SearchResult{
		Identifier:             "2301.07041",
		Title:                  "Paper A (extended)",
		Authors:                []string{"Smith", "Jones"},
		Abstract:               "An abstract.",
		Source:                 "semantic_scholar",
		RelevanceScore:         0.9,
		PreferredAcquisitionID: "2301.07041",
		Date:                   time.Date(2023, 1, 17, 0, 0, 0, 0, time.UTC),
	}

	mergeInto(&dst, src)

	if len(dst.Authors) != 2 {
		t.Errorf("Authors should be filled from src, got %v", dst.Authors)
	}
	if dst.Abstract != "An abstract." {
		t.Errorf("Abstract should be filled from src")
	}
	if !dst.Date.Equal(time.Date(2023, 1, 17, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("Date should be filled from src")
	}
	if math.Abs(dst.RelevanceScore-0.9) > 0.001 {
		t.Errorf("RelevanceScore should be max(0.8, 0.9) = 0.9, got %f", dst.RelevanceScore)
	}
	if !strings.Contains(dst.Source, "semantic_scholar") {
		t.Errorf("Source should contain both backends, got %q", dst.Source)
	}
}
