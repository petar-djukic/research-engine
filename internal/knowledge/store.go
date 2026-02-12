// Copyright Mesh Intelligence Inc., 2026. All rights reserved.

// Package knowledge persists KnowledgeItems and builds a retrieval index.
// Implements: prd004-knowledge-base (R1-R6);
//
//	docs/ARCHITECTURE § Knowledge Base.
package knowledge

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"go.yaml.in/yaml/v3"

	"github.com/pdiddy/research-engine/pkg/types"
)

const (
	extractedDir = "extracted"
	indexDir     = "index"
	metadataDir  = "metadata"
	markdownDir  = "markdown"
	dbFile       = "research.db"
)

// Store manages the knowledge base SQLite database.
type Store struct {
	db           *sql.DB
	knowledgeDir string
	papersDir    string
	maxResults   int
}

// NewStore opens or creates the knowledge base SQLite database at
// knowledgeDir/index/research.db. It creates the schema if it does not
// exist (R1.2, R1.3).
func NewStore(cfg types.KnowledgeBaseConfig, papersDir string) (*Store, error) {
	dbDir := filepath.Join(cfg.KnowledgeDir, indexDir)
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating index directory: %w", err)
	}

	dbPath := filepath.Join(dbDir, dbFile)
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	maxResults := cfg.MaxResults
	if maxResults <= 0 {
		maxResults = 20
	}

	s := &Store{
		db:           db,
		knowledgeDir: cfg.KnowledgeDir,
		papersDir:    papersDir,
		maxResults:   maxResults,
	}

	if err := s.createSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("creating schema: %w", err)
	}

	return s, nil
}

