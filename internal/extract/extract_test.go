package extract

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/pdiddy/research-engine/pkg/types"
)

// --- mock backend ---

type mockAIBackend struct {
	responses map[string]AIResponse // section prefix → response
	err       error                 // forced error for retry testing
	calls     int                   // counts calls for retry verification
}

func (m *mockAIBackend) Extract(_ context.Context, section string) (AIResponse, error) {
	m.calls++
	if m.err != nil {
		return AIResponse{}, m.err
	}
	// Match by first line of the section (heading).
	firstLine := strings.SplitN(section, "\n", 2)[0]
	if resp, ok := m.responses[firstLine]; ok {
		return resp, nil
	}
	return AIResponse{Items: nil}, nil
}

// failNTimesBackend fails the first N calls, then succeeds.
type failNTimesBackend struct {
	failures  int
	callCount int
	response  AIResponse
}

func (f *failNTimesBackend) Extract(_ context.Context, _ string) (AIResponse, error) {
	f.callCount++
	if f.callCount <= f.failures {
		return AIResponse{}, fmt.Errorf("transient error (call %d)", f.callCount)
	}
	return f.response, nil
}

func TestMain(m *testing.M) {
	// Override backoff to avoid real sleeps in retry tests.
	backoffBase = time.Millisecond
	os.Exit(m.Run())
}

func testConfig(papersDir, knowledgeDir string) types.ExtractionConfig {
	return types.ExtractionConfig{
		AIConfig: types.AIConfig{
			Model:      "test-model",
			MaxRetries: 3,
		},
		PapersDir:    papersDir,
		KnowledgeDir: knowledgeDir,
	}
}

// --- chunkByHeadings ---

func TestChunkByHeadings(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantLen  int
		wantHead []string
	}{
		{
			name:     "single section",
			content:  "## Introduction\n\nSome text here.",
			wantLen:  1,
			wantHead: []string{"Introduction"},
		},
		{
			name:     "two sections",
			content:  "## Introduction\n\nText.\n\n## Methods\n\nMore text.",
			wantLen:  2,
			wantHead: []string{"Introduction", "Methods"},
		},
		{
			name:     "h3 headings",
			content:  "### Sub-Section\n\nDetails.",
			wantLen:  1,
			wantHead: []string{"Sub-Section"},
		},
		{
			name:     "preamble before heading",
			content:  "Preamble text.\n\n## Introduction\n\nBody.",
			wantLen:  2,
			wantHead: []string{"", "Introduction"},
		},
		{
			name:     "page markers",
			content:  "## Results\n<!-- page 5 -->\nSome results.\n<!-- page 6 -->\nMore results.",
			wantLen:  1,
			wantHead: []string{"Results"},
		},
		{
			name:     "empty content",
			content:  "",
			wantLen:  0,
			wantHead: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sections := chunkByHeadings(tt.content)
			if len(sections) != tt.wantLen {
				t.Errorf("got %d sections, want %d", len(sections), tt.wantLen)
				for i, s := range sections {
					t.Logf("  section[%d]: heading=%q body=%q", i, s.heading, s.body)
				}
				return
			}
			for i, wantH := range tt.wantHead {
				if sections[i].heading != wantH {
					t.Errorf("section[%d].heading = %q, want %q", i, sections[i].heading, wantH)
				}
			}
		})
	}
}

func TestParsePageMarker(t *testing.T) {
	tests := []struct {
		line string
		page int
		ok   bool
	}{
		{"<!-- page 3 -->", 3, true},
		{"<!-- page 12 -->", 12, true},
		{"<!-- page 0 -->", 0, true},
		{"<!-- page -->", 0, false},
		{"not a marker", 0, false},
		{"<!-- page abc -->", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			page, ok := parsePageMarker(tt.line)
			if ok != tt.ok || page != tt.page {
				t.Errorf("parsePageMarker(%q) = (%d, %v), want (%d, %v)", tt.line, page, ok, tt.page, tt.ok)
			}
		})
	}
}

// --- stableID ---

func TestStableID(t *testing.T) {
	id1 := stableID("paper1", "Introduction", "Some claim.")
	id2 := stableID("paper1", "Introduction", "Some claim.")
	id3 := stableID("paper1", "Introduction", "Different claim.")

	if id1 != id2 {
		t.Errorf("same inputs produced different IDs: %s vs %s", id1, id2)
	}
	if id1 == id3 {
		t.Errorf("different inputs produced the same ID: %s", id1)
	}
	if len(id1) != 12 {
		t.Errorf("ID length = %d, want 12", len(id1))
	}
}

