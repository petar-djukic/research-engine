// Copyright Mesh Intelligence Inc., 2026. All rights reserved.

package types

// Author identifies a paper author or contributor. Per prd007-paper-writing R2.1.
type Author struct {
	// Name is the author's display name.
	Name string `json:"name" yaml:"name"`

	// Affiliation is the author's institutional affiliation.
	Affiliation string `json:"affiliation,omitempty" yaml:"affiliation,omitempty"`
}

// TitlePageMeta holds the YAML frontmatter from 00-title-page.md.
// Per prd007-paper-writing R2.1-R2.3.
type TitlePageMeta struct {
	// Title is the paper title.
	Title string `json:"title" yaml:"title"`

	// Authors lists the paper's authors.
	Authors []Author `json:"authors" yaml:"authors"`

	// Date is the creation date in YYYY-MM-DD format.
	Date string `json:"date" yaml:"date"`

	// Type classifies the paper: survey, literature-review, original-research, position-paper.
	Type string `json:"type" yaml:"type"`

	// Abstract summarizes the paper. Updated as the paper develops (R2.3).
	Abstract string `json:"abstract" yaml:"abstract"`

	// Keywords lists topic keywords for the paper.
	Keywords []string `json:"keywords" yaml:"keywords"`
}

// SectionStatus tracks a section's progress through the writing workflow.
// Per prd007-paper-writing R4.1.
type SectionStatus string

const (
	StatusOutline SectionStatus = "outline"
	StatusDraft   SectionStatus = "draft"
	StatusRevised SectionStatus = "revised"
	StatusFinal   SectionStatus = "final"
)

// OutlineSection describes one section in a paper project's outline.
// Per prd007-paper-writing R4.1.
type OutlineSection struct {
	// Number is the two-digit sequence number (e.g. "01", "02").
	Number string `json:"number" yaml:"number"`

	// Title is the section heading.
	Title string `json:"title" yaml:"title"`

	// File is the section's filename (e.g. "01-introduction.md").
	File string `json:"file" yaml:"file"`

	// Description explains what the section covers.
	Description string `json:"description" yaml:"description"`

	// Status tracks writing progress: outline, draft, revised, final.
	Status SectionStatus `json:"status" yaml:"status"`
}

// Outline holds the paper structure from outline.yaml.
// Per prd007-paper-writing R4.1-R4.3.
type Outline struct {
	// Sections lists the paper's sections in order.
	Sections []OutlineSection `json:"sections" yaml:"sections"`
}

// ReferenceEntry records a cited paper in references.yaml.
// Per prd007-paper-writing R5.1.
type ReferenceEntry struct {
	// CitationKey is the inline citation label (e.g. "Vaswani2017").
	CitationKey string `json:"citation_key" yaml:"citation_key"`

	// PaperID is the acquisition slug linking to papers/.
	PaperID string `json:"paper_id" yaml:"paper_id"`

	// Title is the cited paper's title.
	Title string `json:"title" yaml:"title"`

	// Authors lists author surnames.
	Authors []string `json:"authors" yaml:"authors"`

	// Year is the publication year.
	Year int `json:"year" yaml:"year"`

	// Venue is the journal or conference (optional).
	Venue string `json:"venue,omitempty" yaml:"venue,omitempty"`
}

// ReferencesFile holds all cited papers from references.yaml.
// Per prd007-paper-writing R5.1-R5.4.
type ReferencesFile struct {
	// Papers lists every cited paper.
	Papers []ReferenceEntry `json:"papers" yaml:"papers"`
}
