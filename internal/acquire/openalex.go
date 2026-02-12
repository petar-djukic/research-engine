// Copyright Mesh Intelligence Inc., 2026. All rights reserved.

package acquire

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/pdiddy/research-engine/pkg/types"
)

// openAlexAPIBase is the OpenAlex works endpoint. Declared as a var so tests
// can substitute an httptest server.
var openAlexAPIBase = "https://api.openalex.org/works/"

// openAlexResponse captures the fields we need from an OpenAlex work record.
type openAlexResponse struct {
	BestOALocation *openAlexLocation `json:"best_oa_location"`
}

// openAlexLocation represents an open-access location in the OpenAlex response.
type openAlexLocation struct {
	PDFURL     string `json:"pdf_url"`
	LandingURL string `json:"landing_page_url"`
}

// resolveOpenAlex queries the OpenAlex API for a DOI and returns the
// open-access PDF URL if one exists. It returns an empty string when the
// paper is not available or has no open-access PDF.
func resolveOpenAlex(client *http.Client, doi string, cfg types.AcquisitionConfig) (string, error) {
	apiURL := openAlexAPIBase + "https://doi.org/" + doi
	if cfg.UserAgent != "" {
		apiURL += "?mailto=" + cfg.UserAgent
	}

	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("creating OpenAlex request: %w", err)
	}
	req.Header.Set("User-Agent", cfg.UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("OpenAlex API request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("OpenAlex API returned HTTP %d", resp.StatusCode)
	}

	var oa openAlexResponse
	if err := json.NewDecoder(resp.Body).Decode(&oa); err != nil {
		return "", fmt.Errorf("parsing OpenAlex response: %w", err)
	}

	if oa.BestOALocation == nil {
		return "", nil
	}
	if oa.BestOALocation.PDFURL != "" {
		return oa.BestOALocation.PDFURL, nil
	}
	return "", nil
}