// --- convertItems ---

func TestConvertItems(t *testing.T) {
	tests := []struct {
		name       string
		items      []AIResponseItem
		paperID    string
		section    string
		wantCount  int
		wantErrors int
	}{
		{
			name: "valid items",
			items: []AIResponseItem{
				{Type: "claim", Content: "X improves Y.", Section: "Results", Page: 5, Confidence: 0.9, Tags: []string{"ml"}},
				{Type: "method", Content: "We use Z.", Section: "Methods", Page: 3, Confidence: 0.85, Tags: []string{"algorithm"}},
			},
			paperID:    "2301.07041",
			section:    "Results",
			wantCount:  2,
			wantErrors: 0,
		},
		{
			name: "invalid type rejected",
			items: []AIResponseItem{
				{Type: "opinion", Content: "Something.", Confidence: 0.5},
			},
			paperID:    "paper1",
			section:    "Intro",
			wantCount:  0,
			wantErrors: 1,
		},
		{
			name: "empty content rejected",
			items: []AIResponseItem{
				{Type: "claim", Content: "", Confidence: 0.5},
			},
			paperID:    "paper1",
			section:    "Intro",
			wantCount:  0,
			wantErrors: 1,
		},
		{
			name: "confidence out of range rejected",
			items: []AIResponseItem{
				{Type: "claim", Content: "Something.", Confidence: 1.5},
			},
			paperID:    "paper1",
			section:    "Intro",
			wantCount:  0,
			wantErrors: 1,
		},
		{
			name: "section falls back to parent heading",
			items: []AIResponseItem{
				{Type: "definition", Content: "A term.", Section: "", Page: 1, Confidence: 0.8},
			},
			paperID:    "paper1",
			section:    "Background",
			wantCount:  1,
			wantErrors: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items, errors := convertItems(tt.items, tt.paperID, tt.section)
			if len(items) != tt.wantCount {
				t.Errorf("got %d items, want %d", len(items), tt.wantCount)
			}
			if len(errors) != tt.wantErrors {
				t.Errorf("got %d errors, want %d: %v", len(errors), tt.wantErrors, errors)
			}
			// Verify stable ID is populated on valid items.
			for _, item := range items {
				if item.ID == "" {
					t.Error("item has empty ID")
				}
				if item.PaperID != tt.paperID {
					t.Errorf("item.PaperID = %q, want %q", item.PaperID, tt.paperID)
				}
			}
			// Verify section fallback.
			if tt.name == "section falls back to parent heading" && len(items) == 1 {
				if items[0].Section != "Background" {
					t.Errorf("item.Section = %q, want %q", items[0].Section, "Background")
				}
			}
		})
	}
}

// --- callWithRetry ---

