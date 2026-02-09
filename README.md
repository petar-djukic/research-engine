# Research Engine

A personal research pipeline that takes academic papers from acquisition through knowledge extraction to new paper generation. The system is a Go command-line tool backed by Mage build targets, designed to run locally on a single machine. Each stage transforms data into a more useful form: raw PDFs become structured text, structured text becomes extracted knowledge, and extracted knowledge feeds into new writing.

See [VISION.md](docs/VISION.md) for project goals and boundaries. See [ARCHITECTURE.md](docs/ARCHITECTURE.md) for system design and data flow.

## Prerequisites

This guide assumes macOS with [Homebrew](https://brew.sh) installed. If you do not have Homebrew, install it first:

```bash
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
```

### Go

Go is the implementation language for the pipeline and CLI.

```bash
brew install go
```

Verify:

```bash
go version
# go version go1.24.x darwin/arm64
```

### Mage

Mage is a Go-native build tool that orchestrates the pipeline stages.

```bash
go install github.com/magefile/mage@latest
```

Make sure `$GOPATH/bin` (typically `~/go/bin`) is on your PATH. Add this to your shell profile (`~/.zshrc` or `~/.bashrc`) if it is not already there:

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

Verify:

```bash
mage -version
```

### SQLite

SQLite powers the knowledge base index. macOS includes SQLite by default, but you can install a newer version:

```bash
brew install sqlite
```

Verify:

```bash
sqlite3 --version
```

### PDF Conversion Tools

The conversion stage requires an external tool to extract text from PDFs. Install at least one:

**Option A: Poppler (pdftotext)** — lightweight, handles most papers well:

```bash
brew install poppler
```

Verify:

```bash
pdftotext -v
```

**Option B: GROBID** — machine-learning-based, better at identifying document structure (sections, references). Requires Docker:

```bash
brew install docker
docker pull lfoppiano/grobid:0.8.1
docker run --rm -p 8070:8070 lfoppiano/grobid:0.8.1
```

Verify by visiting `http://localhost:8070` in a browser.

### Claude API Key (for Extraction and Generation)

The extraction and generation stages call the Claude API. Set your API key as an environment variable:

```bash
export ANTHROPIC_API_KEY="your-key-here"
```

Add this to your shell profile to persist it across sessions.

## Project Structure

```text
research-engine/
  cmd/research-engine/    CLI entry point
  internal/               Private implementation (one package per pipeline stage)
  pkg/types/              Shared data structures (Paper, KnowledgeItem, Draft)
  magefiles/              Mage build targets (one file per stage)
  docs/                   VISION, ARCHITECTURE, PRDs, use cases
  papers/                 Working directory for acquired papers (per-project)
  knowledge/              Working directory for knowledge base (per-project)
  output/                 Working directory for generated drafts (per-project)
```

## Pipeline Stages

The pipeline has five stages, each exposed as a Mage target:

| Stage          | Mage Target      | Description                                       | PRD                                                                                 |
| -------------- | ---------------- | ------------------------------------------------- | ----------------------------------------------------------------------------------- |
| Acquisition    | `mage download`  | Download papers from arXiv, DOI, or URL           | [prd001-acquisition](docs/specs/product-requirements/prd001-acquisition.yaml)       |
| Conversion     | `mage convert`   | Transform PDFs into structured Markdown           | [prd002-conversion](docs/specs/product-requirements/prd002-conversion.yaml)         |
| Extraction     | `mage extract`   | Pull claims, methods, and definitions from text   | [prd003-extraction](docs/specs/product-requirements/prd003-extraction.yaml)         |
| Knowledge Base | `mage index`     | Store and index extracted knowledge for retrieval | [prd004-knowledge-base](docs/specs/product-requirements/prd004-knowledge-base.yaml) |
| Generation     | `mage draft`     | Produce cited drafts from the knowledge base      | [prd005-generation](docs/specs/product-requirements/prd005-generation.yaml)         |

Run the full pipeline end-to-end:

```bash
mage pipeline
```

## Development

Run tests:

```bash
go test ./...
```

List available Mage targets:

```bash
mage -l
```
