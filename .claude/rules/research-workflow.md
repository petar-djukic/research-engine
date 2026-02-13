<!-- Copyright Mesh Intelligence Inc., 2026. All rights reserved. -->

# Research Workflow

The research engine provides a Go CLI (`research-engine`) and a filesystem-based data layout for academic paper research. Claude operates as the researcher's partner, choosing which commands to run and which files to read based on what the researcher asks for. This rule describes the available tools, file conventions, and configuration so Claude can act on any research request.

## CLI Commands

All commands share a global `--config` flag for specifying a config file (default: `./research-engine.yaml` or `~/.config/research-engine/config.yaml`).

### search

We search academic APIs (arXiv, Semantic Scholar, OpenAlex) and the PatentsView patent database for papers and patents matching a research question. Results are deduplicated across sources and ranked by relevance.

Table 1 Search Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `query` (positional or `--query`) | string | | Free-text research question |
| `--author` | string | | Filter by author name |
| `--keywords` | string | | Filter by keywords (comma-separated) |
| `--from` | string | | Publication date range start (YYYY-MM-DD) |
| `--to` | string | | Publication date range end (YYYY-MM-DD) |
| `--max-results` | int | 20 | Maximum results to return |
| `--json` | bool | false | Output as JSON for programmatic parsing |
| `--csl` | bool | false | Output as CSL YAML for Pandoc and reference managers |
| `--recency-bias` | bool | false | Boost recently published papers |
| `--patents` | bool | false | Search only PatentsView (disables academic backends) |
| `--patentsview-api-key` | string | | PatentsView API key (also loaded from `.secrets/patentsview-api-key`) |
| `--query-file` | string | | YAML file to save or reload query and results |

When the PatentsView API key is configured, patent results appear alongside academic results automatically. Use `--patents` to search only PatentsView. Use `--query-file` without a query to reload saved results.

### acquire

We download PDFs and create metadata records from paper or patent identifiers. Existing papers are skipped.

Table 2 Acquire Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| identifiers (positional) | strings | | One or more identifiers to acquire |
| `--papers-dir` | string | `papers` | Base directory for papers |
| `--timeout` | duration | 60s | HTTP request timeout |
| `--delay` | duration | 1s | Delay between consecutive downloads |

Table 3 Supported Identifier Types

| Type | Format | Example |
|------|--------|---------|
| arXiv ID | digits with dot | `2301.01234` or `arxiv:2301.01234` |
| DOI | 10.prefix/suffix | `10.1234/example` or `doi:10.1234/example` |
| US patent | US prefix + digits + optional kind code | `US7654321`, `US7654321B2`, `US20230012345A1` |
| Direct URL | HTTPS URL to PDF | `https://example.com/paper.pdf` |

Patent identifiers are auto-detected by their format. No `--type` flag is needed. Identifiers of different types can be mixed in one command.

### convert

We transform PDF files into structured Markdown that preserves section hierarchy, paragraphs, and reference lists. Conversion requires a container runtime (Docker or Podman) for the markitdown backend.

Table 4 Convert Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| papers (positional) | strings | | Specific PDF paths to convert |
| `--backend` | string | `markitdown` | Conversion backend: `grobid`, `pdftotext`, or `markitdown` |
| `--batch` | bool | false | Process all unconverted papers in papers-dir |
| `--papers-dir` | string | `papers` | Base directory for papers |

### extract

We read structured Markdown and produce typed knowledge items (claims, methods, definitions, results) with provenance links back to the source paper, section, and page. Extraction calls the Claude API and costs tokens.

Table 5 Extract Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| papers (positional) | strings | | Specific paper IDs to extract |
| `--batch` | bool | false | Process all unextracted papers in papers-dir |
| `--model` | string | | AI model identifier for extraction |
| `--api-key` | string | | API key for the AI backend (or set `RESEARCH_ENGINE_EXTRACTION_API_KEY`) |
| `--papers-dir` | string | `papers` | Base directory for papers (contains `markdown/`) |
| `--knowledge-dir` | string | `knowledge` | Base directory for knowledge output (contains `extracted/`) |

Configuration priority for API key: CLI flag, config file, environment variable (`RESEARCH_ENGINE_EXTRACTION_API_KEY`), secrets directory (`.secrets/anthropic-api-key`).

### knowledge

We manage a local SQLite knowledge base built from extracted knowledge items. The `knowledge` command has three subcommands and shared flags.

