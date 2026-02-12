// Copyright Mesh Intelligence Inc., 2026. All rights reserved.

// Package types defines shared data structures for the research-engine pipeline.
// Implements: prd006-search (SearchResult, R4.1, R4.4);
//
//	prd001-acquisition (Paper, R3.2);
//	prd003-extraction (KnowledgeItem, R1.1-R1.4, R2.1-R2.5, R4.1-R4.4);
//	prd005-generation (Draft, R3.2-R3.5);
//	prd002-conversion (ConversionStatus).
//
// See docs/ARCHITECTURE.md § Pipeline Interface, § Data Structures.
package types

import "time"

// SearchResult represents a candidate paper returned by an academic API query.
// Per prd006-search R4.1, each result carries an identifier, metadata, source,
// relevance score, and a preferred acquisition identifier (R4.4).
type SearchResult struct {
	// Identifier is the canonical ID from the source (arXiv ID, DOI, or URL).
	Identifier string `json:"identifier" yaml:"identifier"`

	// Title is the paper title as returned by the source.
	Title string `json:"title" yaml:"title"`

	// Authors lists the paper authors in source order.
	Authors []string `json:"authors" yaml:"authors"`

	// Abstract is the paper abstract or summary.
	Abstract string `json:"abstract" yaml:"abstract"`

	// Date is the publication or preprint date.
	Date time.Time `json:"date" yaml:"date"`

	// Source identifies which backend found this result (e.g. "arxiv", "semantic_scholar").
	Source string `json:"source" yaml:"source"`

	// RelevanceScore is a value between 0.0 and 1.0 indicating relevance to the query.
	RelevanceScore float64 `json:"relevance_score" yaml:"relevance_score"`

	// PreferredAcquisitionID is the identifier the acquisition stage should use
	// to download this paper: arXiv ID if available, then DOI, then URL.
	PreferredAcquisitionID string `json:"preferred_acquisition_id" yaml:"preferred_acquisition_id"`
}