func TestCallWithRetry(t *testing.T) {
	tests := []struct {
		name       string
		failures   int
		maxRetries int
		wantErr    bool
	}{
		{"succeeds first try", 0, 3, false},
		{"succeeds after 2 failures", 2, 3, false},
		{"fails after exhausting retries", 4, 3, true},
		{"succeeds on last retry", 3, 3, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := &failNTimesBackend{
				failures: tt.failures,
				response: AIResponse{Items: []AIResponseItem{
					{Type: "claim", Content: "Test.", Confidence: 0.9},
				}},
			}

			ctx := context.Background()
			_, err := callWithRetry(ctx, backend, "test chunk", tt.maxRetries)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// --- ExtractPaper (integration with mock) ---

func TestExtractPaper(t *testing.T) {
	tmpDir := t.TempDir()
	mdDir := filepath.Join(tmpDir, "papers", markdownDir)
	if err := os.MkdirAll(mdDir, 0o755); err != nil {
		t.Fatal(err)
	}

	mdContent := `## Introduction

This paper presents a new method for text classification.

## Methods

We use a transformer-based architecture with self-attention.

## Results

Our method achieves 95% accuracy on the benchmark dataset.
`
	if err := os.WriteFile(filepath.Join(mdDir, "test-paper.md"), []byte(mdContent), 0o644); err != nil {
		t.Fatal(err)
	}

	backend := &mockAIBackend{
		responses: map[string]AIResponse{
			"## Introduction": {Items: []AIResponseItem{
				{Type: "claim", Content: "This paper presents a new method for text classification.", Section: "Introduction", Page: 1, Confidence: 0.88, Tags: []string{"text-classification"}},
			}},
			"## Methods": {Items: []AIResponseItem{
				{Type: "method", Content: "We use a transformer-based architecture with self-attention.", Section: "Methods", Page: 1, Confidence: 0.92, Tags: []string{"transformer", "self-attention"}},
			}},
			"## Results": {Items: []AIResponseItem{
				{Type: "result", Content: "Our method achieves 95% accuracy on the benchmark dataset.", Section: "Results", Page: 1, Confidence: 0.95, Tags: []string{"accuracy", "benchmark"}},
			}},
		},
	}

	cfg := testConfig(filepath.Join(tmpDir, "papers"), filepath.Join(tmpDir, "knowledge"))

	result, err := ExtractPaper(context.Background(), backend, "test-paper", filepath.Join(mdDir, "test-paper.md"), cfg)
	if err != nil {
		t.Fatalf("ExtractPaper: %v", err)
	}

	if result.PaperID != "test-paper" {
		t.Errorf("PaperID = %q, want %q", result.PaperID, "test-paper")
	}
	if len(result.Items) != 3 {
		t.Errorf("got %d items, want 3", len(result.Items))
	}

	// Verify types.
	wantTypes := map[types.KnowledgeItemType]bool{
		types.ItemClaim:  false,
		types.ItemMethod: false,
		types.ItemResult: false,
	}
	for _, item := range result.Items {
		wantTypes[item.Type] = true
	}
	for typ, found := range wantTypes {
		if !found {
			t.Errorf("missing item type %q", typ)
		}
	}
}

// --- ExtractAll (batch processing) ---

func TestExtractAll(t *testing.T) {
	tmpDir := t.TempDir()
	mdDir := filepath.Join(tmpDir, "papers", markdownDir)
	knowledgeDir := filepath.Join(tmpDir, "knowledge")
	if err := os.MkdirAll(mdDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create two Markdown files.
	md1 := "## Intro\n\nClaim one."
	md2 := "## Intro\n\nClaim two."
	if err := os.WriteFile(filepath.Join(mdDir, "paper1.md"), []byte(md1), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mdDir, "paper2.md"), []byte(md2), 0o644); err != nil {
		t.Fatal(err)
	}

	backend := &mockAIBackend{
		responses: map[string]AIResponse{
			"## Intro": {Items: []AIResponseItem{
				{Type: "claim", Content: "A claim.", Section: "Intro", Page: 1, Confidence: 0.9, Tags: []string{"test"}},
			}},
		},
	}

	cfg := testConfig(filepath.Join(tmpDir, "papers"), knowledgeDir)

	var buf strings.Builder
	summary, err := ExtractAll(context.Background(), backend, cfg, &buf)
	if err != nil {
		t.Fatalf("ExtractAll: %v", err)
	}

	if summary.Extracted != 2 {
		t.Errorf("Extracted = %d, want 2", summary.Extracted)
	}
	if summary.Skipped != 0 {
		t.Errorf("Skipped = %d, want 0", summary.Skipped)
	}
	if summary.Failed != 0 {
		t.Errorf("Failed = %d, want 0", summary.Failed)
	}

	// Verify YAML files written.
	outDir := filepath.Join(knowledgeDir, extractedDir)
	for _, name := range []string{"paper1-items.yaml", "paper2-items.yaml"} {
		path := filepath.Join(outDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("reading %s: %v", name, err)
			continue
		}
		var result types.ExtractionResult
		if err := yaml.Unmarshal(data, &result); err != nil {
			t.Errorf("unmarshaling %s: %v", name, err)
			continue
		}
		if len(result.Items) != 1 {
			t.Errorf("%s: got %d items, want 1", name, len(result.Items))
		}
	}
}

// --- Skip unchanged files ---

func TestExtractAllSkipsUnchanged(t *testing.T) {
	tmpDir := t.TempDir()
	mdDir := filepath.Join(tmpDir, "papers", markdownDir)
	knowledgeDir := filepath.Join(tmpDir, "knowledge")
	outDir := filepath.Join(knowledgeDir, extractedDir)
	if err := os.MkdirAll(mdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}

	mdPath := filepath.Join(mdDir, "paper1.md")
	outPath := filepath.Join(outDir, "paper1-items.yaml")

	if err := os.WriteFile(mdPath, []byte("## Intro\n\nText."), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create output file with a future modification time so the Markdown
	// appears unchanged.
	result := &types.ExtractionResult{PaperID: "paper1", Items: []types.KnowledgeItem{
		{ID: "abc123", Type: types.ItemClaim, Content: "Existing.", PaperID: "paper1", Section: "Intro", Confidence: 0.9},
	}}
	data, _ := yaml.Marshal(result)
	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(outPath, future, future); err != nil {
		t.Fatal(err)
	}

	backend := &mockAIBackend{responses: map[string]AIResponse{}}

	cfg := testConfig(filepath.Join(tmpDir, "papers"), knowledgeDir)
	var buf strings.Builder
	summary, err := ExtractAll(context.Background(), backend, cfg, &buf)
	if err != nil {
		t.Fatalf("ExtractAll: %v", err)
	}

	if summary.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", summary.Skipped)
	}
	if summary.Extracted != 0 {
		t.Errorf("Extracted = %d, want 0", summary.Extracted)
	}
	if backend.calls != 0 {
		t.Errorf("backend.calls = %d, want 0 (should not call AI for skipped papers)", backend.calls)
	}
}

// --- Re-extract when changed ---

func TestExtractAllReextractsChanged(t *testing.T) {
	tmpDir := t.TempDir()
	mdDir := filepath.Join(tmpDir, "papers", markdownDir)
	knowledgeDir := filepath.Join(tmpDir, "knowledge")
	outDir := filepath.Join(knowledgeDir, extractedDir)
	if err := os.MkdirAll(mdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}

	outPath := filepath.Join(outDir, "paper1-items.yaml")
	// Create old output file.
	oldResult := &types.ExtractionResult{PaperID: "paper1", Items: []types.KnowledgeItem{
		{ID: "old123", Type: types.ItemClaim, Content: "Old claim.", PaperID: "paper1", Section: "Intro", Confidence: 0.9},
	}}
	oldData, _ := yaml.Marshal(oldResult)
	if err := os.WriteFile(outPath, oldData, 0o644); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-time.Hour)
	if err := os.Chtimes(outPath, past, past); err != nil {
		t.Fatal(err)
	}

	// Write a newer Markdown file.
	mdPath := filepath.Join(mdDir, "paper1.md")
	if err := os.WriteFile(mdPath, []byte("## Intro\n\nUpdated text."), 0o644); err != nil {
		t.Fatal(err)
	}

	backend := &mockAIBackend{
		responses: map[string]AIResponse{
			"## Intro": {Items: []AIResponseItem{
				{Type: "claim", Content: "Updated claim.", Section: "Intro", Page: 1, Confidence: 0.9, Tags: []string{"update"}},
			}},
		},
	}

	cfg := testConfig(filepath.Join(tmpDir, "papers"), knowledgeDir)
	var buf strings.Builder
	summary, err := ExtractAll(context.Background(), backend, cfg, &buf)
	if err != nil {
		t.Fatalf("ExtractAll: %v", err)
	}

	if summary.Extracted != 1 {
		t.Errorf("Extracted = %d, want 1", summary.Extracted)
	}

	// Verify the output was replaced.
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	var newResult types.ExtractionResult
	if err := yaml.Unmarshal(data, &newResult); err != nil {
		t.Fatal(err)
	}
	if len(newResult.Items) != 1 {
		t.Fatalf("got %d items, want 1", len(newResult.Items))
	}
	if newResult.Items[0].Content != "Updated claim." {
		t.Errorf("item content = %q, want %q", newResult.Items[0].Content, "Updated claim.")
	}
}

// --- Validation failure ---

func TestExtractPaperValidationFailure(t *testing.T) {
	tmpDir := t.TempDir()
	mdDir := filepath.Join(tmpDir, "papers", markdownDir)
	if err := os.MkdirAll(mdDir, 0o755); err != nil {
		t.Fatal(err)
	}

	mdPath := filepath.Join(mdDir, "bad-paper.md")
	if err := os.WriteFile(mdPath, []byte("## Intro\n\nText."), 0o644); err != nil {
		t.Fatal(err)
	}

	// Backend returns an invalid item type.
	backend := &mockAIBackend{
		responses: map[string]AIResponse{
			"## Intro": {Items: []AIResponseItem{
				{Type: "opinion", Content: "Not a valid type.", Confidence: 0.5},
			}},
		},
	}

	cfg := testConfig(filepath.Join(tmpDir, "papers"), filepath.Join(tmpDir, "knowledge"))
	_, err := ExtractPaper(context.Background(), backend, "bad-paper", mdPath, cfg)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid type") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "invalid type")
	}
}

// --- Retry exhaustion in batch ---

func TestExtractAllRetryExhaustion(t *testing.T) {
	tmpDir := t.TempDir()
	mdDir := filepath.Join(tmpDir, "papers", markdownDir)
	knowledgeDir := filepath.Join(tmpDir, "knowledge")
	if err := os.MkdirAll(mdDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(mdDir, "fail-paper.md"), []byte("## Intro\n\nText."), 0o644); err != nil {
		t.Fatal(err)
	}

	// Backend always fails.
	backend := &mockAIBackend{
		err: fmt.Errorf("API unavailable"),
	}

	cfg := testConfig(filepath.Join(tmpDir, "papers"), knowledgeDir)
	var buf strings.Builder
	summary, err := ExtractAll(context.Background(), backend, cfg, &buf)
	if err != nil {
		t.Fatalf("ExtractAll should not return error for individual failures: %v", err)
	}

	if summary.Failed != 1 {
		t.Errorf("Failed = %d, want 1", summary.Failed)
	}
	if summary.HasFailures() != true {
		t.Error("HasFailures() should return true")
	}
	if !strings.Contains(buf.String(), "failed") {
		t.Errorf("output should contain 'failed': %s", buf.String())
	}
}

// --- BatchSummary ---

func TestBatchSummary(t *testing.T) {
	s := BatchSummary{Extracted: 3, Skipped: 2, Failed: 1}
	if s.Total() != 6 {
		t.Errorf("Total() = %d, want 6", s.Total())
	}
	if !s.HasFailures() {
		t.Error("HasFailures() should return true")
	}

	s2 := BatchSummary{Extracted: 5, Skipped: 0, Failed: 0}
	if s2.HasFailures() {
		t.Error("HasFailures() should return false")
	}
}

// --- renderPrompt ---

func TestRenderPrompt(t *testing.T) {
	prompt, err := renderPrompt("## Introduction\n\nSome text.")
	if err != nil {
		t.Fatalf("renderPrompt: %v", err)
	}
	if !strings.Contains(prompt, "## Introduction") {
		t.Error("prompt should contain the section content")
	}
	if !strings.Contains(prompt, "knowledge extraction") {
		t.Error("prompt should contain extraction instructions")
	}
}

// --- ParseCitations ---

func TestParseCitations(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		wantKeys []string
	}{
		{
			name:     "numeric citations",
			text:     "As shown in [1] and confirmed by [2], the method works.",
			wantKeys: []string{"1", "2"},
		},
		{
			name:     "author-year citation",
			text:     "According to [Smith et al., 2020], transformers outperform RNNs.",
			wantKeys: []string{"Smith et al., 2020"},
		},
		{
			name:     "mixed formats",
			text:     "Prior work [1] and [Jones, 2019] both report similar findings [3].",
			wantKeys: []string{"1", "3", "Jones, 2019"},
		},
		{
			name:     "no citations",
			text:     "This sentence has no citations at all.",
			wantKeys: nil,
		},
		{
			name:     "duplicate numeric citation",
			text:     "See [1] for details and also [1] for more.",
			wantKeys: []string{"1"},
		},
		{
			name:     "author and coauthor",
			text:     "As described by [Smith and Jones, 2021], the results hold.",
			wantKeys: []string{"Smith and Jones, 2021"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			citations := ParseCitations(tt.text)
			var gotKeys []string
			for _, c := range citations {
				gotKeys = append(gotKeys, c.Key)
			}
			if len(gotKeys) != len(tt.wantKeys) {
				t.Errorf("got %d citations %v, want %d %v", len(gotKeys), gotKeys, len(tt.wantKeys), tt.wantKeys)
				return
			}
			for i, want := range tt.wantKeys {
				if gotKeys[i] != want {
					t.Errorf("citation[%d].Key = %q, want %q", i, gotKeys[i], want)
				}
			}
			// All unlinked citations should have BibIndex -1.
			for i, c := range citations {
				if c.BibIndex != -1 {
					t.Errorf("citation[%d].BibIndex = %d, want -1", i, c.BibIndex)
				}
			}
		})
	}
}

