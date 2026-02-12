# Research Engine Architecture

## System Overview

We build a two-layer system for academic research. The bottom layer is a Go CLI that provides infrastructure: searching academic APIs, downloading PDFs, converting them to structured Markdown, extracting typed knowledge items, and indexing them in a local database. The top layer is Claude, operating through skills that drive the research workflow. Claude searches for papers, reads them, queries the knowledge base, and writes new work grounded in what it has read.

The core insight is that Claude can read, understand, and write, but it needs papers in a format it can process and a knowledge base to retrieve specific items across many papers. We convert PDFs to Markdown so Claude can read them. We extract knowledge items with provenance so Claude can find and cite specific claims without re-reading entire papers. The Go infrastructure handles the mechanical work; Claude handles the judgment.

### Two-Layer Architecture

|  |
|:--:|

```plantuml
@startuml
!theme plain
skinparam backgroundColor white
skinparam arrowColor #333333

package "Researcher" {
  [Researcher] as user
}

package "Claude (Skills Layer)" {
  [search-papers] as sp
  [acquire-papers] as ap
  [read-papers] as rp
  [query-knowledge] as qk
  [write-paper] as wp
}

package "Go CLI (Infrastructure Layer)" {
  [search] as srch
  [acquire] as acq
  [convert] as conv
  [extract] as ext
  [knowledge] as kb
}

package "Filesystem (Data Layer)" {
  [papers/] as papers
  [knowledge/] as know
  [output/papers/] as out
}

user --> sp
user --> ap
user --> rp
user --> qk
user --> wp

sp --> srch
ap --> acq
ap --> conv
ap --> ext
qk --> kb
wp --> kb

rp ..> papers : reads directly
wp ..> papers : reads directly

srch --> papers
acq --> papers
conv --> papers
ext --> know
kb --> know

wp --> out

@enduml
```

|Figure 1 Two-layer architecture showing Claude skills, Go CLI infrastructure, and filesystem |

### Pipeline Lifecycle

A paper moves through five infrastructure states, one per pipeline stage:

1. Searched: the paper appears as a candidate result from an academic API query.
2. Acquired: the raw PDF exists on disk with metadata (source URL, DOI, title).
3. Converted: the PDF has been transformed into structured Markdown with section boundaries.
4. Extracted: knowledge items have been pulled from the structured text and linked to their source sections.
5. Stored: knowledge items reside in the knowledge base, indexed for retrieval.

Paper writing is handled by Claude through the write-paper skill, not by a Go pipeline stage. Claude reads papers directly (via converted Markdown), queries the knowledge base, and writes sections with inline citations.

Each state is visible as files on disk. A paper's progress through the pipeline is determined by which artifacts exist in the project directory. PRDs define the detailed state transitions and error conditions for each stage.

### Data Flow

Data flows forward through the infrastructure pipeline. Each stage consumes the output of the previous stage and produces input for the next. The stages share no in-memory state; all communication happens through files on disk. Claude reads these files directly through skills.

|  |
|:--:|

```plantuml
@startuml
!theme plain
skinparam backgroundColor white
skinparam arrowColor #333333

rectangle "Search" as srch
rectangle "Acquisition" as acq
rectangle "Conversion" as conv
rectangle "Extraction" as ext
rectangle "Knowledge Base" as kb

srch -right-> acq : Identifiers + metadata
acq -right-> conv : PDF + metadata
conv -right-> ext : Structured Markdown
ext -right-> kb : Knowledge items

note bottom of srch : Research query\n-> candidate papers
note bottom of acq : Identifier/URL\n-> raw PDF
note bottom of conv : PDF\n-> sections, paragraphs
note bottom of ext : Markdown\n-> claims, methods
note bottom of kb : Items\n-> indexed store

@enduml
```

|Figure 2 Infrastructure pipeline data flow from search through knowledge base |

## Pipeline Interface

The pipeline operates on three data structures that flow between infrastructure stages. We describe them here at the architecture level; PRDs specify the full field definitions and validation rules.

### Data Structures

Table 1 Pipeline Data Structures

| Structure | Role | Produced By | Consumed By |
|-----------|------|-------------|-------------|
| SearchResult | A candidate paper from an academic API query (identifier, title, authors, abstract, source, relevance score) | Search | Acquisition (as input identifiers) |
| Paper | Metadata and file paths for an acquired paper (URL, DOI, title, authors, PDF path) | Acquisition | Conversion, Extraction |
| KnowledgeItem | A typed extraction from a paper (claim, method, definition) with provenance (paper ID, section, page) | Extraction | Knowledge Base, Claude (via skills) |

### Operations per Stage

Table 2 Pipeline Operations

