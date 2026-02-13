// Copyright Mesh Intelligence Inc., 2026. All rights reserved.

// Package draft provides utilities for loading and validating paper projects.
// Implements: prd007-paper-writing (R4, R5, R6);
//
//	docs/ARCHITECTURE § Claude Skills § write-paper.
package draft

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/pdiddy/research-engine/pkg/types"
)

const (
	outlineFile    = "outline.yaml"
	referencesFile = "references.yaml"
)

// sectionFilePattern matches numbered section files: NN-slug.md.
var sectionFilePattern = regexp.MustCompile(`^\d{2}-.+\.md$`)

// citationPattern matches inline citations: [Key] or [Key1; Key2].
var citationPattern = regexp.MustCompile(`\[([^\[\]]+)\]`)

// LoadOutline reads outline.yaml from a paper project directory.
func LoadOutline(projectDir string) (*types.Outline, error) {
	data, err := os.ReadFile(filepath.Join(projectDir, outlineFile))
	if err != nil {
		return nil, fmt.Errorf("reading outline: %w", err)
	}
	var outline types.Outline
	if err := yaml.Unmarshal(data, &outline); err != nil {
		return nil, fmt.Errorf("parsing outline: %w", err)
	}
	return &outline, nil
}

// LoadReferences reads references.yaml from a paper project directory.
func LoadReferences(projectDir string) (*types.ReferencesFile, error) {
	data, err := os.ReadFile(filepath.Join(projectDir, referencesFile))
	if err != nil {
		return nil, fmt.Errorf("reading references: %w", err)
	}
	var refs types.ReferencesFile
	if err := yaml.Unmarshal(data, &refs); err != nil {
		return nil, fmt.Errorf("parsing references: %w", err)
	}
	return &refs, nil
}

// SectionFiles returns the ordered list of numbered section file paths
// (NN-*.md) in a paper project directory.
func SectionFiles(projectDir string) ([]string, error) {
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return nil, fmt.Errorf("reading project directory: %w", err)
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if sectionFilePattern.MatchString(e.Name()) {
			files = append(files, filepath.Join(projectDir, e.Name()))
		}
	}
	sort.Strings(files)
	return files, nil
}

// ValidateCitations scans section files for inline citation keys and returns
// any keys that have no corresponding entry in references.yaml. Per R6.3.
func ValidateCitations(projectDir string) ([]string, error) {
	refs, err := LoadReferences(projectDir)
	if err != nil {
		return nil, err
	}

	knownKeys := make(map[string]bool)
	for _, r := range refs.Papers {
		knownKeys[r.CitationKey] = true
	}

	files, err := SectionFiles(projectDir)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", filepath.Base(f), err)
		}
		for _, key := range extractCitationKeys(string(data)) {
			if !knownKeys[key] && !seen[key] {
				seen[key] = true
			}
		}
	}

	var missing []string
	for key := range seen {
		missing = append(missing, key)
	}
	sort.Strings(missing)
	return missing, nil
}

// extractCitationKeys finds all citation keys in text. It handles both single
// citations [Key] and multi-citations [Key1; Key2].
func extractCitationKeys(text string) []string {
	matches := citationPattern.FindAllStringSubmatch(text, -1)
	var keys []string
	for _, m := range matches {
		inner := m[1]
		// Split on semicolons for multi-citations.
		parts := strings.Split(inner, ";")
		for _, p := range parts {
			key := strings.TrimSpace(p)
			if key != "" && isCitationKey(key) {
				keys = append(keys, key)
			}
		}
	}
	return keys
}

// isCitationKey checks whether a string looks like a citation key (AuthorYear
// format). It rejects strings that look like Markdown links, image references,
// or other bracket content.
func isCitationKey(s string) bool {
	// Citation keys are alphanumeric, possibly with hyphens.
	// They must contain at least one letter and one digit.
	hasLetter := false
	hasDigit := false
	for _, c := range s {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z':
			hasLetter = true
		case c >= '0' && c <= '9':
			hasDigit = true
		case c == '-', c == '_':
			// allowed
		default:
			return false
		}
	}
	return hasLetter && hasDigit
}

// GenerateBibTeX produces BibTeX content from a ReferencesFile. Per R6.4.
func GenerateBibTeX(refs *types.ReferencesFile) string {
	var b strings.Builder
	for _, r := range refs.Papers {
		fmt.Fprintf(&b, "@article{%s,\n", r.CitationKey)
		fmt.Fprintf(&b, "  title = {%s},\n", r.Title)
		if len(r.Authors) > 0 {
			fmt.Fprintf(&b, "  author = {%s},\n", strings.Join(r.Authors, " and "))
		}
		if r.Year > 0 {
			fmt.Fprintf(&b, "  year = {%d},\n", r.Year)
		}
		if r.Venue != "" {
			fmt.Fprintf(&b, "  journal = {%s},\n", r.Venue)
		}
		fmt.Fprintf(&b, "}\n\n")
	}
	return b.String()
}