// --- ParseBibliography ---

func TestParseBibliography(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantCount int
		wantKeys  []string
	}{
		{
			name: "numbered bibliography",
			content: `## Introduction

Some text.

## References

[1] Smith, A. and Jones, B. Attention is all you need. NeurIPS, 2017.
[2] Brown, T. et al. Language models are few-shot learners. NeurIPS, 2020.
[3] Devlin, J. BERT: Pre-training of deep bidirectional transformers. NAACL, 2019.
`,
			wantCount: 3,
			wantKeys:  []string{"1", "2", "3"},
		},
		{
			name: "bibliography heading",
			content: `## Methods

Details.

## Bibliography

[1] Author, A. Title of paper. Journal, 2020.
`,
			wantCount: 1,
			wantKeys:  []string{"1"},
		},
		{
			name:      "no references section",
			content:   "## Introduction\n\nText.\n\n## Methods\n\nMore text.",
			wantCount: 0,
			wantKeys:  nil,
		},
		{
			name: "references with following section",
			content: `## References

[1] Author A. Title one. Journal, 2020.
[2] Author B. Title two. Conference, 2021.

## Appendix

Extra material.
`,
			wantCount: 2,
			wantKeys:  []string{"1", "2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries := ParseBibliography(tt.content)
			if len(entries) != tt.wantCount {
				t.Errorf("got %d entries, want %d", len(entries), tt.wantCount)
				for i, e := range entries {
					t.Logf("  entry[%d]: key=%q title=%q", i, e.Key, e.Title)
				}
				return
			}
			for i, wantKey := range tt.wantKeys {
				if entries[i].Key != wantKey {
					t.Errorf("entry[%d].Key = %q, want %q", i, entries[i].Key, wantKey)
				}
			}
		})
	}
}

