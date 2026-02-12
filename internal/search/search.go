// Copyright Mesh Intelligence Inc., 2026. All rights reserved.

// Package search queries academic APIs and returns unified, deduplicated results.
// Implements: prd006-search (R1-R5);
//
//	docs/ARCHITECTURE § Search.
package search

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/pdiddy/research-engine/pkg/types"
)

// Backend searches a single academic API. Each backend (arXiv, Semantic
// Scholar) implements this interface per the Strategy pattern (R2.6).
type Backend interface {
	Name() string
	Search(ctx context.Context, query Query, cfg types.SearchConfig) ([]types.SearchResult, error)
}

// Query holds the search parameters (R1.1, R1.2, R1.3).
type Query struct {
	FreeText string
	Author   string
	Keywords []string
	DateFrom time.Time
	DateTo   time.Time
}

// IsEmpty reports whether the query contains no searchable terms (R1.5).
func (q Query) IsEmpty() bool {
	return q.FreeText == "" && q.Author == "" && len(q.Keywords) == 0
}

// SearchOutput holds the results and dedup statistics.
type SearchOutput struct {
	Results        []types.SearchResult
	DupsRemoved    int
	BackendErrors  []string
}

// Search fans out the query to all backends concurrently, deduplicates
// results, ranks them, and returns the top N (R1-R4).
func Search(ctx context.Context, query Query, backends []Backend, cfg types.SearchConfig, recencyBias bool, w io.Writer) (SearchOutput, error) {
	if query.IsEmpty() {
		return SearchOutput{}, fmt.Errorf("query is empty: provide a research question or structured parameters")
	}
	if len(backends) == 0 {
		return SearchOutput{}, fmt.Errorf("no search backends configured")
	}

	type backendResult struct {
		results []types.SearchResult
		err     error
		name    string
	}

	ch := make(chan backendResult, len(backends))
	var wg sync.WaitGroup

	for i, b := range backends {
		if i > 0 && cfg.InterBackendDelay > 0 {
			time.Sleep(cfg.InterBackendDelay)
		}
		wg.Add(1)
		go func(b Backend) {
			defer wg.Done()
			results, err := b.Search(ctx, query, cfg)
			ch <- backendResult{results: results, err: err, name: b.Name()}
		}(b)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	var all []types.SearchResult
	var backendErrors []string
	for br := range ch {
		if br.err != nil {
			msg := fmt.Sprintf("%s: %v", br.name, br.err)
			backendErrors = append(backendErrors, msg)
			fmt.Fprintf(w, "warning: backend %s failed: %v\n", br.name, br.err)
			continue
		}
		all = append(all, br.results...)
	}

	deduped, removed := deduplicate(all)

	if recencyBias && cfg.RecencyBiasWindow > 0 {
		applyRecencyBias(deduped, cfg.RecencyBiasWindow)
	}

	sort.Slice(deduped, func(i, j int) bool {
		return deduped[i].RelevanceScore > deduped[j].RelevanceScore
	})

	if cfg.MaxResults > 0 && len(deduped) > cfg.MaxResults {
		deduped = deduped[:cfg.MaxResults]
	}

	return SearchOutput{
		Results:       deduped,
		DupsRemoved:   removed,
		BackendErrors: backendErrors,
	}, nil
}

// deduplicate merges results that share an identifier or normalized title (R3.1, R3.2).
func deduplicate(results []types.SearchResult) ([]types.SearchResult, int) {
	seen := make(map[string]int) // dedup key → index in deduped
	var deduped []types.SearchResult
	removed := 0

	for _, r := range results {
		key := dedupKey(r)
		if idx, ok := seen[key]; ok {
			mergeInto(&deduped[idx], r)
			removed++
			continue
		}

		// Also check by normalized title.
		titleKey := "title:" + normalizeTitle(r.Title)
		if titleKey != "title:" {
			if idx, ok := seen[titleKey]; ok {
				mergeInto(&deduped[idx], r)
				removed++
				continue
			}
		}

		idx := len(deduped)
		deduped = append(deduped, r)
		if key != "" {
			seen[key] = idx
		}
		if titleKey != "title:" {
			seen[titleKey] = idx
		}
	}
	return deduped, removed
}

// dedupKey returns a key for identifier-based dedup. It prefers the
// Identifier field (arXiv ID or DOI set by backends).
func dedupKey(r types.SearchResult) string {
	if r.Identifier != "" {
		return "id:" + r.Identifier
	}
	return ""
}

// mergeInto fills empty fields of dst from src and keeps the higher score (R3.2).
func mergeInto(dst *types.SearchResult, src types.SearchResult) {
	if dst.Title == "" && src.Title != "" {
		dst.Title = src.Title
	}
	if len(dst.Authors) == 0 && len(src.Authors) > 0 {
		dst.Authors = src.Authors
	}
	if dst.Abstract == "" && src.Abstract != "" {
		dst.Abstract = src.Abstract
	}
	if dst.Date.IsZero() && !src.Date.IsZero() {
		dst.Date = src.Date
	}
	if src.RelevanceScore > dst.RelevanceScore {
		dst.RelevanceScore = src.RelevanceScore
	}
	// Prefer arXiv ID for acquisition (R4.4).
	if isArxivID(src.PreferredAcquisitionID) && !isArxivID(dst.PreferredAcquisitionID) {
		dst.PreferredAcquisitionID = src.PreferredAcquisitionID
	}
	if dst.Source != src.Source && !strings.Contains(dst.Source, src.Source) {
		dst.Source = dst.Source + "," + src.Source
	}
}

// isArxivID returns true if the string looks like an arXiv ID (e.g. "2301.07041").
func isArxivID(s string) bool {
	if len(s) < 9 {
		return false
	}
	return s[4] == '.' && s[0] >= '0' && s[0] <= '9'
}

// normalizeTitle returns a lowercased, punctuation-stripped version of the title (R3.1).
func normalizeTitle(title string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(title) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) {
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// applyRecencyBias boosts scores for papers published within the window (R3.4).
func applyRecencyBias(results []types.SearchResult, window time.Duration) {
	now := time.Now()
	for i := range results {
		if results[i].Date.IsZero() {
			continue
		}
		age := now.Sub(results[i].Date)
		if age <= window {
			boost := 0.2 * (1.0 - float64(age)/float64(window))
			results[i].RelevanceScore = math.Min(1.0, results[i].RelevanceScore+boost)
		}
	}
}

// FormatTable writes results as a human-readable table to w (R4.2, R4.5).
func FormatTable(out SearchOutput, w io.Writer) {
	if len(out.Results) == 0 {
		fmt.Fprintln(w, "No results found.")
		return
	}

	fmt.Fprintf(w, "%-4s  %-60s  %-20s  %-4s  %-6s  %s\n",
		"Rank", "Title", "Authors", "Year", "Score", "Source")
	fmt.Fprintln(w, strings.Repeat("-", 110))

	for i, r := range out.Results {
		title := r.Title
		if len(title) > 60 {
			title = title[:57] + "..."
		}
		authors := formatAuthors(r.Authors)
		year := ""
		if !r.Date.IsZero() {
			year = fmt.Sprintf("%d", r.Date.Year())
		}
		fmt.Fprintf(w, "%-4d  %-60s  %-20s  %-4s  %-6.2f  %s\n",
			i+1, title, authors, year, r.RelevanceScore, r.Source)
	}

	fmt.Fprintf(w, "\n%d results", len(out.Results))
	if out.DupsRemoved > 0 {
		fmt.Fprintf(w, " (%d duplicates removed)", out.DupsRemoved)
	}
	fmt.Fprintln(w)
}

// FormatJSON writes results as indented JSON to w (R4.3).
func FormatJSON(out SearchOutput, w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out.Results)
}

func formatAuthors(authors []string) string {
	switch len(authors) {
	case 0:
		return ""
	case 1:
		return truncate(authors[0], 20)
	default:
		return truncate(authors[0], 14) + " et al."
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