| Stage | Operation | Input | Output | Description |
|-------|-----------|-------|--------|-------------|
| Search | Search | Research query (free-text or structured) | List of SearchResults | Queries academic APIs, deduplicates, and ranks candidate papers |
| Acquisition | Acquire | URL, DOI, or file path | Paper record + PDF on disk | Downloads or copies the PDF and creates metadata |
| Conversion | Convert | Paper record | Structured Markdown file | Transforms PDF into sections, paragraphs, and figure references |
| Extraction | Extract | Structured Markdown | List of KnowledgeItems | Identifies claims, methods, and definitions with provenance |
| Knowledge Base | Store | KnowledgeItems | Indexed knowledge base | Persists items and builds retrieval index |
| Knowledge Base | Retrieve | Query string | Ranked KnowledgeItems | Returns items relevant to the query |

Each operation is a Cobra subcommand under `research-engine`. PRDs define the full signatures, preconditions, postconditions, and error handling for each operation.

## Claude Skills

Claude drives the research workflow through five skills defined in `.claude/commands/`. Each skill combines Claude's ability to reason, read, and write with the Go CLI infrastructure. The researcher invokes skills through slash commands in Claude Code.

Table 3 Claude Skills

| Skill | Purpose | Infrastructure Used |
|-------|---------|-------------------|
| search-papers | Search for papers on a topic, recommend acquisitions | research-engine search |
| acquire-papers | Download papers, convert to Markdown, optionally extract | research-engine acquire, convert, extract |
| read-papers | Browse and read converted Markdown papers | Reads files directly (no CLI) |
| query-knowledge | Search the knowledge base, trace items to sources | research-engine knowledge retrieve |
| write-paper | Create paper projects, write sections with citations | research-engine knowledge retrieve, reads papers directly |

### search-papers

Claude accepts a research topic or question, formulates one or more queries, and invokes the search CLI. It interprets results, recommends which papers to acquire, and can proceed directly to acquisition. Claude adds value by reasoning about query formulation and result relevance.

### acquire-papers

Claude downloads papers and converts them to readable Markdown in a single action. It invokes the acquire and convert CLI commands, reports successes and failures, and optionally triggers extraction to populate the knowledge base. This combines three infrastructure stages into one researcher-facing action.

### read-papers

Claude reads converted Markdown papers directly using the Read tool. It can list available papers, present summaries, read specific sections, discuss findings, and compare across papers. This is the skill that makes the "Claude reads papers" vision concrete. No CLI command is needed; Claude reads the files on disk.

### query-knowledge

Claude queries the knowledge base to find extracted items across all papers. It invokes the retrieve CLI with queries and filters, presents results grouped by paper and type, and can trace items back to their source passages. Claude helps identify connections, gaps, and themes.

### write-paper

Claude creates structured paper projects and writes content iteratively. The workflow has four phases:

1. Initialize: create a project directory with a title page containing YAML frontmatter.
2. Outline: propose numbered section files with descriptions, based on available knowledge.
3. Write: compose section content with inline citations, maintaining a references file.
4. Refine: revise sections based on researcher feedback.

See PRD: Paper Writing for the project structure, file naming, and citation conventions.

## System Components

### Search

We query academic APIs to find papers relevant to a research question. The search component accepts free-text queries or structured parameters (author, keywords, date range), fans them out to multiple backends (arXiv API, Semantic Scholar API), deduplicates results across sources, and returns a ranked list of candidate papers. Each result carries an identifier that the acquisition stage can consume directly, along with title, authors, abstract, and a relevance score. The backend interface is extensible so we can add sources without modifying existing backend code.

See PRD: Paper Search for query interface, backend requirements, deduplication rules, and output format.

### Acquisition

We download papers from URLs, DOIs, or arXiv identifiers and store them locally. The acquisition component resolves identifiers to PDF download links, fetches the PDF, and creates a Paper metadata record. It handles arXiv abstract-to-PDF URL mapping, DOI resolution through content negotiation, OpenAlex open-access resolution, and direct PDF URLs. When a paper already exists on disk, acquisition skips the download and returns the existing record.

See PRD: Paper Acquisition for identifier resolution rules, retry behavior, and metadata fields.

### Conversion

We transform PDF files into structured Markdown that preserves document organization. The conversion component extracts text from PDF pages, identifies section headings and paragraph boundaries, and produces a Markdown file with hierarchical structure. It handles multi-column layouts, inline equations, and figure captions. The output Markdown includes page-number annotations so downstream stages can link back to specific pages. Papers are converted to Markdown so that Claude can read them directly.

See PRD: PDF Conversion for supported PDF formats, section detection heuristics, and output schema.

### Extraction

We identify typed knowledge items within converted text. The extraction component reads structured Markdown and produces a list of KnowledgeItems, each categorized as a claim, method, definition, or result. Every item carries provenance: the paper ID, section heading, and page number where it appears. Extraction uses Generative AI to classify and summarize items while preserving the source language for verification. Claude can also read papers directly for deeper understanding beyond what batch extraction captures.