func TestParseBibEntryMetadata(t *testing.T) {
	content := `## References

[1] Smith, A. and Jones, B. Attention is all you need. NeurIPS, 2017.
`
	entries := ParseBibliography(content)
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}

	e := entries[0]
	if e.Year != "2017" {
		t.Errorf("Year = %q, want %q", e.Year, "2017")
	}
	if e.Title != "Attention is all you need" {
		t.Errorf("Title = %q, want %q", e.Title, "Attention is all you need")
	}
	if len(e.Authors) == 0 {
		t.Error("Authors is empty")
	}
}

// --- LinkCitations ---

func TestLinkCitations(t *testing.T) {
	bibliography := []types.BibliographyEntry{
		{Key: "1", Title: "Paper One", Year: "2020"},
		{Key: "2", Title: "Paper Two", Year: "2021"},
		{Key: "3", Title: "Paper Three", Year: "2022"},
	}

	tests := []struct {
		name     string
		citations []types.Citation
		wantIdx  []int
	}{
		{
			name: "all match",
			citations: []types.Citation{
				{Key: "1", BibIndex: -1},
				{Key: "3", BibIndex: -1},
			},
			wantIdx: []int{0, 2},
		},
		{
			name: "partial match",
			citations: []types.Citation{
				{Key: "1", BibIndex: -1},
				{Key: "99", BibIndex: -1},
			},
			wantIdx: []int{0, -1},
		},
		{
			name:      "empty citations",
			citations: nil,
			wantIdx:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			linked := LinkCitations(tt.citations, bibliography)
			if len(linked) != len(tt.wantIdx) {
				t.Errorf("got %d citations, want %d", len(linked), len(tt.wantIdx))
				return
			}
			for i, wantBib := range tt.wantIdx {
				if linked[i].BibIndex != wantBib {
					t.Errorf("citation[%d].BibIndex = %d, want %d", i, linked[i].BibIndex, wantBib)
				}
			}
		})
	}
}

