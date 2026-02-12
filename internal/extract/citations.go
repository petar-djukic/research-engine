// Copyright Mesh Intelligence Inc., 2026. All rights reserved.

// Package extract identifies typed knowledge items within converted text.
// citations.go handles citation graph construction and automatic tagging.
// Implements: prd003-extraction (R3, R4);
//
//	docs/ARCHITECTURE § Extraction.
package extract

import (
	"regexp"
	"sort"
	"strings"

	"github.com/pdiddy/research-engine/pkg/types"
)

// Citation regex patterns (R3.1).
var (
	// numericCiteRe matches numeric citations like [1], [2], [12].
	numericCiteRe = regexp.MustCompile(`\[(\d+)\]`)

	// authorYearCiteRe matches author-year citations like
	// [Smith et al., 2020] or [Smith and Jones, 2019].
	authorYearCiteRe = regexp.MustCompile(`\[([A-Z][a-z]+(?:\s+(?:et\s+al\.|and\s+[A-Z][a-z]+))?(?:,\s*\d{4}))\]`)

	// bibEntryRe matches numbered bibliography entries like:
	// [1] Authors. Title. Venue, Year.
	bibEntryRe = regexp.MustCompile(`(?m)^\[(\d+)\]\s+(.+)$`)
)

// ParseCitations scans text for inline citation references and returns
// Citation objects with BibIndex set to -1 (unlinked). Handles numeric
// [N] and author-year [Author, Year] formats (R3.1).
func ParseCitations(text string) []types.Citation {
	seen := make(map[string]bool)
	var citations []types.Citation

	// Numeric citations: [1], [2], etc.
	for _, match := range numericCiteRe.FindAllStringSubmatchIndex(text, -1) {
		key := text[match[2]:match[3]] // capture group 1 (the number)
		fullMatch := text[match[0]:match[1]]
		if seen[fullMatch] {
			continue
		}
		seen[fullMatch] = true
		citations = append(citations, types.Citation{
			Key:      key,
			BibIndex: -1,
			Context:  extractContext(text, match[0], match[1]),
		})
	}

	// Author-year citations: [Smith et al., 2020], etc.
	for _, match := range authorYearCiteRe.FindAllStringSubmatchIndex(text, -1) {
		key := text[match[2]:match[3]]
		fullMatch := text[match[0]:match[1]]
		if seen[fullMatch] {
			continue
		}
		seen[fullMatch] = true
		citations = append(citations, types.Citation{
			Key:      key,
			BibIndex: -1,
			Context:  extractContext(text, match[0], match[1]),
		})
	}

	return citations
}

// extractContext returns a snippet of surrounding text around a citation.
// It takes up to 40 characters before and after the match boundaries.
func extractContext(text string, start, end int) string {
	const window = 40
	ctxStart := start - window
	if ctxStart < 0 {
		ctxStart = 0
	}
	ctxEnd := end + window
	if ctxEnd > len(text) {
		ctxEnd = len(text)
	}
	snippet := text[ctxStart:ctxEnd]
	// Trim to word boundaries.
	if ctxStart > 0 {
		if i := strings.IndexByte(snippet, ' '); i >= 0 && i < window {
			snippet = snippet[i+1:]
		}
	}
	if ctxEnd < len(text) {
		if i := strings.LastIndexByte(snippet, ' '); i >= 0 && i > len(snippet)-window {
			snippet = snippet[:i]
		}
	}
	return strings.TrimSpace(snippet)
}

// ParseBibliography extracts bibliography entries from the references section
// of Markdown content. It looks for a heading containing "references" or
// "bibliography" and parses numbered entries like "[1] Authors. Title." (R3.2).
func ParseBibliography(content string) []types.BibliographyEntry {
	refSection := findReferencesSection(content)
	if refSection == "" {
		return nil
	}

	matches := bibEntryRe.FindAllStringSubmatch(refSection, -1)
	if len(matches) == 0 {
		return nil
	}

	var entries []types.BibliographyEntry
	for _, m := range matches {
		key := m[1]
		raw := strings.TrimSpace(m[2])
		entry := parseBibEntry(key, raw)
		entries = append(entries, entry)
	}
	return entries
}

// findReferencesSection returns the text under a "References" or "Bibliography"
// heading in the Markdown content. Returns empty string if not found.
func findReferencesSection(content string) string {
	lines := strings.Split(content, "\n")
	var collecting bool
	var sectionLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if isHeading(trimmed) {
			heading := strings.ToLower(stripHeadingPrefix(trimmed))
			if strings.Contains(heading, "references") || strings.Contains(heading, "bibliography") {
				collecting = true
				continue
			}
			if collecting {
				break
			}
		}

		if collecting {
			sectionLines = append(sectionLines, line)
		}
	}

	return strings.Join(sectionLines, "\n")
}

// authorBlockRe matches an author section like "Smith, A. and Jones, B." or
// "Brown, T. et al." at the start of a bibliography entry. It captures the
// author block so we can separate it from the title that follows.
var authorBlockRe = regexp.MustCompile(
	`^((?:[A-Z][a-z]+(?:,\s+[A-Z]\.?)?(?:,?\s+(?:and|&)\s+)?)+(?:\s*et\s+al\.)?)\s*[.]?\s+(.+)$`,
)