See PRD: Knowledge Extraction for item types, provenance requirements, and extraction prompts.

### Knowledge Base

We store extracted knowledge items and make them retrievable by topic. The knowledge base component persists KnowledgeItems to SQLite with FTS5 full-text indexing and supports structured queries by type, tag, and paper. A researcher (or Claude through the query-knowledge skill) queries the knowledge base and receives ranked items with their provenance. The storage format supports human-readable YAML/JSON export for version control.

See PRD: Knowledge Base for storage schema, indexing approach, and retrieval ranking.

## Design Decisions

### Decision 1 File-Based Inter-Stage Communication

We pass data between pipeline stages through files on disk rather than in-memory channels or a database. Each stage reads input files and writes output files in well-defined formats (PDF, Markdown, JSON/YAML). This makes intermediate results inspectable, debuggable, and version-controllable. A researcher can examine or edit any artifact between stages. Claude reads these same files through skills.

Benefits: transparency, reproducibility, ability to re-run individual stages, compatibility with version control. The tradeoff is that file I/O adds latency, but for a personal tool processing tens of papers, throughput is not the bottleneck.

### Decision 2 Cobra CLI with Skill-Based Composition

We expose each infrastructure stage as a Cobra subcommand (`research-engine search`, `research-engine acquire`, `research-engine convert`, etc.). Claude skills compose multi-stage workflows by invoking CLI commands and reading files directly. The researcher interacts primarily through Claude skills rather than invoking CLI commands directly.

Benefits: the CLI provides a stable, testable interface that Claude can invoke; skills add reasoning and judgment on top of mechanical operations; the researcher gets a conversational interface rather than a command-line one.

### Decision 3 Go Over Python

We implement the infrastructure in Go rather than Python. Go produces static binaries that a researcher can install without managing virtual environments or dependency conflicts. Go's type system catches errors at compile time that Python would surface only at runtime. The research tooling ecosystem in Python is richer, but we invoke external tools (PDF converters, Generative AI APIs) through subprocesses and HTTP, so the implementation language matters less than the interfaces.

Benefits: static binaries, compile-time safety, single-binary distribution. The tradeoff is fewer research-focused libraries, which we mitigate by calling external tools.

### Decision 4 Local-First with Selective Internet Access

We store all data locally and require network access for four activities: search (academic APIs), acquisition (paper downloads), extraction (Claude API for batch processing), and writing (Claude API through skills). Conversion and knowledge base storage run fully offline. The researcher owns their data and can inspect every file.

Benefits: data ownership, privacy, offline operation for conversion and storage, version-controllable artifacts.

### Decision 5 Generative AI for Extraction and Generation

We use Generative AI (via API calls to Claude) for the extraction stage and for paper writing through skills. Research papers vary too widely in structure and language for hand-coded extraction rules to generalize. Generative AI handles the ambiguity of natural language while we enforce structure through typed output schemas.

Benefits: handles diverse paper formats, produces readable output. The risk of hallucination is mitigated by requiring provenance links and researcher review. See VISION.md Risks and Mitigations.

### Decision 6 Claude as Researcher

We position Claude as the researcher's partner rather than as a backend API. Claude drives the research workflow through skills: it searches for papers, reads their content, queries the knowledge base, and writes new work. The Go CLI provides infrastructure that Claude invokes. This separates concerns: Go handles deterministic, repeatable operations (search, download, convert, index); Claude handles work that requires judgment (reading comprehension, finding connections, writing prose).

Benefits: leverages Claude's full capabilities (reasoning, reading, writing) rather than reducing it to API calls for classification. The researcher gets a conversational research partner rather than a command-line tool. The tradeoff is dependence on Claude's context window for long papers, which we mitigate by supporting section-by-section reading and the knowledge base for cross-paper retrieval.

## Technology Choices

Table 4 Technology Choices

| Component | Technology | Purpose |
|-----------|-----------|---------|
| Implementation language | Go | Infrastructure implementation, CLI, type safety |
| Build automation | Mage | Build automation, testing, developer tooling, paper compilation |
| Academic search | arXiv API, Semantic Scholar API | Query academic sources for candidate papers |
| Open-access resolution | OpenAlex API | Prefer open-access PDF downloads for DOI identifiers |
| PDF conversion | MarkItDown (container-based) | Transform PDF to structured Markdown |
| Knowledge storage | SQLite with FTS5 | Full-text indexed knowledge base with structured queries |
| Knowledge export | YAML/JSON files | Human-readable, version-controllable item export |
| Generative AI | Claude API (Anthropic) | Extraction classification, paper writing through skills |
| Research interface | Claude Code skills | Five skills that drive the research workflow |
| CLI framework | Cobra | Infrastructure command-line interface |
| Configuration | Viper | CLI configuration and project settings |
| Testing | Go testing + testify | Unit and integration tests |
| Paper compilation | Pandoc (external) | Compile paper projects to PDF via Mage target |