func TestLinkCitationsEmptyBibliography(t *testing.T) {
	citations := []types.Citation{
		{Key: "1", BibIndex: -1},
	}
	linked := LinkCitations(citations, nil)
	if len(linked) != 1 {
		t.Fatalf("got %d citations, want 1", len(linked))
	}
	if linked[0].BibIndex != -1 {
		t.Errorf("BibIndex = %d, want -1 (no bibliography to match)", linked[0].BibIndex)
	}
}

// --- AggregatePaperTags ---

func TestAggregatePaperTags(t *testing.T) {
	tests := []struct {
		name     string
		items    []types.KnowledgeItem
		wantTags []string
	}{
		{
			name: "unique tags sorted",
			items: []types.KnowledgeItem{
				{Tags: []string{"transformer", "attention"}},
				{Tags: []string{"benchmark", "transformer"}},
			},
			wantTags: []string{"attention", "benchmark", "transformer"},
		},
		{
			name:     "no items",
			items:    nil,
			wantTags: []string{},
		},
		{
			name: "items without tags",
			items: []types.KnowledgeItem{
				{Tags: nil},
				{Tags: []string{}},
			},
			wantTags: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tags := AggregatePaperTags(tt.items)
			if len(tags) != len(tt.wantTags) {
				t.Errorf("got %d tags %v, want %d %v", len(tags), tags, len(tt.wantTags), tt.wantTags)
				return
			}
			for i, want := range tt.wantTags {
				if tags[i] != want {
					t.Errorf("tag[%d] = %q, want %q", i, tags[i], want)
				}
			}
		})
	}
}