// Close releases the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) createSchema() error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS papers (
			id TEXT PRIMARY KEY,
			title TEXT,
			authors TEXT,
			date TEXT,
			abstract TEXT,
			source_url TEXT,
			pdf_path TEXT,
			conversion_status TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS items (
			rowid INTEGER PRIMARY KEY AUTOINCREMENT,
			id TEXT NOT NULL UNIQUE,
			type TEXT NOT NULL,
			content TEXT NOT NULL,
			paper_id TEXT NOT NULL REFERENCES papers(id),
			section TEXT,
			page INTEGER,
			confidence REAL,
			tags TEXT,
			citations TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_items_paper_id ON items(paper_id)`,
		`CREATE INDEX IF NOT EXISTS idx_items_type ON items(type)`,
		`CREATE TABLE IF NOT EXISTS indexing_status (
			paper_id TEXT PRIMARY KEY,
			file_mod_time TEXT
		)`,
	}

	for _, stmt := range statements {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("executing schema statement: %w", err)
		}
	}

	// FTS5 virtual table with triggers for sync.
	var ftsExists int
	if err := s.db.QueryRow(
		`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='items_fts'`,
	).Scan(&ftsExists); err != nil {
		return fmt.Errorf("checking FTS table: %w", err)
	}

	if ftsExists == 0 {
		ftsStatements := []string{
			`CREATE VIRTUAL TABLE items_fts USING fts5(content, content=items, content_rowid=rowid)`,
			`CREATE TRIGGER items_ai AFTER INSERT ON items BEGIN
				INSERT INTO items_fts(rowid, content) VALUES (new.rowid, new.content);
			END`,
			`CREATE TRIGGER items_ad AFTER DELETE ON items BEGIN
				INSERT INTO items_fts(items_fts, rowid, content) VALUES('delete', old.rowid, old.content);
			END`,
			`CREATE TRIGGER items_au AFTER UPDATE ON items BEGIN
				INSERT INTO items_fts(items_fts, rowid, content) VALUES('delete', old.rowid, old.content);
				INSERT INTO items_fts(rowid, content) VALUES (new.rowid, new.content);
			END`,
		}
		for _, stmt := range ftsStatements {
			if _, err := s.db.Exec(stmt); err != nil {
				return fmt.Errorf("creating FTS infrastructure: %w", err)
			}
		}
	}

	return nil
}

// IngestSummary holds counts from a knowledge base indexing run (R5.5).
type IngestSummary struct {
	Indexed int
	Updated int
	Skipped int
	Failed  int
}

// Total returns the number of papers processed.
func (s IngestSummary) Total() int {
	return s.Indexed + s.Updated + s.Skipped + s.Failed
}

// Ingest reads extraction YAML files from knowledgeDir/extracted/ and
// populates the database. It detects new, changed, and unchanged files
// for incremental updates (R1.1, R5.1-R5.5). On success it writes
// export.yaml (R1.6).
func (s *Store) Ingest(ctx context.Context, w io.Writer) (IngestSummary, error) {
	extractDir := filepath.Join(s.knowledgeDir, extractedDir)
	metaDir := filepath.Join(s.papersDir, metadataDir)

	entries, err := os.ReadDir(extractDir)
	if err != nil {
		return IngestSummary{}, fmt.Errorf("reading extraction directory %s: %w", extractDir, err)
	}

	var summary IngestSummary

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "-items.yaml") {
			continue
		}

		select {
		case <-ctx.Done():
			return summary, ctx.Err()
		default:
		}

		paperID := strings.TrimSuffix(entry.Name(), "-items.yaml")
		filePath := filepath.Join(extractDir, entry.Name())

		info, err := entry.Info()
		if err != nil {
			fmt.Fprintf(w, "failed  %s: %v\n", paperID, err)
			summary.Failed++
			continue
		}
		modTime := info.ModTime().UTC().Format(time.RFC3339Nano)

		// Check whether the file has changed since last indexing (R5.1, R5.3).
		var storedModTime string
		err = s.db.QueryRowContext(ctx,
			`SELECT file_mod_time FROM indexing_status WHERE paper_id = ?`, paperID,
		).Scan(&storedModTime)

		if err == nil && storedModTime == modTime {
			fmt.Fprintf(w, "skipped %s\n", paperID)
			summary.Skipped++
			continue
		}

		isUpdate := err == nil

		data, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Fprintf(w, "failed  %s: %v\n", paperID, err)
			summary.Failed++
			continue
		}

		var result types.ExtractionResult
		if err := yaml.Unmarshal(data, &result); err != nil {
			fmt.Fprintf(w, "failed  %s: parse error: %v\n", paperID, err)
			summary.Failed++
			continue
		}

		paper := loadPaperMetadata(metaDir, paperID)

		if err := s.ingestPaper(ctx, paperID, &result, paper, modTime, isUpdate); err != nil {
			fmt.Fprintf(w, "failed  %s: %v\n", paperID, err)
			summary.Failed++
			continue
		}

		if isUpdate {
			fmt.Fprintf(w, "updated %s (%d items)\n", paperID, len(result.Items))
			summary.Updated++
		} else {
			fmt.Fprintf(w, "indexing %s (%d items)\n", paperID, len(result.Items))
			summary.Indexed++
		}
	}

	fmt.Fprintf(w, "\nindexed: %d, updated: %d, skipped: %d, failed: %d\n",
		summary.Indexed, summary.Updated, summary.Skipped, summary.Failed)

	// Write export.yaml after successful ingestion (R1.6).
	if summary.Indexed > 0 || summary.Updated > 0 {
		if err := s.ExportYAML(ctx, QueryOptions{}); err != nil {
			fmt.Fprintf(w, "warning: export.yaml write failed: %v\n", err)
		}
	}

	return summary, nil
}

func (s *Store) ingestPaper(ctx context.Context, paperID string, result *types.ExtractionResult, paper *types.Paper, modTime string, isUpdate bool) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	// Remove old items if updating (R5.2).
	if isUpdate {
		if _, err := tx.ExecContext(ctx, `DELETE FROM items WHERE paper_id = ?`, paperID); err != nil {
			return fmt.Errorf("deleting old items: %w", err)
		}
	}

	// Upsert paper record (R1.5).
	if paper != nil {
		authorsJSON, _ := json.Marshal(paper.Authors)
		dateStr := ""
		if !paper.Date.IsZero() {
			dateStr = paper.Date.Format(time.RFC3339)
		}
		_, err := tx.ExecContext(ctx,
			`INSERT INTO papers (id, title, authors, date, abstract, source_url, pdf_path, conversion_status)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT(id) DO UPDATE SET
				title=excluded.title, authors=excluded.authors, date=excluded.date,
				abstract=excluded.abstract, source_url=excluded.source_url,
				pdf_path=excluded.pdf_path, conversion_status=excluded.conversion_status`,
			paper.ID, paper.Title, string(authorsJSON), dateStr,
			paper.Abstract, paper.SourceURL, paper.PDFPath, string(paper.ConversionStatus),
		)
		if err != nil {
			return fmt.Errorf("upserting paper: %w", err)
		}
	} else {
		_, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO papers (id) VALUES (?)`, paperID,
		)
		if err != nil {
			return fmt.Errorf("inserting paper stub: %w", err)
		}
	}

	// Insert items (R1.4).
	stmt, err := tx.PrepareContext(ctx,
		`INSERT OR REPLACE INTO items (id, type, content, paper_id, section, page, confidence, tags, citations)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("preparing insert: %w", err)
	}
	defer stmt.Close()

	for _, item := range result.Items {
		tagsJSON, _ := json.Marshal(item.Tags)
		citationsJSON, _ := json.Marshal(item.Citations)
		_, err := stmt.ExecContext(ctx,
			item.ID, string(item.Type), item.Content, item.PaperID,
			item.Section, item.Page, item.Confidence,
			string(tagsJSON), string(citationsJSON),
		)
		if err != nil {
			return fmt.Errorf("inserting item %s: %w", item.ID, err)
		}
	}

	// Update indexing status (R5.1).
	_, err = tx.ExecContext(ctx,
		`INSERT INTO indexing_status (paper_id, file_mod_time) VALUES (?, ?)
		 ON CONFLICT(paper_id) DO UPDATE SET file_mod_time=excluded.file_mod_time`,
		paperID, modTime,
	)
	if err != nil {
		return fmt.Errorf("updating indexing status: %w", err)
	}

	return tx.Commit()
}

// loadPaperMetadata reads a Paper record from metaDir/[paperID].yaml.
// Returns nil if the file does not exist or cannot be parsed.
func loadPaperMetadata(metaDir, paperID string) *types.Paper {
	path := filepath.Join(metaDir, paperID+".yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var paper types.Paper
	if err := yaml.Unmarshal(data, &paper); err != nil {
		return nil
	}
	return &paper
}
