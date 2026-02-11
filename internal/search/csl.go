package search

import (
	"io"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/pdiddy/research-engine/pkg/types"
)

// CSLItem represents a bibliographic entry in CSL (Citation Style Language)
// format. The field names and structure follow the CSL-JSON/CSL-YAML schema
// so that output is consumable by Pandoc and reference managers.
// Implements: prd006-search R4.7.
type CSLItem struct {
	ID       string    `yaml:"id"`
	Type     string    `yaml:"type"`
	Title    string    `yaml:"title"`
	Author   []CSLName `yaml:"author,omitempty"`
	Abstract string    `yaml:"abstract,omitempty"`
	Issued   *CSLDate  `yaml:"issued,omitempty"`
	DOI      string    `yaml:"DOI,omitempty"`
}

// CSLName represents a person's name in CSL format.
type CSLName struct {
	Family  string `yaml:"family,omitempty"`
	Given   string `yaml:"given,omitempty"`
	Literal string `yaml:"literal,omitempty"`
}

// CSLDate represents a date in CSL format using date-parts.
type CSLDate struct {
	DateParts [][]int `yaml:"date-parts"`
}

// FormatCSL writes search results as a CSL-YAML list to w.
func FormatCSL(out SearchOutput, w io.Writer) error {
	items := make([]CSLItem, len(out.Results))
	for i, r := range out.Results {
		items[i] = toCSLItem(r)
	}
	enc := yaml.NewEncoder(w)
	defer enc.Close()
	return enc.Encode(items)
}

// toCSLItem converts a SearchResult to a CSLItem.
func toCSLItem(r types.SearchResult) CSLItem {
	item := CSLItem{
		ID:       r.Identifier,
		Type:     "article",
		Title:    r.Title,
		Abstract: r.Abstract,
	}

	for _, a := range r.Authors {
		item.Author = append(item.Author, parseAuthorName(a))
	}

	if !r.Date.IsZero() {
		item.Issued = &CSLDate{
			DateParts: [][]int{{r.Date.Year(), int(r.Date.Month()), r.Date.Day()}},
		}
	}

	// Set DOI if the identifier looks like one.
	if strings.HasPrefix(r.Identifier, "10.") {
		item.DOI = r.Identifier
	}

	return item
}

// parseAuthorName splits a full name string into CSL family/given parts.
// It splits on the last space: everything before is given, the last token
// is family. Single-token names use the literal field.
func parseAuthorName(name string) CSLName {
	name = strings.TrimSpace(name)
	if name == "" {
		return CSLName{}
	}
	idx := strings.LastIndex(name, " ")
	if idx < 0 {
		return CSLName{Literal: name}
	}
	return CSLName{
		Given:  name[:idx],
		Family: name[idx+1:],
	}
}