// parseBibEntry extracts metadata from a raw bibliography entry string.
// It uses regex to identify the author block, then splits the remainder
// into title and venue.
func parseBibEntry(key, raw string) types.BibliographyEntry {
	entry := types.BibliographyEntry{Key: key}
	entry.Year = extractYear(raw)

	m := authorBlockRe.FindStringSubmatch(raw)
	if m != nil {
		entry.Authors = parseAuthors(strings.TrimRight(m[1], ". "))
		remainder := m[2]
		// Split remainder on ". " to get title and venue.
		parts := splitOnPeriods(remainder)
		if len(parts) >= 1 {
			entry.Title = strings.TrimSpace(parts[0])
		}
		if len(parts) >= 2 {
			entry.Venue = cleanVenue(parts[1])
		}
	} else {
		// Fallback: treat first sentence as title.
		parts := splitOnPeriods(raw)
		if len(parts) >= 1 {
			entry.Title = strings.TrimSpace(parts[0])
		}
		if len(parts) >= 2 {
			entry.Venue = cleanVenue(parts[1])
		}
	}

	return entry
}

// yearRe matches a 4-digit year.
var yearRe = regexp.MustCompile(`\b((?:19|20)\d{2})\b`)

// extractYear finds the first 4-digit year (19xx or 20xx) in the text.
func extractYear(text string) string {
	m := yearRe.FindStringSubmatch(text)
	if len(m) >= 2 {
		return m[1]
	}
	return ""
}

// initialRe matches single-letter author initials like "A." or "B." so we
// can protect them from period-based splitting.
var initialRe = regexp.MustCompile(`\b([A-Z])\.`)

// splitOnPeriods splits a bibliography entry into segments at period boundaries,
// but avoids splitting on common abbreviations (et al., e.g., i.e.) and
// single-letter initials (A., B., J.).
func splitOnPeriods(text string) []string {
	// Replace common abbreviations with placeholders to avoid false splits.
	safe := strings.ReplaceAll(text, "et al.", "et al\x00")
	safe = strings.ReplaceAll(safe, "e.g.", "e\x00g\x00")
	safe = strings.ReplaceAll(safe, "i.e.", "i\x00e\x00")

	// Protect single-letter initials: "A." → "A\x00"
	safe = initialRe.ReplaceAllString(safe, "${1}\x00")

	// Split on ". " (period followed by space) or terminal period.
	parts := strings.Split(safe, ". ")

	// Restore placeholders.
	var result []string
	for _, p := range parts {
		p = strings.ReplaceAll(p, "et al\x00", "et al.")
		p = strings.ReplaceAll(p, "e\x00g\x00", "e.g.")
		p = strings.ReplaceAll(p, "i\x00e\x00", "i.e.")
		p = initialRe.ReplaceAllString(p, "${1}.")           // in case any survived
		p = strings.ReplaceAll(p, "\x00", ".")               // restore remaining
		p = strings.TrimRight(p, ".")
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// parseAuthors splits an author string like "Smith, A., Jones, B." into
// individual author names.
func parseAuthors(authorStr string) []string {
	authorStr = strings.TrimSpace(authorStr)
	if authorStr == "" {
		return nil
	}

	// Try splitting on " and " first, then on ", " between full names.
	// Handle "Smith, A., Jones, B." pattern by looking for ", " after initials.
	var authors []string

	// Split on " and " connector.
	halves := strings.SplitN(authorStr, " and ", 2)
	for _, half := range halves {
		half = strings.TrimSpace(half)
		if half == "" {
			continue
		}
		authors = append(authors, half)
	}

	return authors
}

// cleanVenue extracts the venue from a bibliography segment, removing year
// and trailing punctuation.
func cleanVenue(text string) string {
	text = strings.TrimSpace(text)
	text = yearRe.ReplaceAllString(text, "")
	text = strings.TrimRight(text, "., ")
	return strings.TrimSpace(text)
}

// LinkCitations matches Citation objects to BibliographyEntry objects by
// comparing citation keys to bibliography entry keys (R3.3). Numeric
// citations are matched to numbered bibliography entries.
func LinkCitations(citations []types.Citation, bibliography []types.BibliographyEntry) []types.Citation {
	if len(bibliography) == 0 {
		return citations
	}

	keyIndex := make(map[string]int, len(bibliography))
	for i, entry := range bibliography {
		keyIndex[entry.Key] = i
	}

	linked := make([]types.Citation, len(citations))
	copy(linked, citations)

	for i := range linked {
		if idx, ok := keyIndex[linked[i].Key]; ok {
			linked[i].BibIndex = idx
		}
	}

	return linked
}

// AggregatePaperTags collects unique tags from all items and returns them
// sorted alphabetically (R4.3).
func AggregatePaperTags(items []types.KnowledgeItem) []string {
	seen := make(map[string]bool)
	for _, item := range items {
		for _, tag := range item.Tags {
			seen[tag] = true
		}
	}

	tags := make([]string, 0, len(seen))
	for tag := range seen {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	return tags
}