PRDs for each stage specify the exact tool versions and configuration.

## Project Structure

```
research-engine/
  .claude/
    commands/                # Claude research skills
      search-papers.md       # Search for academic papers
      acquire-papers.md      # Download and convert papers
      read-papers.md         # Browse and read converted papers
      query-knowledge.md     # Search the knowledge base
      write-paper.md         # Create and refine papers
    rules/                   # Project conventions and format rules
  cmd/
    research-engine/         # CLI entry point and subcommands
      main.go
      search.go              # research-engine search subcommand
      acquire.go             # research-engine acquire subcommand
      convert.go             # research-engine convert subcommand
      extract.go             # research-engine extract subcommand
      knowledge.go           # research-engine knowledge subcommand
  internal/
    search/                  # Search stage implementation
    acquire/                 # Acquisition stage implementation
    convert/                 # PDF conversion stage
    extract/                 # Knowledge extraction stage
    knowledge/               # Knowledge base storage and retrieval
    container/               # Container runtime abstraction
  pkg/
    types/                   # Shared types: Paper, KnowledgeItem, SearchResult, Config
  magefiles/
    magefile.go              # Build automation, stats, paper compilation
  tests/
    integration/             # End-to-end pipeline tests
  docs/
    VISION.md
    ARCHITECTURE.md
    specs/
      product-requirements/
      use-cases/
      test-suites/
  papers/                    # Working directory for acquired papers (per-project)
  knowledge/                 # Working directory for knowledge base (per-project)
  output/
    papers/                  # Paper projects written by Claude
```

Table 5 Package Roles

| Directory | Role |
|-----------|------|
| .claude/commands/ | Claude research skills. The researcher's primary interface to the tool. |
| cmd/research-engine/ | CLI entry point and Cobra subcommands. Infrastructure interface. |
| internal/search/ | Queries academic APIs, deduplicates and ranks candidate papers. |
| internal/acquire/ | Downloads papers, resolves identifiers, creates Paper records. |
| internal/convert/ | Invokes PDF conversion tools, produces structured Markdown. |
| internal/extract/ | Calls Generative AI to classify and extract KnowledgeItems. |
| internal/knowledge/ | Persists KnowledgeItems, builds and queries the retrieval index. |
| internal/container/ | Container runtime abstraction (Docker and Podman support). |
| pkg/types/ | Shared data structures: SearchResult, Paper, KnowledgeItem, Config. |
| magefiles/ | Build automation, stats, paper compilation. No pipeline stage logic. |
| tests/integration/ | Tests that run multiple stages end-to-end. |

## Implementation Status

We have completed the Foundation, Core Pipeline, and Knowledge phases. The Skills phase is in progress.

Implemented packages:

- `internal/search/` — arXiv and Semantic Scholar backends, deduplication, CSL YAML output, query file persistence
- `internal/acquire/` — identifier resolution (arXiv, DOI, direct URL, OpenAlex), PDF download with retry and rate limiting
- `internal/convert/` — PDF-to-Markdown conversion via MarkItDown in a container runtime
- `internal/container/` — container runtime abstraction (Docker and Podman support)
- `internal/extract/` — AI-based knowledge extraction with citation graph and tagging
- `internal/knowledge/` — SQLite + FTS5 knowledge base with store, retrieve, trace, and export

Table 6 Implementation Phases

| Phase | Status | Notes |
|-------|--------|-------|
| Foundation | Done | VISION, ARCHITECTURE, PRDs for all stages, Cobra CLI scaffolding |
| Core Pipeline | Done | Search, Acquisition, and Conversion stages implemented |
| Knowledge | Done | Extraction and Knowledge Base stages implemented |
| Skills | In progress | Claude research skills, paper writing workflow, Mage compile target |

## Related Documents

Table 7 Related Documents

| Document | Role |
|----------|------|
| VISION.md | Project purpose, success criteria, and boundaries |
| PRD: Paper Search | Requirements for querying academic APIs and ranking results |
| PRD: Paper Acquisition | Requirements for downloading and resolving papers |
| PRD: PDF Conversion | Requirements for transforming PDFs to structured text |
| PRD: Knowledge Extraction | Requirements for identifying typed knowledge items |
| PRD: Knowledge Base | Requirements for storage, indexing, and retrieval |
| PRD: Paper Writing | Requirements for Claude-driven paper writing workflow (supersedes PRD: Paper Generation) |

## References

See VISION.md for goals, success criteria, and explicit boundaries. See PRDs in docs/specs/product-requirements/ for detailed requirements per stage.
