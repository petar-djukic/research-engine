// Copyright Mesh Intelligence Inc., 2026. All rights reserved.

package acquire

import "testing"

func TestClassifyPatent(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantType IdentifierType
		wantNorm string
	}{
		// Positive: granted patents (7-8 digit numbers).
		{"granted no kind code", "US7654321", TypePatent, "US7654321"},
		{"granted with kind code B2", "US7654321B2", TypePatent, "US7654321B2"},
		{"granted with kind code B1", "US7654321B1", TypePatent, "US7654321B1"},

		// Positive: application publications (11-digit numbers).
		{"application with kind code", "US20230012345A1", TypePatent, "US20230012345A1"},
		{"application no kind code", "US20230012345", TypePatent, "US20230012345"},

		// Negative: too short, too long, or malformed.
		{"US alone", "US", TypeUnknown, "US"},
		{"US too short 1 digit", "US1", TypeUnknown, "US1"},
		{"US too short 5 digits", "US12345", TypeUnknown, "US12345"},
		{"US alpha suffix only", "USA", TypeUnknown, "USA"},
		{"US all alpha", "USABC", TypeUnknown, "USABC"},
		{"non-patent string", "hello-world", TypeUnknown, "hello-world"},
		{"empty string", "", TypeUnknown, ""},
		{"just digits no US prefix", "7654321", TypeUnknown, "7654321"},

		// Whitespace handling.
		{"patent with whitespace", "  US7654321B2  ", TypePatent, "US7654321B2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotNorm := Classify(tt.input)
			if gotType != tt.wantType {
				t.Errorf("Classify(%q) type = %v, want %v", tt.input, gotType, tt.wantType)
			}
			if gotNorm != tt.wantNorm {
				t.Errorf("Classify(%q) norm = %q, want %q", tt.input, gotNorm, tt.wantNorm)
			}
		})
	}
}

func TestSlugPatent(t *testing.T) {
	tests := []struct {
		name     string
		norm     string
		wantSlug string
	}{
		{"granted patent", "US7654321", "US7654321"},
		{"granted with kind code", "US7654321B2", "US7654321B2"},
		{"application patent", "US20230012345A1", "US20230012345A1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Slug(TypePatent, tt.norm)
			if got != tt.wantSlug {
				t.Errorf("Slug(TypePatent, %q) = %q, want %q", tt.norm, got, tt.wantSlug)
			}
		})
	}
}

func TestPDFURLPatent(t *testing.T) {
	tests := []struct {
		name    string
		norm    string
		wantURL string
	}{
		{"granted patent", "US7654321", googlePatentsPDFBase + "US7654321.pdf"},
		{"granted with kind code", "US7654321B2", googlePatentsPDFBase + "US7654321B2.pdf"},
		{"application patent", "US20230012345A1", googlePatentsPDFBase + "US20230012345A1.pdf"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PDFURL(TypePatent, tt.norm)
			if got != tt.wantURL {
				t.Errorf("PDFURL(TypePatent, %q) = %q, want %q", tt.norm, got, tt.wantURL)
			}
		})
	}
}
