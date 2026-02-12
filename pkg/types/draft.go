// Copyright Mesh Intelligence Inc., 2026. All rights reserved.

package types

// CitationEntry maps an inline citation in a generated draft to its source
// KnowledgeItem and paper. Per prd005-generation R4.2.
type CitationEntry struct {
	// CitationKey is the inline citation label in the draft text (e.g. "[Smith2020]").
	CitationKey string `json:"citation_key" yaml:"citation_key"`

	// ItemID is the KnowledgeItem ID that supports this citation.
	ItemID string `json:"item_id" yaml:"item_id"`

	// PaperID is the source paper's identifier.
	PaperID string `json:"paper_id" yaml:"paper_id"`

	// Section is the section heading in the source paper.
	Section string `json:"section" yaml:"section"`

	// Page is the page number in the source paper.
	Page int `json:"page" yaml:"page"`
}

// DraftSection represents one section in a generated draft document.
// Per prd005-generation R2.3-R2.4.
type DraftSection struct {
	// Heading is the section title.
	Heading string `json:"heading" yaml:"heading"`

	// Content is the generated Markdown prose for this section.
	Content string `json:"content" yaml:"content"`
}

// Draft represents a generated document with inline citations linking claims
// to KnowledgeItems and their source papers.
// Per prd005-generation R3.2-R3.5, R4.1-R4.4.
type Draft struct {
	// Title is the draft document title.
	Title string `json:"title" yaml:"title"`

	// Query is the research question or topic that prompted generation.
	Query string `json:"query" yaml:"query"`

	// Sections contains the generated content organized by section.
	Sections []DraftSection `json:"sections" yaml:"sections"`

	// Citations maps every inline citation to its source KnowledgeItem and paper.
	Citations []CitationEntry `json:"citations" yaml:"citations"`

	// References lists the papers cited in this draft. Per R3.4.
	References []Paper `json:"references" yaml:"references"`

	// OutputPath is the file path where the draft was written.
	OutputPath string `json:"output_path" yaml:"output_path"`
}
