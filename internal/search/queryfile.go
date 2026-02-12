// Copyright Mesh Intelligence Inc., 2026. All rights reserved.

package search

import (
	"fmt"
	"os"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/pdiddy/research-engine/pkg/types"
)

// QueryFile is the on-disk representation of a search query and its results.
// The researcher can save a search to a file and reload it later without
// re-querying APIs.
// Implements: prd006-search R1.6, R4.6.
type QueryFile struct {
	Query   QueryParams          `yaml:"query"`
	Config  QueryFileConfig      `yaml:"config"`
	Results []types.SearchResult `yaml:"results"`
	Summary QuerySummary         `yaml:"summary"`
}

// QueryParams stores the query parameters in a serializable form.
type QueryParams struct {
	FreeText string   `yaml:"free_text,omitempty"`
	Author   string   `yaml:"author,omitempty"`
	Keywords []string `yaml:"keywords,omitempty"`
	DateFrom string   `yaml:"date_from,omitempty"`
	DateTo   string   `yaml:"date_to,omitempty"`
}

// QueryFileConfig stores the search configuration that produced the results.
type QueryFileConfig struct {
	MaxResults  int  `yaml:"max_results"`
	RecencyBias bool `yaml:"recency_bias"`
}

// QuerySummary stores result statistics and a timestamp.
type QuerySummary struct {
	Total           int       `yaml:"total"`
	DuplicatesRemoved int     `yaml:"duplicates_removed"`
	BackendErrors   []string  `yaml:"backend_errors,omitempty"`
	Timestamp       time.Time `yaml:"timestamp"`
}

const dateFmt = "2006-01-02"

// WriteQueryFile saves query parameters and results to a YAML file.
func WriteQueryFile(path string, query Query, cfg types.SearchConfig, recencyBias bool, out SearchOutput) error {
	qf := QueryFile{
		Query: QueryParams{
			FreeText: query.FreeText,
			Author:   query.Author,
			Keywords: query.Keywords,
		},
		Config: QueryFileConfig{
			MaxResults:  cfg.MaxResults,
			RecencyBias: recencyBias,
		},
		Results: out.Results,
		Summary: QuerySummary{
			Total:             len(out.Results),
			DuplicatesRemoved: out.DupsRemoved,
			BackendErrors:     out.BackendErrors,
			Timestamp:         time.Now(),
		},
	}

	if !query.DateFrom.IsZero() {
		qf.Query.DateFrom = query.DateFrom.Format(dateFmt)
	}
	if !query.DateTo.IsZero() {
		qf.Query.DateTo = query.DateTo.Format(dateFmt)
	}

	data, err := yaml.Marshal(&qf)
	if err != nil {
		return fmt.Errorf("marshaling query file: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

// ReadQueryFile loads a previously saved query file from disk.
func ReadQueryFile(path string) (*QueryFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading query file: %w", err)
	}
	var qf QueryFile
	if err := yaml.Unmarshal(data, &qf); err != nil {
		return nil, fmt.Errorf("parsing query file: %w", err)
	}
	return &qf, nil
}

// ToQuery converts stored QueryParams back into a Query struct.
func (p QueryParams) ToQuery() (Query, error) {
	q := Query{
		FreeText: p.FreeText,
		Author:   p.Author,
		Keywords: p.Keywords,
	}
	if p.DateFrom != "" {
		t, err := time.Parse(dateFmt, p.DateFrom)
		if err != nil {
			return q, fmt.Errorf("invalid date_from %q: %w", p.DateFrom, err)
		}
		q.DateFrom = t
	}
	if p.DateTo != "" {
		t, err := time.Parse(dateFmt, p.DateTo)
		if err != nil {
			return q, fmt.Errorf("invalid date_to %q: %w", p.DateTo, err)
		}
		q.DateTo = t
	}
	return q, nil
}
