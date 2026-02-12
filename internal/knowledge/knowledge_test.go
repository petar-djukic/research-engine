package knowledge

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/pdiddy/research-engine/pkg/types"
)

// --- test helpers ---

func testSetup(t *testing.T) (*Store, string) {
	t.Helper()
	tmpDir := t.TempDir()

	for _, dir := range []string{
		filepath.Join(tmpDir, "knowledge", extractedDir),
		filepath.Join(tmpDir, "papers", metadataDir),
		filepath.Join(tmpDir, "papers", markdownDir),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	cfg := types.KnowledgeBaseConfig{
		KnowledgeDir: filepath.Join(tmpDir, "knowledge"),
		MaxResults:   20,
	}
	store, err := NewStore(cfg, filepath.Join(tmpDir, "papers"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })

	return store, tmpDir
}

func writeExtraction(t *testing.T, tmpDir, paperID string, items []types.KnowledgeItem) {
	t.Helper()
	result := types.ExtractionResult{
		PaperID: paperID,
		Items:   items,
	}
	data, err := yaml.Marshal(&result)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(tmpDir, "knowledge", extractedDir, paperID+"-items.yaml")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func writePaperMeta(t *testing.T, tmpDir string, paper types.Paper) {
	t.Helper()
	data, err := yaml.Marshal(&paper)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(tmpDir, "papers", metadataDir, paper.ID+".yaml")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeMarkdown(t *testing.T, tmpDir, paperID, content string) {
	t.Helper()
	path := filepath.Join(tmpDir, "papers", markdownDir, paperID+".md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func sampleItems(paperID string) []types.KnowledgeItem {
	return []types.KnowledgeItem{
		{
			ID: paperID + "-claim1", Type: types.ItemClaim,
			Content: "Efficient attention reduces computation from O(n^2) to O(n log n)",
			PaperID: paperID, Section: "Method", Page: 2, Confidence: 0.92,
			Tags: []string{"attention", "efficiency"},
		},
		{
			ID: paperID + "-method1", Type: types.ItemMethod,
			Content: "We define efficient attention as a linear approximation of softmax",
			PaperID: paperID, Section: "Method", Page: 3, Confidence: 0.95,
			Tags: []string{"attention", "linear-approximation"},
		},
		{
			ID: paperID + "-def1", Type: types.ItemDefinition,
			Content: "Softmax attention computes weighted averages over all input positions",
			PaperID: paperID, Section: "Background", Page: 1, Confidence: 0.88,
			Tags: []string{"attention", "softmax"},
		},
		{
			ID: paperID + "-result1", Type: types.ItemResult,
			Content: "Our method achieves 89.2% accuracy on the GLUE benchmark",
			PaperID: paperID, Section: "Results", Page: 5, Confidence: 0.97,
			Tags: []string{"benchmark", "accuracy"},
		},
	}
}

func samplePaper(paperID string) types.Paper {
	return types.Paper{
		ID:      paperID,
		Title:   "Efficient Attention Mechanisms for Transformers",
		Authors: []string{"Smith, J.", "Doe, A."},
	}
}

// ingestHelper writes extraction and metadata files, then ingests.
func ingestHelper(t *testing.T, store *Store, tmpDir, paperID string) {
	t.Helper()
	writeExtraction(t, tmpDir, paperID, sampleItems(paperID))
	writePaperMeta(t, tmpDir, samplePaper(paperID))
	var buf strings.Builder
	if _, err := store.Ingest(context.Background(), &buf); err != nil {
		t.Fatal(err)
	}
}

// --- schema tests ---

func TestNewStoreCreatesSchema(t *testing.T) {
	store, _ := testSetup(t)

	tables := []string{"items", "papers", "items_fts", "indexing_status"}
	for _, table := range tables {
		var count int
		err := store.db.QueryRow(
			`SELECT count(*) FROM sqlite_master WHERE type IN ('table','view') AND name = ?`, table,
		).Scan(&count)
		if err != nil {
			t.Fatalf("checking table %s: %v", table, err)
		}
		if count == 0 {
			t.Errorf("table %s does not exist", table)
		}
	}
}

func TestNewStoreCreatesDBFile(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "knowledge", indexDir, dbFile)

	cfg := types.KnowledgeBaseConfig{KnowledgeDir: filepath.Join(tmpDir, "knowledge")}
	store, err := NewStore(cfg, filepath.Join(tmpDir, "papers"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Errorf("database file not created at %s", dbPath)
	}
}

// --- ingest tests ---

func TestIngest(t *testing.T) {
	tests := []struct {
		name        string
		papers      int
		wantIndexed int
	}{
		{"single paper", 1, 1},
		{"multiple papers", 3, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, tmpDir := testSetup(t)

			for i := 0; i < tt.papers; i++ {
				paperID := fmt.Sprintf("paper-%d", i)
				writeExtraction(t, tmpDir, paperID, sampleItems(paperID))
				writePaperMeta(t, tmpDir, types.Paper{
					ID:      paperID,
					Title:   fmt.Sprintf("Paper %d Title", i),
					Authors: []string{"Author A"},
				})
			}

			var buf strings.Builder
			summary, err := store.Ingest(context.Background(), &buf)
			if err != nil {
				t.Fatalf("Ingest: %v", err)
			}
			if summary.Indexed != tt.wantIndexed {
				t.Errorf("Indexed = %d, want %d", summary.Indexed, tt.wantIndexed)
			}
			if summary.Failed != 0 {
				t.Errorf("Failed = %d, want 0; output: %s", summary.Failed, buf.String())
			}
		})
	}
}

func TestIngestStoresAllFields(t *testing.T) {
	store, tmpDir := testSetup(t)

	items := []types.KnowledgeItem{{
		ID: "item-abc", Type: types.ItemMethod,
		Content: "We define efficient attention as a linear approximation",
		PaperID: "2301.07041", Section: "Method", Page: 2, Confidence: 0.95,
		Tags:      []string{"attention", "linear-approximation"},
		Citations: []types.Citation{{Key: "[1]", BibIndex: 0, Context: "prior work [1]"}},
	}}
	writeExtraction(t, tmpDir, "2301.07041", items)
	writePaperMeta(t, tmpDir, types.Paper{ID: "2301.07041", Title: "Test"})

	var buf strings.Builder
	if _, err := store.Ingest(context.Background(), &buf); err != nil {
		t.Fatal(err)
	}

	// Verify all fields round-trip through the database.
	results, err := store.Retrieve(context.Background(), QueryOptions{PaperID: "2301.07041"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}

	r := results[0]
	if r.ID != "item-abc" {
		t.Errorf("ID = %q, want %q", r.ID, "item-abc")
	}
	if r.Type != types.ItemMethod {
		t.Errorf("Type = %q, want %q", r.Type, types.ItemMethod)
	}
	if r.Section != "Method" {
		t.Errorf("Section = %q, want %q", r.Section, "Method")
	}
	if r.Page != 2 {
		t.Errorf("Page = %d, want 2", r.Page)
	}
	if r.Confidence != 0.95 {
		t.Errorf("Confidence = %f, want 0.95", r.Confidence)
	}
	if len(r.Tags) != 2 || r.Tags[0] != "attention" {
		t.Errorf("Tags = %v, want [attention linear-approximation]", r.Tags)
	}
	if len(r.Citations) != 1 || r.Citations[0].Key != "[1]" {
		t.Errorf("Citations = %v, want [{Key:[1]}]", r.Citations)
	}
}

func TestIngestPopulatesPapersTable(t *testing.T) {
	store, tmpDir := testSetup(t)
	ingestHelper(t, store, tmpDir, "2301.07041")

	var title, authorsJSON string
	err := store.db.QueryRow(
		`SELECT title, authors FROM papers WHERE id = ?`, "2301.07041",
	).Scan(&title, &authorsJSON)
	if err != nil {
		t.Fatal(err)
	}
	if title != "Efficient Attention Mechanisms for Transformers" {
		t.Errorf("title = %q", title)
	}
	var authors []string
	json.Unmarshal([]byte(authorsJSON), &authors)
	if len(authors) != 2 {
		t.Errorf("authors = %v, want 2 entries", authors)
	}
}

func TestIngestWritesExportYAML(t *testing.T) {
	store, tmpDir := testSetup(t)
	ingestHelper(t, store, tmpDir, "paper-export")

	path := filepath.Join(tmpDir, "knowledge", indexDir, "export.yaml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("export.yaml not written after ingestion")
	}
}

// --- incremental update tests ---

func TestIngestSkipsUnchanged(t *testing.T) {
	store, tmpDir := testSetup(t)
	ingestHelper(t, store, tmpDir, "paper-skip")

	// Second ingestion without modifying the file.
	var buf strings.Builder
	summary, err := store.Ingest(context.Background(), &buf)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", summary.Skipped)
	}
	if summary.Indexed != 0 {
		t.Errorf("Indexed = %d, want 0", summary.Indexed)
	}
	if !strings.Contains(buf.String(), "skipped") {
		t.Errorf("output should contain 'skipped': %s", buf.String())
	}
}

func TestIngestUpdatesChanged(t *testing.T) {
	store, tmpDir := testSetup(t)
	ingestHelper(t, store, tmpDir, "paper-update")

	// Rewrite the extraction file with new content and a newer mod time.
	newItems := []types.KnowledgeItem{{
		ID: "updated-item", Type: types.ItemClaim,
		Content: "Updated claim content",
		PaperID: "paper-update", Section: "New Section", Page: 10, Confidence: 0.99,
		Tags: []string{"updated"},
	}}
	writeExtraction(t, tmpDir, "paper-update", newItems)

	// Touch the file to ensure mod time changes.
	path := filepath.Join(tmpDir, "knowledge", extractedDir, "paper-update-items.yaml")
	future := time.Now().Add(time.Second)
	os.Chtimes(path, future, future)

	var buf strings.Builder
	summary, err := store.Ingest(context.Background(), &buf)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Updated != 1 {
		t.Errorf("Updated = %d, want 1", summary.Updated)
	}

	// Verify old items removed and new item present.
	results, err := store.Retrieve(context.Background(), QueryOptions{PaperID: "paper-update"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1 (old items should be removed)", len(results))
	}
	if results[0].Content != "Updated claim content" {
		t.Errorf("content = %q, want %q", results[0].Content, "Updated claim content")
	}
}

func TestIngestSummaryOutput(t *testing.T) {
	store, tmpDir := testSetup(t)

	writeExtraction(t, tmpDir, "paper1", sampleItems("paper1"))
	writePaperMeta(t, tmpDir, types.Paper{ID: "paper1", Title: "P1"})

	var buf strings.Builder
	_, err := store.Ingest(context.Background(), &buf)
	if err != nil {
		t.Fatal(err)
	}
	output := buf.String()

	if !strings.Contains(output, "indexed: 1") {
		t.Errorf("output should contain 'indexed: 1': %s", output)
	}
	if !strings.Contains(output, "skipped: 0") {
		t.Errorf("output should contain 'skipped: 0': %s", output)
	}
}

// --- full-text search tests ---

func TestRetrieveFullTextSearch(t *testing.T) {
	store, tmpDir := testSetup(t)
	ingestHelper(t, store, tmpDir, "fts-paper")

	tests := []struct {
		name       string
		query      string
		wantMin    int
		wantInContent string
	}{
		{"matching term", "attention", 3, "attention"},
		{"exact phrase", "GLUE benchmark", 1, "GLUE"},
		{"no match", "quantum entanglement xyzzy", 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := store.Retrieve(context.Background(), QueryOptions{Query: tt.query})
			if err != nil {
				t.Fatal(err)
			}
			if len(results) < tt.wantMin {
				t.Errorf("got %d results, want >= %d", len(results), tt.wantMin)
			}
			if tt.wantInContent != "" {
				for _, r := range results {
					if !strings.Contains(strings.ToLower(r.Content), strings.ToLower(tt.wantInContent)) {
						t.Errorf("result content %q does not contain %q", r.Content, tt.wantInContent)
					}
				}
			}
		})
	}
}

func TestRetrieveFullTextSearchIncludesPaperMetadata(t *testing.T) {
	store, tmpDir := testSetup(t)
	ingestHelper(t, store, tmpDir, "meta-paper")

	results, err := store.Retrieve(context.Background(), QueryOptions{Query: "attention"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected results")
	}
	for _, r := range results {
		if r.PaperID == "" {
			t.Error("result missing paper_id")
		}
		if r.Section == "" {
			t.Error("result missing section")
		}
		if r.PaperTitle == "" {
			t.Error("result missing paper_title")
		}
		if len(r.PaperAuthors) == 0 {
			t.Error("result missing paper_authors")
		}
	}
}

func TestRetrieveRespectsMaxResults(t *testing.T) {
	store, tmpDir := testSetup(t)
	ingestHelper(t, store, tmpDir, "limit-paper")

	results, err := store.Retrieve(context.Background(), QueryOptions{
		Query:      "attention",
		MaxResults: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) > 2 {
		t.Errorf("got %d results, want <= 2", len(results))
	}
}

// --- structured query tests ---

func TestRetrieveByType(t *testing.T) {
	store, tmpDir := testSetup(t)
	ingestHelper(t, store, tmpDir, "type-paper")

	tests := []struct {
		itemType types.KnowledgeItemType
		wantCount int
	}{
		{types.ItemClaim, 1},
		{types.ItemMethod, 1},
		{types.ItemDefinition, 1},
		{types.ItemResult, 1},
	}

	for _, tt := range tests {
		t.Run(string(tt.itemType), func(t *testing.T) {
			results, err := store.Retrieve(context.Background(), QueryOptions{Type: tt.itemType})
			if err != nil {
				t.Fatal(err)
			}
			if len(results) != tt.wantCount {
				t.Errorf("got %d results, want %d", len(results), tt.wantCount)
			}
			for _, r := range results {
				if r.Type != tt.itemType {
					t.Errorf("result type = %q, want %q", r.Type, tt.itemType)
				}
			}
		})
	}
}

func TestRetrieveByTag(t *testing.T) {
	store, tmpDir := testSetup(t)
	ingestHelper(t, store, tmpDir, "tag-paper")

	tests := []struct {
		tag       string
		wantMin   int
	}{
		{"attention", 3},
		{"benchmark", 1},
		{"nonexistent-tag", 0},
	}

	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			results, err := store.Retrieve(context.Background(), QueryOptions{Tags: []string{tt.tag}})
			if err != nil {
				t.Fatal(err)
			}
			if len(results) < tt.wantMin {
				t.Errorf("got %d results, want >= %d", len(results), tt.wantMin)
			}
			for _, r := range results {
				found := false
				for _, t2 := range r.Tags {
					if t2 == tt.tag {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("result tags %v do not contain %q", r.Tags, tt.tag)
				}
			}
		})
	}
}

func TestRetrieveByPaperID(t *testing.T) {
	store, tmpDir := testSetup(t)

	// Ingest two papers.
	for _, pid := range []string{"paper-a", "paper-b"} {
		writeExtraction(t, tmpDir, pid, sampleItems(pid))
		writePaperMeta(t, tmpDir, types.Paper{ID: pid, Title: pid})
	}
	var buf strings.Builder
	store.Ingest(context.Background(), &buf)

	results, err := store.Retrieve(context.Background(), QueryOptions{PaperID: "paper-a"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 4 {
		t.Errorf("got %d results, want 4", len(results))
	}
	for _, r := range results {
		if r.PaperID != "paper-a" {
			t.Errorf("result paper_id = %q, want %q", r.PaperID, "paper-a")
		}
	}
}

// --- combined query tests ---

func TestRetrieveCombinedQuery(t *testing.T) {
	store, tmpDir := testSetup(t)
	ingestHelper(t, store, tmpDir, "combo-paper")

	// FTS + type + tag.
	results, err := store.Retrieve(context.Background(), QueryOptions{
		Query: "attention",
		Type:  types.ItemClaim,
		Tags:  []string{"efficiency"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	r := results[0]
	if r.Type != types.ItemClaim {
		t.Errorf("type = %q, want claim", r.Type)
	}
	if !strings.Contains(r.Content, "attention") {
		t.Errorf("content should contain 'attention': %s", r.Content)
	}
}

func TestRetrieveStructuredQuerySortOrder(t *testing.T) {
	store, tmpDir := testSetup(t)

	// Ingest two papers to verify cross-paper sort order.
	for _, pid := range []string{"aaa-paper", "zzz-paper"} {
		writeExtraction(t, tmpDir, pid, sampleItems(pid))
		writePaperMeta(t, tmpDir, types.Paper{ID: pid, Title: pid})
	}
	var buf strings.Builder
	store.Ingest(context.Background(), &buf)

	results, err := store.Retrieve(context.Background(), QueryOptions{Type: types.ItemClaim})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) < 2 {
		t.Fatal("expected at least 2 results")
	}
	// Structured queries are sorted by paper_id, section, page.
	if results[0].PaperID > results[len(results)-1].PaperID {
		t.Errorf("results not sorted by paper_id: first=%q last=%q",
			results[0].PaperID, results[len(results)-1].PaperID)
	}
}

func TestRetrieveEmptyQueryError(t *testing.T) {
	opts := QueryOptions{}
	if !opts.IsEmpty() {
		t.Error("empty QueryOptions should report IsEmpty() = true")
	}
}

func TestRetrieveNoResults(t *testing.T) {
	store, tmpDir := testSetup(t)
	ingestHelper(t, store, tmpDir, "empty-query-paper")

	results, err := store.Retrieve(context.Background(), QueryOptions{
		Query: "nonexistent topic xyz123",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("got %d results, want 0", len(results))
	}
}

// --- trace tests ---

func TestTrace(t *testing.T) {
	store, tmpDir := testSetup(t)
	ingestHelper(t, store, tmpDir, "trace-paper")

	writeMarkdown(t, tmpDir, "trace-paper", `## Abstract
<!-- page 1 -->
We propose a new attention mechanism.

## Method
<!-- page 2 -->
We define efficient attention as a linear approximation of softmax attention.
Our approach reduces computation from O(n^2) to O(n log n).

## Results
<!-- page 3 -->
On the GLUE benchmark, our method achieves 89.2% accuracy.
`)

	ctx := context.Background()
	// trace-paper-claim1 is in section "Method".
	text, err := store.Trace(ctx, "trace-paper-claim1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "efficient attention") {
		t.Errorf("trace should contain 'efficient attention': %s", text)
	}
	if !strings.Contains(text, "O(n log n)") {
		t.Errorf("trace should contain 'O(n log n)': %s", text)
	}
}

func TestTraceItemNotFound(t *testing.T) {
	store, _ := testSetup(t)

	_, err := store.Trace(context.Background(), "nonexistent-item")
	if err == nil {
		t.Fatal("expected error for nonexistent item")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want 'not found'", err.Error())
	}
}

func TestTraceMarkdownMissing(t *testing.T) {
	store, tmpDir := testSetup(t)
	ingestHelper(t, store, tmpDir, "missing-md-paper")

	// Do not write the Markdown file.
	_, err := store.Trace(context.Background(), "missing-md-paper-claim1")
	if err == nil {
		t.Fatal("expected error for missing Markdown")
	}
	if !strings.Contains(err.Error(), "missing-md-paper.md") {
		t.Errorf("error = %q, should reference the Markdown path", err.Error())
	}
}

// --- export tests ---

func TestExportYAML(t *testing.T) {
	store, tmpDir := testSetup(t)
	ingestHelper(t, store, tmpDir, "export-yaml-paper")

	if err := store.ExportYAML(context.Background(), QueryOptions{}); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(tmpDir, "knowledge", indexDir, "export.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var entries []ExportEntry
	if err := yaml.Unmarshal(data, &entries); err != nil {
		t.Fatalf("invalid YAML: %v", err)
	}
	if len(entries) != 4 {
		t.Errorf("got %d entries, want 4", len(entries))
	}
	// Verify paper metadata included.
	for _, e := range entries {
		if e.Paper == nil {
			t.Errorf("entry %s missing paper metadata", e.ID)
		}
	}
}

func TestExportJSON(t *testing.T) {
	store, tmpDir := testSetup(t)
	ingestHelper(t, store, tmpDir, "export-json-paper")

	if err := store.ExportJSON(context.Background(), QueryOptions{}); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(tmpDir, "knowledge", indexDir, "export.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var entries []ExportEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(entries) != 4 {
		t.Errorf("got %d entries, want 4", len(entries))
	}
}

func TestExportFilteredByType(t *testing.T) {
	store, tmpDir := testSetup(t)
	ingestHelper(t, store, tmpDir, "filtered-export")

	if err := store.ExportYAML(context.Background(), QueryOptions{Type: types.ItemMethod}); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(tmpDir, "knowledge", indexDir, "export.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var entries []ExportEntry
	yaml.Unmarshal(data, &entries)
	for _, e := range entries {
		if e.Type != string(types.ItemMethod) {
			t.Errorf("entry type = %q, want %q", e.Type, types.ItemMethod)
		}
	}
}

func TestExportFilteredByTag(t *testing.T) {
	store, tmpDir := testSetup(t)
	ingestHelper(t, store, tmpDir, "tag-export")

	if err := store.ExportJSON(context.Background(), QueryOptions{Tags: []string{"benchmark"}}); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(tmpDir, "knowledge", indexDir, "export.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var entries []ExportEntry
	json.Unmarshal(data, &entries)
	if len(entries) != 1 {
		t.Errorf("got %d entries, want 1 (only result has benchmark tag)", len(entries))
	}
	for _, e := range entries {
		found := false
		for _, tag := range e.Tags {
			if tag == "benchmark" {
				found = true
			}
		}
		if !found {
			t.Errorf("entry tags %v do not contain 'benchmark'", e.Tags)
		}
	}
}

// --- IngestSummary ---

func TestIngestSummaryTotal(t *testing.T) {
	s := IngestSummary{Indexed: 2, Updated: 1, Skipped: 3, Failed: 1}
	if s.Total() != 7 {
		t.Errorf("Total() = %d, want 7", s.Total())
	}
}

// --- extractSectionContext ---

func TestExtractSectionContext(t *testing.T) {
	md := `## Abstract
<!-- page 1 -->
We propose a new method.

## Method
<!-- page 2 -->
We define efficient attention as a linear approximation.
Our approach reduces computation.

## Results
<!-- page 3 -->
We achieve high accuracy.
`

	tests := []struct {
		section     string
		wantContain string
		wantMissing string
	}{
		{"Method", "efficient attention", "We propose"},
		{"Results", "high accuracy", "efficient attention"},
		{"Abstract", "We propose a new method", "efficient attention"},
		{"Nonexistent", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.section, func(t *testing.T) {
			result := extractSectionContext(md, tt.section)
			if tt.wantContain != "" && !strings.Contains(result, tt.wantContain) {
				t.Errorf("result should contain %q: got %q", tt.wantContain, result)
			}
			if tt.wantMissing != "" && strings.Contains(result, tt.wantMissing) {
				t.Errorf("result should not contain %q: got %q", tt.wantMissing, result)
			}
		})
	}
}
