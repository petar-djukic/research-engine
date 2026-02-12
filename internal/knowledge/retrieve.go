// Copyright Mesh Intelligence Inc., 2026. All rights reserved.

package knowledge

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pdiddy/research-engine/pkg/types"
)

// QueryOptions holds parameters for knowledge base queries (R2, R3).
type QueryOptions struct {
	// Query is the FTS5 full-text search string (R2.1).
	Query string

	// Type filters by KnowledgeItemType (R3.1).
	Type types.KnowledgeItemType

	// Tags filters by one or more tags with AND semantics (R3.2).
	Tags []string

	// PaperID filters by paper (R3.3).
	PaperID string

	// MaxResults limits result count. Zero uses store default (R2.3).
	MaxResults int
}

// IsEmpty reports whether the query has no search terms or filters.
func (q QueryOptions) IsEmpty() bool {
	return q.Query == "" && q.Type == "" && len(q.Tags) == 0 && q.PaperID == ""
}

// QueryResult is a KnowledgeItem with associated Paper metadata (R2.4).
type QueryResult struct {
	types.KnowledgeItem
	PaperTitle   string   `json:"paper_title" yaml:"paper_title"`
	PaperAuthors []string `json:"paper_authors" yaml:"paper_authors"`
}

// Retrieve queries the knowledge base with optional full-text search
// and structured filters (R2, R3). Results are ranked by relevance for
// full-text queries or sorted by paper_id, section, page for
// structured-only queries (R3.6).
func (s *Store) Retrieve(ctx context.Context, opts QueryOptions) ([]QueryResult, error) {
	maxResults := opts.MaxResults
	if maxResults <= 0 {
		maxResults = s.maxResults
	}

	var (
		qb     strings.Builder
		args   []any
		useFTS = opts.Query != ""
	)

	if useFTS {
		qb.WriteString(
			`SELECT i.id, i.type, i.content, i.paper_id, i.section, i.page,
				i.confidence, i.tags, i.citations,
				p.title, p.authors, items_fts.rank
			FROM items_fts
			JOIN items i ON i.rowid = items_fts.rowid
			LEFT JOIN papers p ON i.paper_id = p.id
			WHERE items_fts MATCH ?`)
		args = append(args, opts.Query)
	} else {
		qb.WriteString(
			`SELECT i.id, i.type, i.content, i.paper_id, i.section, i.page,
				i.confidence, i.tags, i.citations,
				p.title, p.authors, 0 AS rank
			FROM items i
			LEFT JOIN papers p ON i.paper_id = p.id
			WHERE 1=1`)
	}

	if opts.Type != "" {
		qb.WriteString(` AND i.type = ?`)
		args = append(args, string(opts.Type))
	}

	if opts.PaperID != "" {
		qb.WriteString(` AND i.paper_id = ?`)
		args = append(args, opts.PaperID)
	}

	for _, tag := range opts.Tags {
		qb.WriteString(` AND EXISTS (SELECT 1 FROM json_each(i.tags) WHERE value = ?)`)
		args = append(args, tag)
	}

	if useFTS {
		qb.WriteString(` ORDER BY items_fts.rank`)
	} else {
		qb.WriteString(` ORDER BY i.paper_id, i.section, i.page`)
	}

	qb.WriteString(` LIMIT ?`)
	args = append(args, maxResults)

	rows, err := s.db.QueryContext(ctx, qb.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("querying knowledge base: %w", err)
	}
	defer rows.Close()

	var results []QueryResult
	for rows.Next() {
		var (
			qr          QueryResult
			itemType    string
			tagsJSON    sql.NullString
			citJSON     sql.NullString
			paperTitle  sql.NullString
			authorsJSON sql.NullString
			rank        float64
		)

		if err := rows.Scan(
			&qr.ID, &itemType, &qr.Content, &qr.PaperID, &qr.Section, &qr.Page,
			&qr.Confidence, &tagsJSON, &citJSON,
			&paperTitle, &authorsJSON, &rank,
		); err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}

		qr.Type = types.KnowledgeItemType(itemType)

		if tagsJSON.Valid {
			json.Unmarshal([]byte(tagsJSON.String), &qr.Tags)
		}
		if citJSON.Valid {
			json.Unmarshal([]byte(citJSON.String), &qr.Citations)
		}
		if paperTitle.Valid {
			qr.PaperTitle = paperTitle.String
		}
		if authorsJSON.Valid {
			json.Unmarshal([]byte(authorsJSON.String), &qr.PaperAuthors)
		}

		results = append(results, qr)
	}

	return results, rows.Err()
}

// Trace returns the surrounding context from the source Markdown for a
// given item ID (R4.2, R4.3). It reads from papers/markdown/ using the
// item's paper_id and section to locate the source passage.
func (s *Store) Trace(ctx context.Context, itemID string) (string, error) {
	var paperID, section string
	var page int

	err := s.db.QueryRowContext(ctx,
		`SELECT paper_id, section, page FROM items WHERE id = ?`, itemID,
	).Scan(&paperID, &section, &page)

	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("item %s not found", itemID)
		}
		return "", fmt.Errorf("looking up item: %w", err)
	}

	mdPath := filepath.Join(s.papersDir, markdownDir, paperID+".md")
	content, err := os.ReadFile(mdPath)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", mdPath, err)
	}

	return extractSectionContext(string(content), section), nil
}

// extractSectionContext finds the named section in Markdown and returns
// its body text, stripping page markers.
func extractSectionContext(content, targetSection string) string {
	lines := strings.Split(content, "\n")
	var capturing bool
	var result []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "### ") {
			heading := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
			if heading == targetSection {
				capturing = true
				continue
			} else if capturing {
				break
			}
		}

		if capturing {
			if strings.HasPrefix(trimmed, "<!-- page") {
				continue
			}
			result = append(result, line)
		}
	}

	return strings.TrimSpace(strings.Join(result, "\n"))
}
