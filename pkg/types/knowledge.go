// Copyright Mesh Intelligence Inc., 2026. All rights reserved.

package types

// KnowledgeItemType categorizes a knowledge item extracted from a paper.
// Per prd003-extraction R1.1.
type KnowledgeItemType string

const (
	ItemClaim      KnowledgeItemType = "claim"
	ItemMethod     KnowledgeItemType = "method"
	ItemDefinition KnowledgeItemType = "definition"
	ItemResult     KnowledgeItemType = "result"
)

// BibliographyEntry represents a parsed entry from a paper's reference section.
// Per prd003-extraction R3.2.
type BibliographyEntry struct {
	// Key is the reference label as it appears in the paper (e.g. "1", "Smith2020").
	Key string `json:"key" yaml:"key"`

	// Authors lists the cited work's authors.
	Authors []string `json:"authors" yaml:"authors"`

	// Title is the cited work's title.
	Title string `json:"title" yaml:"title"`

	// Year is the publication year.
	Year string `json:"year" yaml:"year"`

	// Venue is the journal, conference, or publisher.
	Venue string `json:"venue" yaml:"venue"`
}

// Citation represents an inline reference within a KnowledgeItem's content,
// linking it to a BibliographyEntry. Per prd003-extraction R3.1, R3.3.
type Citation struct {
	// Key is the inline reference identifier as it appears in the text
	// (e.g. "[1]", "[Smith et al., 2020]").
	Key string `json:"key" yaml:"key"`

	// BibIndex is the zero-based index into the ExtractionResult.Bibliography
	// slice for the matching bibliography entry. A value of -1 indicates
	// no matching entry was found.
	BibIndex int `json:"bib_index" yaml:"bib_index"`

	// Context is the surrounding text where the citation appears.
	Context string `json:"context" yaml:"context"`
}

// KnowledgeItem is a typed extraction from a paper with provenance.
// Per prd003-extraction R1.1-R1.4, R2.1-R2.5, R3.1, R3.3-R3.4, R4.1-R4.4.
type KnowledgeItem struct {
	// ID is a stable identifier for this item, consistent across re-extractions
	// of unchanged content. Per R2.5.
	ID string `json:"id" yaml:"id"`

	// Type categorizes the item: claim, method, definition, or result.
	Type KnowledgeItemType `json:"type" yaml:"type"`

	// Content preserves the original language from the source paper. Per R1.3.
	Content string `json:"content" yaml:"content"`

	// PaperID matches the Paper record from acquisition. Per R2.1.
	PaperID string `json:"paper_id" yaml:"paper_id"`

	// Section is the heading under which the item was found. Per R2.2.
	Section string `json:"section" yaml:"section"`

	// Page is the page number where the item begins. Per R2.3, R2.4.
	Page int `json:"page" yaml:"page"`

	// Confidence is a float between 0.0 and 1.0 indicating extraction certainty. Per R1.4.
	Confidence float64 `json:"confidence" yaml:"confidence"`

	// Tags are lowercase, hyphenated topic labels drawn from the paper vocabulary. Per R4.1-R4.4.
	Tags []string `json:"tags" yaml:"tags"`

	// Citations lists inline references cited within this item's content. Per R3.1, R3.3, R3.4.
	Citations []Citation `json:"citations,omitempty" yaml:"citations,omitempty"`
}

// ExtractionResult holds the output of extracting knowledge from a single paper.
// Per prd003-extraction R5.6, R3.2, R4.3.
type ExtractionResult struct {
	// PaperID identifies the source paper.
	PaperID string `json:"paper_id" yaml:"paper_id"`

	// Items contains the extracted knowledge items.
	Items []KnowledgeItem `json:"items" yaml:"items"`

	// Bibliography contains the parsed reference entries from the paper.
	Bibliography []BibliographyEntry `json:"bibliography" yaml:"bibliography"`

	// PaperTags are paper-level topic tags summarizing the overall topics. Per R4.3.
	PaperTags []string `json:"paper_tags" yaml:"paper_tags"`

	// Error records an extraction failure message. Empty on success.
	Error string `json:"error,omitempty" yaml:"error,omitempty"`
}
