// Copyright Mesh Intelligence Inc., 2026. All rights reserved.

package draft

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pdiddy/research-engine/pkg/types"
)

// writeFile is a test helper that creates a file with the given content.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadOutline(t *testing.T) {
	tests := []struct {
		name      string
		yaml      string
		wantCount int
		wantErr   bool
	}{
		{
			name: "valid outline",
			yaml: `sections:
  - number: "01"
    title: Introduction
    file: 01-introduction.md
    description: "Motivates the survey."
    status: draft
  - number: "02"
    title: Related Work
    file: 02-related-work.md
    description: "Reviews prior work."
    status: outline
`,
			wantCount: 2,
		},
		{
			name:      "empty sections",
			yaml:      "sections: []\n",
			wantCount: 0,
		},
		{
			name:    "invalid yaml",
			yaml:    ":::bad\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, dir, "outline.yaml", tt.yaml)

			outline, err := LoadOutline(dir)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(outline.Sections) != tt.wantCount {
				t.Errorf("len(Sections) = %d, want %d", len(outline.Sections), tt.wantCount)
			}
		})
	}
}

func TestLoadOutlineMissingFile(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadOutline(dir)
	if err == nil {
		t.Error("expected error for missing outline.yaml")
	}
}

func TestLoadOutlineFields(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "outline.yaml", `sections:
  - number: "03"
    title: Methods
    file: 03-methods.md
    description: "Describes the methodology."
    status: revised
`)
	outline, err := LoadOutline(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := outline.Sections[0]
	if s.Number != "03" {
		t.Errorf("Number = %q, want %q", s.Number, "03")
	}
	if s.Title != "Methods" {
		t.Errorf("Title = %q, want %q", s.Title, "Methods")
	}
	if s.File != "03-methods.md" {
		t.Errorf("File = %q, want %q", s.File, "03-methods.md")
	}
	if s.Status != types.StatusRevised {
		t.Errorf("Status = %q, want %q", s.Status, types.StatusRevised)
	}
}

func TestLoadReferences(t *testing.T) {
	tests := []struct {
		name      string
		yaml      string
		wantCount int
		wantErr   bool
	}{
		{
			name: "valid references",
			yaml: `papers:
  - citation_key: Vaswani2017
    paper_id: attention-is-all-you-need
    title: "Attention Is All You Need"
    authors:
      - Vaswani
      - Shazeer
    year: 2017
    venue: NeurIPS
  - citation_key: Brown2020
    paper_id: gpt3
    title: "Language Models are Few-Shot Learners"
    authors:
      - Brown
    year: 2020
`,
			wantCount: 2,
		},
		{
			name:      "empty papers",
			yaml:      "papers: []\n",
			wantCount: 0,
		},
		{
			name:    "invalid yaml",
			yaml:    "{{{bad",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, dir, "references.yaml", tt.yaml)

			refs, err := LoadReferences(dir)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(refs.Papers) != tt.wantCount {
				t.Errorf("len(Papers) = %d, want %d", len(refs.Papers), tt.wantCount)
			}
		})
	}
}

func TestLoadReferencesMissingFile(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadReferences(dir)
	if err == nil {
		t.Error("expected error for missing references.yaml")
	}
}

func TestLoadReferencesFields(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "references.yaml", `papers:
  - citation_key: Vaswani2017
    paper_id: attention-is-all-you-need
    title: "Attention Is All You Need"
    authors:
      - Vaswani
      - Shazeer
    year: 2017
    venue: NeurIPS
`)
	refs, err := LoadReferences(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r := refs.Papers[0]
	if r.CitationKey != "Vaswani2017" {
		t.Errorf("CitationKey = %q, want %q", r.CitationKey, "Vaswani2017")
	}
	if r.PaperID != "attention-is-all-you-need" {
		t.Errorf("PaperID = %q, want %q", r.PaperID, "attention-is-all-you-need")
	}
	if r.Year != 2017 {
		t.Errorf("Year = %d, want %d", r.Year, 2017)
	}
	if r.Venue != "NeurIPS" {
		t.Errorf("Venue = %q, want %q", r.Venue, "NeurIPS")
	}
	if len(r.Authors) != 2 {
		t.Errorf("len(Authors) = %d, want %d", len(r.Authors), 2)
	}
}