// --- Integration: ExtractPaper with citations and tags ---

func TestExtractPaperCitationsAndTags(t *testing.T) {
	tmpDir := t.TempDir()
	mdDir := filepath.Join(tmpDir, "papers", markdownDir)
	if err := os.MkdirAll(mdDir, 0o755); err != nil {
		t.Fatal(err)
	}

	mdContent := `## Introduction

Transformers [1] have revolutionized NLP. Building on prior work [2], we propose
an improved attention mechanism.

## Methods

We extend the self-attention layer described in [1] with sparse attention.

## Results

Our method outperforms the baseline [3] by 5% on GLUE.

## References

[1] Vaswani, A. et al. Attention is all you need. NeurIPS, 2017.
[2] Devlin, J. et al. BERT: Pre-training of deep bidirectional transformers. NAACL, 2019.
[3] Brown, T. et al. Language models are few-shot learners. NeurIPS, 2020.
`
	mdPath := filepath.Join(mdDir, "cite-paper.md")
	if err := os.WriteFile(mdPath, []byte(mdContent), 0o644); err != nil {
		t.Fatal(err)
	}

	backend := &mockAIBackend{
		responses: map[string]AIResponse{
			"## Introduction": {Items: []AIResponseItem{
				{Type: "claim", Content: "Transformers [1] have revolutionized NLP.", Section: "Introduction", Page: 1, Confidence: 0.9, Tags: []string{"transformer", "nlp"}},
			}},
			"## Methods": {Items: []AIResponseItem{
				{Type: "method", Content: "We extend the self-attention layer described in [1] with sparse attention.", Section: "Methods", Page: 1, Confidence: 0.88, Tags: []string{"self-attention", "sparse-attention"}},
			}},
			"## Results": {Items: []AIResponseItem{
				{Type: "result", Content: "Our method outperforms the baseline [3] by 5% on GLUE.", Section: "Results", Page: 1, Confidence: 0.95, Tags: []string{"benchmark", "glue"}},
			}},
		},
	}

	cfg := testConfig(filepath.Join(tmpDir, "papers"), filepath.Join(tmpDir, "knowledge"))

	result, err := ExtractPaper(context.Background(), backend, "cite-paper", mdPath, cfg)
	if err != nil {
		t.Fatalf("ExtractPaper: %v", err)
	}

	// Verify bibliography was parsed (R3.2).
	if len(result.Bibliography) != 3 {
		t.Errorf("Bibliography: got %d entries, want 3", len(result.Bibliography))
	} else {
		if result.Bibliography[0].Key != "1" {
			t.Errorf("Bibliography[0].Key = %q, want %q", result.Bibliography[0].Key, "1")
		}
		if result.Bibliography[0].Year != "2017" {
			t.Errorf("Bibliography[0].Year = %q, want %q", result.Bibliography[0].Year, "2017")
		}
	}

	// Verify citations on items (R3.1, R3.3, R3.4).
	if len(result.Items) != 3 {
		t.Fatalf("got %d items, want 3", len(result.Items))
	}

	// Item 0: "Transformers [1] have revolutionized NLP." → citation [1].
	if len(result.Items[0].Citations) != 1 {
		t.Errorf("Items[0].Citations: got %d, want 1", len(result.Items[0].Citations))
	} else {
		c := result.Items[0].Citations[0]
		if c.Key != "1" {
			t.Errorf("citation key = %q, want %q", c.Key, "1")
		}
		if c.BibIndex != 0 {
			t.Errorf("citation BibIndex = %d, want 0", c.BibIndex)
		}
	}

	// Item 2: "... baseline [3] ..." → citation [3] linked to bib index 2.
	if len(result.Items[2].Citations) != 1 {
		t.Errorf("Items[2].Citations: got %d, want 1", len(result.Items[2].Citations))
	} else if result.Items[2].Citations[0].BibIndex != 2 {
		t.Errorf("Items[2].Citations[0].BibIndex = %d, want 2", result.Items[2].Citations[0].BibIndex)
	}

	// Verify paper-level tags (R4.3).
	if len(result.PaperTags) == 0 {
		t.Error("PaperTags is empty, expected aggregated tags")
	}
	// Should contain all unique tags from items.
	tagSet := make(map[string]bool)
	for _, tag := range result.PaperTags {
		tagSet[tag] = true
	}
	for _, want := range []string{"transformer", "nlp", "self-attention", "sparse-attention", "benchmark", "glue"} {
		if !tagSet[want] {
			t.Errorf("PaperTags missing %q", want)
		}
	}
	// Tags should be sorted.
	for i := 1; i < len(result.PaperTags); i++ {
		if result.PaperTags[i] < result.PaperTags[i-1] {
			t.Errorf("PaperTags not sorted: %v", result.PaperTags)
			break
		}
	}
}

