// Copyright Mesh Intelligence Inc., 2026. All rights reserved.

package types

import "time"

// ConversionStatus indicates the state of PDF-to-Markdown conversion for a paper.
// Per prd002-conversion R4.4.
type ConversionStatus string

const (
	ConversionNone    ConversionStatus = "none"
	ConversionDone    ConversionStatus = "converted"
	ConversionPartial ConversionStatus = "partial"
	ConversionFailed  ConversionStatus = "failed"
)

// Paper holds metadata and file paths for an acquired paper.
// Per prd001-acquisition R3.2: source URL, local PDF path, title, authors,
// date, abstract, and conversion status.
type Paper struct {
	// ID is a slug derived from the paper identifier (e.g. "2301.07041").
	ID string `json:"id" yaml:"id"`

	// SourceURL is the URL from which the paper was downloaded.
	SourceURL string `json:"source_url" yaml:"source_url"`

	// PDFPath is the local filesystem path to the downloaded PDF.
	PDFPath string `json:"pdf_path" yaml:"pdf_path"`

	// Title is the paper title.
	Title string `json:"title" yaml:"title"`

	// Authors lists the paper authors in source order.
	Authors []string `json:"authors" yaml:"authors"`

	// Date is the publication or preprint date.
	Date time.Time `json:"date" yaml:"date"`

	// Abstract is the paper abstract.
	Abstract string `json:"abstract" yaml:"abstract"`

	// Source identifies which backend provided the PDF (e.g. "arxiv", "doi", "openalex", "url").
	Source string `json:"source,omitempty" yaml:"source,omitempty"`

	// ConversionStatus tracks whether the PDF has been converted to Markdown.
	ConversionStatus ConversionStatus `json:"conversion_status" yaml:"conversion_status"`
}