func TestSectionFiles(t *testing.T) {
	tests := []struct {
		name      string
		files     []string
		wantFiles []string
	}{
		{
			name:      "ordered sections",
			files:     []string{"02-related-work.md", "01-introduction.md", "03-methods.md"},
			wantFiles: []string{"01-introduction.md", "02-related-work.md", "03-methods.md"},
		},
		{
			name:      "excludes non-md",
			files:     []string{"01-intro.md", "outline.yaml", "references.yaml", "README.txt"},
			wantFiles: []string{"01-intro.md"},
		},
		{
			name:      "excludes non-numbered",
			files:     []string{"01-intro.md", "notes.md", "ab-draft.md"},
			wantFiles: []string{"01-intro.md"},
		},
		{
			name:      "empty directory",
			files:     []string{},
			wantFiles: nil,
		},
		{
			name:      "title page included",
			files:     []string{"00-title-page.md", "01-introduction.md"},
			wantFiles: []string{"00-title-page.md", "01-introduction.md"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, f := range tt.files {
				writeFile(t, dir, f, "content")
			}

			files, err := SectionFiles(dir)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Extract basenames for comparison.
			var basenames []string
			for _, f := range files {
				basenames = append(basenames, filepath.Base(f))
			}

			if len(basenames) != len(tt.wantFiles) {
				t.Fatalf("got %d files, want %d: %v", len(basenames), len(tt.wantFiles), basenames)
			}
			for i, want := range tt.wantFiles {
				if basenames[i] != want {
					t.Errorf("file[%d] = %q, want %q", i, basenames[i], want)
				}
			}
		})
	}
}

func TestValidateCitations(t *testing.T) {
	tests := []struct {
		name        string
		refs        string
		sections    map[string]string
		wantMissing []string
	}{
		{
			name: "all keys present",
			refs: `papers:
  - citation_key: Vaswani2017
    paper_id: attn
    title: "Attention"
    authors: [Vaswani]
    year: 2017
`,
			sections: map[string]string{
				"01-intro.md": "Transformers work well [Vaswani2017].",
			},
			wantMissing: nil,
		},
		{
			name: "missing key",
			refs: `papers:
  - citation_key: Vaswani2017
    paper_id: attn
    title: "Attention"
    authors: [Vaswani]
    year: 2017
`,
			sections: map[string]string{
				"01-intro.md": "Results from [Vaswani2017] and [Brown2020] show progress.",
			},
			wantMissing: []string{"Brown2020"},
		},
		{
			name: "multi-citation bracket",
			refs: `papers:
  - citation_key: Vaswani2017
    paper_id: attn
    title: "Attention"
    authors: [Vaswani]
    year: 2017
`,
			sections: map[string]string{
				"01-intro.md": "Recent advances [Vaswani2017; Tay2022] are notable.",
			},
			wantMissing: []string{"Tay2022"},
		},
		{
			name: "no citations",
			refs: `papers:
  - citation_key: Vaswani2017
    paper_id: attn
    title: "Attention"
    authors: [Vaswani]
    year: 2017
`,
			sections: map[string]string{
				"01-intro.md": "This section has no citations.",
			},
			wantMissing: nil,
		},
		{
			name: "markdown links ignored",
			refs: `papers: []`,
			sections: map[string]string{
				"01-intro.md": "See [this link](https://example.com) and ![image](fig.png).",
			},
			wantMissing: nil,
		},
		{
			name: "deduplicates missing keys across files",
			refs: `papers: []`,
			sections: map[string]string{
				"01-intro.md":   "Work by [Smith2020].",
				"02-methods.md": "Following [Smith2020] we implement...",
			},
			wantMissing: []string{"Smith2020"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, dir, "references.yaml", tt.refs)
			for name, content := range tt.sections {
				writeFile(t, dir, name, content)
			}

			missing, err := ValidateCitations(dir)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(missing) != len(tt.wantMissing) {
				t.Fatalf("missing = %v, want %v", missing, tt.wantMissing)
			}
			for i, want := range tt.wantMissing {
				if missing[i] != want {
					t.Errorf("missing[%d] = %q, want %q", i, missing[i], want)
				}
			}
		})
	}
}

