// Copyright Mesh Intelligence Inc., 2026. All rights reserved.

package acquire

import (
	"crypto/sha256"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
)

// IdentifierType classifies an input identifier.
type IdentifierType int

const (
	TypeUnknown IdentifierType = iota
	TypeArxiv
	TypeDOI
	TypeURL
	TypePatent
)

func (t IdentifierType) String() string {
	switch t {
	case TypeArxiv:
		return "arxiv"
	case TypeDOI:
		return "doi"
	case TypeURL:
		return "url"
	case TypePatent:
		return "patent"
	default:
		return "unknown"
	}
}

// Base URLs for identifier resolution. Declared as vars so tests can
// substitute httptest servers.
var (
	arxivPDFBase       = "https://arxiv.org/pdf/"
	arxivAPIBase       = "https://export.arxiv.org/api/query"
	doiBase            = "https://doi.org/"
	crossrefAPIBase    = "https://api.crossref.org/works/"
	googlePatentsPDFBase = "https://patentimages.storage.googleapis.com/pdfs/"
)

// arxivPattern matches arXiv IDs: "2301.07041", "arXiv:2301.07041", "2301.07041v2".
var arxivPattern = regexp.MustCompile(`^(?:arXiv:)?(\d{4}\.\d{4,5}(?:v\d+)?)$`)

// doiPattern matches DOIs: "10.1145/1234567.1234568".
var doiPattern = regexp.MustCompile(`^10\.\d{4,9}/[^\s]+$`)

// patentPattern matches US patent identifiers: "US7654321", "US7654321B2",
// "US20230012345A1". Captures the full number including optional kind code.
var patentPattern = regexp.MustCompile(`^US(\d{6,11}[A-Z]\d{0,2})$|^US(\d{6,11})$`)

// Classify determines the identifier type and returns the normalized form.
// For arXiv, it strips the optional "arXiv:" prefix.
func Classify(identifier string) (IdentifierType, string) {
	identifier = strings.TrimSpace(identifier)

	if m := arxivPattern.FindStringSubmatch(identifier); m != nil {
		return TypeArxiv, m[1]
	}

	if doiPattern.MatchString(identifier) {
		return TypeDOI, identifier
	}

	if m := patentPattern.FindStringSubmatch(identifier); m != nil {
		// m[1] is the branch with kind code, m[2] is the branch without.
		num := m[1]
		if num == "" {
			num = m[2]
		}
		return TypePatent, "US" + num
	}

	if u, err := url.Parse(identifier); err == nil && (u.Scheme == "http" || u.Scheme == "https") {
		return TypeURL, identifier
	}

	return TypeUnknown, identifier
}

// Slug returns a filesystem-safe filename stem for the identifier.
func Slug(idType IdentifierType, normalized string) string {
	switch idType {
	case TypeArxiv:
		return normalized
	case TypeDOI:
		return strings.NewReplacer("/", "-", ":", "-").Replace(normalized)
	case TypeURL:
		u, err := url.Parse(normalized)
		if err != nil {
			return urlHashSlug(normalized)
		}
		base := strings.TrimSuffix(filepath.Base(u.Path), filepath.Ext(u.Path))
		if base == "" || base == "." || base == "/" {
			return urlHashSlug(normalized)
		}
		return base
	case TypePatent:
		return normalized
	default:
		return "unknown"
	}
}

// PDFURL returns the download URL for the identifier. For arXiv, this is
// the arxiv.org PDF endpoint. For DOI, this is the doi.org resolver
// (the HTTP client follows redirects). For direct URLs, it returns as-is.
func PDFURL(idType IdentifierType, normalized string) string {
	switch idType {
	case TypeArxiv:
		return arxivPDFBase + normalized
	case TypeDOI:
		return doiBase + normalized
	case TypeURL:
		return normalized
	case TypePatent:
		return googlePatentsPDFBase + normalized + ".pdf"
	default:
		return ""
	}
}

func urlHashSlug(rawURL string) string {
	h := sha256.Sum256([]byte(rawURL))
	return fmt.Sprintf("url-%x", h[:8])
}
