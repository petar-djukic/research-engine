// Copyright Mesh Intelligence Inc., 2026. All rights reserved.

package acquire

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pdiddy/research-engine/pkg/types"
)

const sampleOpenAlexOA = `{
  "id": "https://openalex.org/W1234567890",
  "doi": "https://doi.org/10.1145/1234567.1234568",
  "best_oa_location": {
    "pdf_url": "https://example.com/oa-paper.pdf",
    "landing_page_url": "https://example.com/paper-landing"
  }
}`

const sampleOpenAlexNoOA = `{
  "id": "https://openalex.org/W9999999999",
  "doi": "https://doi.org/10.1145/9999999",
  "best_oa_location": null
}`

const sampleOpenAlexNoPDF = `{
  "id": "https://openalex.org/W1111111111",
  "doi": "https://doi.org/10.1145/1111111",
  "best_oa_location": {
    "pdf_url": "",
    "landing_page_url": "https://example.com/landing-only"
  }
}`

func TestResolveOpenAlex(t *testing.T) {
	tests := []struct {
		name       string
		doi        string
		response   string
		statusCode int
		wantURL    string
		wantErr    bool
	}{
		{
			name:       "OA PDF available",
			doi:        "10.1145/1234567.1234568",
			response:   sampleOpenAlexOA,
			statusCode: http.StatusOK,
			wantURL:    "https://example.com/oa-paper.pdf",
		},
		{
			name:       "no OA location",
			doi:        "10.1145/9999999",
			response:   sampleOpenAlexNoOA,
			statusCode: http.StatusOK,
			wantURL:    "",
		},
		{
			name:       "OA location but no PDF URL",
			doi:        "10.1145/1111111",
			response:   sampleOpenAlexNoPDF,
			statusCode: http.StatusOK,
			wantURL:    "",
		},
		{
			name:       "API returns 404",
			doi:        "10.1145/nonexistent",
			response:   `{"error": "not found"}`,
			statusCode: http.StatusNotFound,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				fmt.Fprint(w, tt.response)
			}))
			defer ts.Close()

			origBase := openAlexAPIBase
			openAlexAPIBase = ts.URL + "/"
			defer func() { openAlexAPIBase = origBase }()

			cfg := types.AcquisitionConfig{
				HTTPConfig: types.HTTPConfig{
					Timeout:   10 * time.Second,
					UserAgent: "research-engine-test/0.1",
				},
			}

			got, err := resolveOpenAlex(ts.Client(), tt.doi, cfg)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveOpenAlex: %v", err)
			}
			if got != tt.wantURL {
				t.Errorf("resolveOpenAlex() = %q, want %q", got, tt.wantURL)
			}
		})
	}
}

func TestResolveOpenAlexNetworkError(t *testing.T) {
	origBase := openAlexAPIBase
	openAlexAPIBase = "http://127.0.0.1:1/"
	defer func() { openAlexAPIBase = origBase }()

	cfg := types.AcquisitionConfig{
		HTTPConfig: types.HTTPConfig{
			Timeout:   1 * time.Second,
			UserAgent: "research-engine-test/0.1",
		},
	}

	_, err := resolveOpenAlex(http.DefaultClient, "10.1145/1234567", cfg)
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}
