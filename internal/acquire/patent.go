// Copyright Mesh Intelligence Inc., 2026. All rights reserved.

// Implements: prd008-patent-search (R4.4-R4.6);
//
//	docs/ARCHITECTURE § Acquisition.
package acquire

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/pdiddy/research-engine/pkg/types"
)

// patentsViewAPIBase is the PatentsView patent API endpoint. Declared as a var
// so tests can substitute an httptest server.
var patentsViewAPIBase = "https://search.patentsview.org/api/v1/patent/"

// googlePatentsHTMLBase is the Google Patents page base URL for PDF fallback (R4.4).
var googlePatentsHTMLBase = "https://patents.google.com/patent/"

// patentNumOnlyPattern matches the leading digits of a patent identifier,
// stripping the kind code suffix (e.g., "7654321B2" -> "7654321").
var patentNumOnlyPattern = regexp.MustCompile(`^\d+`)

// PatentsView API JSON structures for metadata retrieval.
type pvMetadataResponse struct {
	Patents []pvMetadataPatent `json:"patents"`
}

type pvMetadataPatent struct {
	PatentTitle    string               `json:"patent_title"`
	PatentAbstract string               `json:"patent_abstract"`
	PatentDate     string               `json:"patent_date"`
	Inventors      []pvMetadataInventor `json:"inventors"`
}

type pvMetadataInventor struct {
	InventorNameLast string `json:"inventor_name_last"`
}

// fetchPatentMetadata retrieves metadata from the PatentsView API (prd008 R4.6).
// If the API call fails (e.g. missing key, rate limit), the caller logs a
// warning and leaves metadata fields empty.
func fetchPatentMetadata(client *http.Client, patentID string, paper *types.Paper, cfg types.AcquisitionConfig) error {
	// Strip "US" prefix and kind code for the API query.
	rawNum := strings.TrimPrefix(patentID, "US")
	queryID := stripKindCode(rawNum)

	q := fmt.Sprintf(`{"patent_id":"%s"}`, queryID)
	fields := `["patent_title","patent_abstract","patent_date","inventors.inventor_name_last"]`

	params := url.Values{
		"q": {q},
		"f": {fields},
	}
	apiURL := patentsViewAPIBase + "?" + params.Encode()

	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", cfg.UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("PatentsView API request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("PatentsView API returned HTTP %d", resp.StatusCode)
	}

	var pvr pvMetadataResponse
	if err := json.NewDecoder(resp.Body).Decode(&pvr); err != nil {
		return fmt.Errorf("parsing PatentsView response: %w", err)
	}

	if len(pvr.Patents) == 0 {
		return fmt.Errorf("no patent found for ID %s", patentID)
	}

	patent := pvr.Patents[0]
	paper.Title = patent.PatentTitle
	paper.Abstract = patent.PatentAbstract

	for _, inv := range patent.Inventors {
		if inv.InventorNameLast != "" {
			paper.Authors = append(paper.Authors, inv.InventorNameLast)
		}
	}

	if patent.PatentDate != "" {
		if t, parseErr := time.Parse("2006-01-02", patent.PatentDate); parseErr == nil {
			paper.Date = t
		}
	}

	return nil
}

// stripKindCode removes the kind code suffix from a patent number
// (e.g., "7654321B2" -> "7654321", "20230012345A1" -> "20230012345").
func stripKindCode(id string) string {
	m := patentNumOnlyPattern.FindString(id)
	if m != "" {
		return m
	}
	return id
}
