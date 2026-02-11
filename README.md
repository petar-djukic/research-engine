# Research Engine

A personal research pipeline that takes academic papers from search through knowledge extraction to new paper generation. The system is a Go CLI that exposes each pipeline stage as a Cobra subcommand, designed to run locally on a single machine. The pipeline begins with search: the researcher describes a topic, and the system finds relevant papers across arXiv, Semantic Scholar, and other sources. Search results feed into acquisition, which downloads PDFs. From there, each stage transforms data into a more useful form: raw PDFs become structured text, structured text becomes extracted knowledge, and extracted knowledge feeds into new writing.

See [VISION.md](docs/VISION.md) for project goals and boundaries. See [ARCHITECTURE.md](docs/ARCHITECTURE.md) for system design and data flow.

## Prerequisites

- **Go 1.25+** — implementation language and build tool
- **Container runtime** (Docker or Podman) — required for PDF conversion (markitdown backend)
- **Claude API key** — required for extraction and generation stages (set `ANTHROPIC_API_KEY` environment variable)

### Install Go

This guide assumes macOS with [Homebrew](https://brew.sh) installed.

```bash
brew install go
```

Verify:

```bash
go version
# go version go1.25.x darwin/arm64
```

Make sure `$GOPATH/bin` (typically `~/go/bin`) is on your PATH. Add this to your shell profile (`~/.zshrc` or `~/.bashrc`) if it is not already there:

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

### Container Runtime

The conversion stage uses a container to run MarkItDown for PDF-to-Markdown conversion. Install Docker or Podman:

```bash
brew install --cask docker   # Docker Desktop
# or
brew install podman           # Podman
```

The CLI auto-detects which runtime is available.

## Build

```bash
go build -o research-engine ./cmd/research-engine/
```

Or install directly:

```bash
go install ./cmd/research-engine/
```

Verify:

```bash
./research-engine version
```

## Pipeline Stages

The pipeline has six stages, each exposed as a Cobra subcommand. The first three are implemented; the rest are planned.

| Stage | Command | Status | PRD |
|-------|---------|--------|-----|
| Search | `research-engine search` | Implemented | [prd006-search](docs/specs/product-requirements/prd006-search.yaml) |
| Acquisition | `research-engine acquire` | Implemented | [prd001-acquisition](docs/specs/product-requirements/prd001-acquisition.yaml) |
| Conversion | `research-engine convert` | Implemented | [prd002-conversion](docs/specs/product-requirements/prd002-conversion.yaml) |
| Extraction | `research-engine extract` | Planned | [prd003-extraction](docs/specs/product-requirements/prd003-extraction.yaml) |
| Knowledge Base | `research-engine store` | Planned | [prd004-knowledge-base](docs/specs/product-requirements/prd004-knowledge-base.yaml) |
| Generation | `research-engine generate` | Planned | [prd005-generation](docs/specs/product-requirements/prd005-generation.yaml) |

### Search

Search queries arXiv and Semantic Scholar for papers matching a research question.

```bash
research-engine search "transformer attention mechanisms"
research-engine search --query "neural networks" --author "Hinton" --max-results 10
research-engine search --keywords "LLM,reasoning" --from 2025-01-01 --json
research-engine search --query "diffusion models" --query-file results.yaml
research-engine search --query-file results.yaml   # reload saved results
research-engine search --query "attention" --csl    # CSL YAML output
```

Flags:

| Flag | Description |
|------|-------------|
| `--query` | Free-text research question |
| `--author` | Filter by author name |
| `--keywords` | Filter by keywords (comma-separated) |
| `--from` | Publication date range start (YYYY-MM-DD) |
| `--to` | Publication date range end (YYYY-MM-DD) |
| `--max-results` | Maximum number of results (default 20) |
| `--json` | Output results as JSON |
| `--csl` | Output results as CSL YAML for reference managers |
| `--recency-bias` | Boost recently published papers |
| `--query-file` | YAML file to save/load query and results |

### Acquire

Acquire downloads papers from arXiv IDs, DOIs, or direct PDF URLs.

```bash
research-engine acquire 2301.07041
research-engine acquire "10.1038/s41586-021-03819-2"
research-engine acquire https://example.com/paper.pdf
research-engine acquire 2301.07041 2305.14314 --timeout 2m --delay 2s
```

Flags:

| Flag | Description |
|------|-------------|
| `--timeout` | HTTP request timeout (default 60s) |
| `--delay` | Delay between consecutive downloads (default 1s) |
| `--papers-dir` | Base directory for papers (default "papers") |

### Convert

Convert transforms downloaded PDFs into structured Markdown.

```bash
research-engine convert papers/raw/2301.07041.pdf
research-engine convert --batch                        # convert all unconverted PDFs
research-engine convert --backend markitdown paper.pdf # explicit backend
```

Flags:

| Flag | Description |
|------|-------------|
| `--backend` | Conversion backend (default "markitdown") |
| `--papers-dir` | Base directory for papers (default "papers") |
| `--batch` | Process all unconverted papers in papers-dir |

## Project Structure

```text
research-engine/
  cmd/research-engine/    CLI entry point and Cobra subcommands
  internal/               Private implementation (one package per pipeline stage)
  pkg/types/              Shared data structures (Paper, SearchResult, KnowledgeItem)
  magefiles/              Build automation and developer tooling
  docs/                   VISION, ARCHITECTURE, PRDs, use cases
  papers/                 Working directory for acquired papers (per-project)
  knowledge/              Working directory for knowledge base (per-project)
  output/                 Working directory for generated drafts (per-project)
```

## Development

Run tests:

```bash
go test ./...
```

Build automation (requires [Mage](https://magefile.org)):

```bash
go run github.com/magefile/mage@latest -l   # list available targets
go run github.com/magefile/mage@latest stats # project statistics
```
