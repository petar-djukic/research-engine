// Copyright Mesh Intelligence Inc., 2026. All rights reserved.

package search

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/pdiddy/research-engine/pkg/types"
)

func TestToCSLItemPatent(t *testing.T) {
	r := types.SearchResult{
		Identifier: "US7654321B2",
		Title:      "Method for Testing Patents",
		Authors:    []string{"Edison", "Tesla"},
		Abstract:   "A method for testing.",
		Date:       time.Date(2023, 3, 14, 0, 0, 0, 0, time.UTC),
		Source:     "patentsview",
	}

	item := toCSLItem(r)

	if item.Type != "patent" {
		t.Errorf("Type = %q, want %q", item.Type, "patent")
	}
	if item.Number != "US7654321B2" {
		t.Errorf("Number = %q, want %q", item.Number, "US7654321B2")
	}
	if item.Authority != "United States Patent and Trademark Office" {
		t.Errorf("Authority = %q, want %q", item.Authority, "United States Patent and Trademark Office")
	}
	if item.DOI != "" {
		t.Errorf("DOI should be empty for patents, got %q", item.DOI)
	}
	if len(item.Author) != 2 {
		t.Fatalf("len(Author) = %d, want 2", len(item.Author))
	}
	if item.Issued == nil || item.Issued.DateParts[0][0] != 2023 {
		t.Errorf("Issued year should be 2023")
	}
}

func TestToCSLItemPatentByIdentifier(t *testing.T) {
	// Patent detected by identifier pattern even without "patentsview" source.
	r := types.SearchResult{
		Identifier: "US20230012345A1",
		Title:      "Application Patent",
		Source:     "some_other_source",
	}

	item := toCSLItem(r)

	if item.Type != "patent" {
		t.Errorf("Type = %q, want %q (detected by identifier pattern)", item.Type, "patent")
	}
	if item.Number != "US20230012345A1" {
		t.Errorf("Number = %q, want %q", item.Number, "US20230012345A1")
	}
}

func TestToCSLItemArticleNotPatent(t *testing.T) {
	r := types.SearchResult{
		Identifier: "2301.07041",
		Title:      "Attention Is All You Need",
		Source:     "arxiv",
	}

	item := toCSLItem(r)

	if item.Type != "article" {
		t.Errorf("Type = %q, want %q", item.Type, "article")
	}
	if item.Number != "" {
		t.Errorf("Number should be empty for articles, got %q", item.Number)
	}
	if item.Authority != "" {
		t.Errorf("Authority should be empty for articles, got %q", item.Authority)
	}
}

func TestFormatCSLMixedPapersAndPatents(t *testing.T) {
	out := SearchOutput{
		Results: []types.SearchResult{
			{
				Identifier: "1706.03762",
				Title:      "Attention Is All You Need",
				Authors:    []string{"Ashish Vaswani"},
				Date:       time.Date(2017, 6, 12, 0, 0, 0, 0, time.UTC),
				Source:     "arxiv",
			},
			{
				Identifier: "US7654321B2",
				Title:      "Method for Testing Patents",
				Authors:    []string{"Edison"},
				Date:       time.Date(2023, 3, 14, 0, 0, 0, 0, time.UTC),
				Source:     "patentsview",
			},
		},
	}

	var buf bytes.Buffer
	if err := FormatCSL(out, &buf); err != nil {
		t.Fatalf("FormatCSL: %v", err)
	}

	s := buf.String()

	// The article should have type: article.
	if !strings.Contains(s, "type: article") {
		t.Error("CSL output should contain type: article for paper")
	}

	// The patent should have type: patent.
	if !strings.Contains(s, "type: patent") {
		t.Error("CSL output should contain type: patent for patent result")
	}

	// The patent should have number field.
	if !strings.Contains(s, "number: US7654321B2") {
		t.Error("CSL output should contain patent number")
	}

	// The patent should have authority field.
	if !strings.Contains(s, "authority: United States Patent and Trademark Office") {
		t.Error("CSL output should contain authority")
	}

	// The article should NOT have number or authority.
	// Check by counting occurrences: exactly one "number:" and one "authority:".
	if strings.Count(s, "number:") != 1 {
		t.Errorf("expected exactly 1 number field, got %d", strings.Count(s, "number:"))
	}
	if strings.Count(s, "authority:") != 1 {
		t.Errorf("expected exactly 1 authority field, got %d", strings.Count(s, "authority:"))
	}
}

func TestIsPatentResult(t *testing.T) {
	tests := []struct {
		name   string
		result types.SearchResult
		want   bool
	}{
		{"patentsview source", types.SearchResult{Source: "patentsview"}, true},
		{"US patent ID", types.SearchResult{Identifier: "US7654321B2", Source: "other"}, true},
		{"US application ID", types.SearchResult{Identifier: "US20230012345A1"}, true},
		{"arXiv ID", types.SearchResult{Identifier: "2301.07041", Source: "arxiv"}, false},
		{"DOI", types.SearchResult{Identifier: "10.1234/test", Source: "semantic_scholar"}, false},
		{"empty", types.SearchResult{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPatentResult(tt.result)
			if got != tt.want {
				t.Errorf("isPatentResult() = %v, want %v", got, tt.want)
			}
		})
	}
}