func TestExtractCitationKeys(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{
			name: "single citation",
			text: "Results [Vaswani2017] show improvement.",
			want: []string{"Vaswani2017"},
		},
		{
			name: "multi-citation",
			text: "Prior work [Vaswani2017; Brown2020; Tay2022] shows...",
			want: []string{"Vaswani2017", "Brown2020", "Tay2022"},
		},
		{
			name: "markdown link not a citation",
			text: "[click here](http://example.com)",
			want: nil,
		},
		{
			name: "plain text bracket not a citation",
			text: "array[0] and map[key]",
			want: nil,
		},
		{
			name: "empty brackets",
			text: "nothing []",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractCitationKeys(tt.text)
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i, want := range tt.want {
				if got[i] != want {
					t.Errorf("got[%d] = %q, want %q", i, got[i], want)
				}
			}
		})
	}
}

func TestIsCitationKey(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"Vaswani2017", true},
		{"Brown2020", true},
		{"Smith-Jones2019", true},
		{"click here", false},
		{"http://example.com", false},
		{"", false},
		{"123", false},
		{"abc", false},
		{"0", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := isCitationKey(tt.input)
			if got != tt.want {
				t.Errorf("isCitationKey(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestGenerateBibTeX(t *testing.T) {
	tests := []struct {
		name     string
		refs     *types.ReferencesFile
		contains []string
		empty    bool
	}{
		{
			name: "single entry with venue",
			refs: &types.ReferencesFile{
				Papers: []types.ReferenceEntry{
					{
						CitationKey: "Vaswani2017",
						Title:       "Attention Is All You Need",
						Authors:     []string{"Vaswani", "Shazeer"},
						Year:        2017,
						Venue:       "NeurIPS",
					},
				},
			},
			contains: []string{
				"@article{Vaswani2017,",
				"title = {Attention Is All You Need}",
				"author = {Vaswani and Shazeer}",
				"year = {2017}",
				"journal = {NeurIPS}",
			},
		},
		{
			name: "venue omitted",
			refs: &types.ReferencesFile{
				Papers: []types.ReferenceEntry{
					{
						CitationKey: "Brown2020",
						Title:       "Language Models",
						Authors:     []string{"Brown"},
						Year:        2020,
					},
				},
			},
			contains: []string{
				"@article{Brown2020,",
				"title = {Language Models}",
			},
		},
		{
			name:  "empty references",
			refs:  &types.ReferencesFile{},
			empty: true,
		},
		{
			name: "multiple entries",
			refs: &types.ReferencesFile{
				Papers: []types.ReferenceEntry{
					{CitationKey: "A2020", Title: "First", Authors: []string{"A"}, Year: 2020},
					{CitationKey: "B2021", Title: "Second", Authors: []string{"B"}, Year: 2021},
				},
			},
			contains: []string{
				"@article{A2020,",
				"@article{B2021,",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateBibTeX(tt.refs)
			if tt.empty {
				if got != "" {
					t.Errorf("expected empty BibTeX, got %q", got)
				}
				return
			}
			for _, want := range tt.contains {
				if !strings.Contains(got, want) {
					t.Errorf("BibTeX missing %q:\n%s", want, got)
				}
			}
		})
	}
}

func TestGenerateBibTeXNoVenue(t *testing.T) {
	refs := &types.ReferencesFile{
		Papers: []types.ReferenceEntry{
			{CitationKey: "X2020", Title: "No Venue", Authors: []string{"X"}, Year: 2020},
		},
	}
	got := GenerateBibTeX(refs)
	if strings.Contains(got, "journal") {
		t.Error("BibTeX should not contain journal field when venue is empty")
	}
}