Table 6 Knowledge Shared Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--knowledge-dir` | string | `knowledge` | Base directory for knowledge (contains `extracted/`, `index/`) |
| `--papers-dir` | string | `papers` | Base directory for papers (contains `metadata/`, `markdown/`) |
| `--max-results` | int | 20 | Maximum query results |

#### knowledge store

We ingest extraction YAML files from `knowledge/extracted/` into a SQLite database with FTS5 indexing. Unchanged papers are skipped on subsequent runs. No additional flags beyond the shared ones.

#### knowledge retrieve

We query the knowledge base using FTS5 full-text search, structured filters, or a combination of both.

Table 7 Retrieve Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| query (positional or `--query`) | string | | Full-text search query |
| `--type` | string | | Filter by item type: `claim`, `method`, `definition`, `result` |
| `--tag` | string | | Filter by tag |
| `--paper` | string | | Filter by paper ID |
| `--limit` | int | 0 (use `--max-results`) | Maximum results |
| `--trace` | string | | Show source context for a specific item ID |
| `--json` | bool | false | Output as JSON for detailed parsing |

Query modes: full-text search (`--query`), type filter (`--type`), tag filter (`--tag`), paper filter (`--paper`), trace (`--trace`), or any combination of text and filters.

#### knowledge export

We export the knowledge base (or a filtered subset) to `knowledge/index/export.yaml` or `export.json`.

Table 8 Export Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--format` | string | `yaml` | Export format: `yaml` or `json` |
| `--query` | string | | Full-text search filter for partial export |
| `--type` | string | | Filter by item type |
| `--tag` | string | | Filter by tag |
| `--paper` | string | | Filter by paper ID |
| `--limit` | int | 0 (all) | Maximum items to export |

### Exit Codes

All commands exit 0 on success and non-zero on failure. Non-zero exits include a descriptive error message on stderr.

## Filesystem Layout

The research engine uses three top-level directories with a pipeline state model: a paper's state is determined by which files exist.

Table 9 Directory Layout

| Directory | Contents | State Indicated |
|-----------|----------|-----------------|
| `papers/raw/` | Downloaded PDF files | Acquired |
| `papers/metadata/` | YAML metadata per paper (title, authors, DOI, source) | Acquired |
| `papers/markdown/` | Converted Markdown files | Converted |
| `knowledge/extracted/` | YAML extraction output (`PAPER-ID-items.yaml`) | Extracted |
| `knowledge/index/` | SQLite database and export files | Indexed |
| `output/papers/` | Paper projects created during writing | Written |

Reading papers requires no CLI: read Markdown files directly from `papers/markdown/PAPER-ID.md`. Read metadata from `papers/metadata/PAPER-ID.yaml` for title, authors, date, DOI, and source URL.

## Paper Project Conventions

Each paper project lives in `output/papers/[slug]/` where slug is a URL-safe directory name. See prd007-paper-writing for the complete specification.

Table 10 Paper Project Files

| File | Purpose |
|------|---------|
| `00-title-page.md` | YAML frontmatter: title, authors, date, type, abstract, keywords |
| `NN-slug.md` | Numbered section files (two-digit prefix, 00 reserved for title page) |
| `outline.yaml` | Section tracking: number, title, file, description, status (`outline`, `draft`, `revised`) |
| `references.yaml` | Cited papers: citation_key, paper_id, title, authors, year, venue |

### Title Page Frontmatter

```yaml
---
title: "Paper Title"
authors:
  - name: "Author Name"
    affiliation: "Institution"
date: YYYY-MM-DD
type: survey  # survey, literature-review, original-research, position-paper
abstract: ""
keywords:
  - keyword1
---
```

### Citation Format

Inline citations use square brackets with AuthorYear keys. Multiple citations use semicolons: `[Vaswani2017; Tay2022]`. Every citation key must have a matching entry in `references.yaml` with `citation_key`, `paper_id`, `title`, `authors`, `year`, and `venue` fields.

### Outline Structure

```yaml
sections:
  - number: "01"
    title: Introduction
    file: 01-introduction.md
    description: "Motivates the paper and previews the structure."
    status: outline
```

## Secrets and Configuration

### Secrets Directory

The `.secrets/` directory stores one file per secret key. The filename is the key name; the file contents are the value (no trailing newline).

Table 11 Supported Secret Keys

| Secret Key | Used By |
|------------|---------|
| `anthropic-api-key` | `extract` (AI extraction backend) |
| `semantic-scholar-api-key` | `search` (Semantic Scholar API) |
| `openalex-email` | `search` (OpenAlex polite pool) |
| `patentsview-api-key` | `search` (PatentsView API) |

### Configuration Priority

We resolve configuration values in this order (first match wins):

1. CLI flags
2. Config file (`research-engine.yaml`)
3. Environment variables (prefix `RESEARCH_ENGINE_`)
4. Secrets directory (`.secrets/`)

Missing secrets are not errors. Commands that need them fail with descriptive messages explaining which secret is missing and how to provide it.