func TestExtractPaperNoBibliography(t *testing.T) {
	tmpDir := t.TempDir()
	mdDir := filepath.Join(tmpDir, "papers", markdownDir)
	if err := os.MkdirAll(mdDir, 0o755); err != nil {
		t.Fatal(err)
	}

	mdContent := `## Introduction

A simple paper with no references section.
`
	mdPath := filepath.Join(mdDir, "no-bib.md")
	if err := os.WriteFile(mdPath, []byte(mdContent), 0o644); err != nil {
		t.Fatal(err)
	}

	backend := &mockAIBackend{
		responses: map[string]AIResponse{
			"## Introduction": {Items: []AIResponseItem{
				{Type: "claim", Content: "A simple paper.", Section: "Introduction", Page: 1, Confidence: 0.8, Tags: []string{"test"}},
			}},
		},
	}

	cfg := testConfig(filepath.Join(tmpDir, "papers"), filepath.Join(tmpDir, "knowledge"))
	result, err := ExtractPaper(context.Background(), backend, "no-bib", mdPath, cfg)
	if err != nil {
		t.Fatalf("ExtractPaper: %v", err)
	}

	if len(result.Bibliography) != 0 {
		t.Errorf("Bibliography: got %d entries, want 0", len(result.Bibliography))
	}

	// Items without citation markers should have no citations.
	if len(result.Items[0].Citations) != 0 {
		t.Errorf("Items[0].Citations: got %d, want 0", len(result.Items[0].Citations))
	}

	// Paper tags should still be aggregated from items.
	if len(result.PaperTags) != 1 || result.PaperTags[0] != "test" {
		t.Errorf("PaperTags = %v, want [test]", result.PaperTags)
	}
}

func TestExtractPaperAuthorYearCitations(t *testing.T) {
	tmpDir := t.TempDir()
	mdDir := filepath.Join(tmpDir, "papers", markdownDir)
	if err := os.MkdirAll(mdDir, 0o755); err != nil {
		t.Fatal(err)
	}

	mdContent := `## Introduction

As shown by [Smith et al., 2020], the approach works well.
`
	mdPath := filepath.Join(mdDir, "author-cite.md")
	if err := os.WriteFile(mdPath, []byte(mdContent), 0o644); err != nil {
		t.Fatal(err)
	}

	backend := &mockAIBackend{
		responses: map[string]AIResponse{
			"## Introduction": {Items: []AIResponseItem{
				{Type: "claim", Content: "As shown by [Smith et al., 2020], the approach works well.", Section: "Introduction", Page: 1, Confidence: 0.85, Tags: []string{"survey"}},
			}},
		},
	}

	cfg := testConfig(filepath.Join(tmpDir, "papers"), filepath.Join(tmpDir, "knowledge"))
	result, err := ExtractPaper(context.Background(), backend, "author-cite", mdPath, cfg)
	if err != nil {
		t.Fatalf("ExtractPaper: %v", err)
	}

	if len(result.Items) != 1 {
		t.Fatalf("got %d items, want 1", len(result.Items))
	}

	// Should find the author-year citation (unlinked since no numbered bib).
	if len(result.Items[0].Citations) != 1 {
		t.Errorf("Citations: got %d, want 1", len(result.Items[0].Citations))
	} else {
		c := result.Items[0].Citations[0]
		if c.Key != "Smith et al., 2020" {
			t.Errorf("citation key = %q, want %q", c.Key, "Smith et al., 2020")
		}
		if c.BibIndex != -1 {
			t.Errorf("citation BibIndex = %d, want -1 (no matching bib entry)", c.BibIndex)
		}
	}
}
