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
