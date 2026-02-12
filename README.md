<!-- Copyright Mesh Intelligence Inc., 2026. All rights reserved. -->

# Research Engine

A Claude-powered research tool for academic papers. This tool is designed to be used with [Claude Code](https://docs.anthropic.com/en/docs/claude-code), Anthropic's CLI for Claude. Claude drives the research workflow through skills: searching for papers, reading them, querying a knowledge base, and writing new work with citations. A Go CLI provides the infrastructure: downloading PDFs, converting them to Markdown so Claude can read them, extracting typed knowledge items, and indexing them for retrieval.

The researcher works inside Claude Code, using slash commands like `/search-papers` and `/read-papers` to interact with the system. Claude reads papers, builds understanding, and writes cited prose. The Go CLI runs behind the scenes, invoked by Claude as needed.

See [VISION.md](docs/VISION.md) for project goals and boundaries. See [ARCHITECTURE.md](docs/ARCHITECTURE.md) for system design and data flow. See [eng01-claude-research-skills](docs/engineering/eng01-claude-research-skills.md) for skill design philosophy and conventions. See [eng02-patent-search](docs/engineering/eng02-patent-search.md) for patent search setup and usage. See [eng03-sample-prompts](docs/engineering/eng03-sample-prompts.md) for example research sessions.

## Prerequisites

- **Go 1.25+** — implementation language and build tool
- **Container runtime** (Docker or Podman) — required for PDF conversion (markitdown backend)
- **Claude API key** — required for extraction stage (set `ANTHROPIC_API_KEY` environment variable)
- **Claude Code** — the researcher's interface to Claude skills
- **PatentsView API key** (optional) — required for patent search. See [eng02-patent-search](docs/engineering/eng02-patent-search.md) for setup instructions. Store the key in `.secrets/patentsview-api-key`.

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

### Container Runtime (Podman)

The conversion stage runs MarkItDown inside a container for PDF-to-Markdown conversion. We recommend Podman; Docker also works.

Install Podman:

```bash
brew install podman
```

Initialize and start a Podman machine (required on macOS, where containers run in a Linux VM):

```bash
podman machine init
podman machine start
```

Verify Podman is working:

```bash
podman info
```

Build the markitdown container image:

```bash
podman build -t markitdown:latest containers/markitdown/
```

Verify the image exists:

```bash
podman image exists markitdown:latest && echo "OK"
```

If you prefer Docker instead, the same Dockerfile works:

```bash
docker build -t markitdown:latest containers/markitdown/
```

The CLI auto-detects which runtime is available.

### Podman Machine Management

The Podman machine must be running for PDF conversion. It persists across reboots but stops when the machine is shut down.

```bash
podman machine start    # start the VM (required before convert)
podman machine stop     # stop the VM when done
podman machine list     # check machine status
```

## Build

```bash
go build -tags sqlite_fts5 -o bin/research-engine ./cmd/research-engine/
```

Or use Mage:

```bash
go run github.com/magefile/mage@latest build
```

## Getting Started with Claude Code

This tool requires [Claude Code](https://docs.anthropic.com/en/docs/claude-code), Anthropic's agentic coding tool for the terminal. Claude Code reads the skill definitions in `.claude/commands/` and makes them available as slash commands.

Install Claude Code:

```bash
npm install -g @anthropic-ai/claude-code
```

Start a session from the project root:

```bash
cd research-engine
claude
```

Claude Code loads the project's skills automatically. Type `/` to see all available commands. A typical research session looks like this:

```text
You:    /search-papers transformer efficiency for long documents
Claude: [formulates queries, runs search, presents ranked results]

You:    /acquire-papers 2009.14794 2110.08678
Claude: [downloads PDFs, converts to Markdown, reports results]

You:    /read-papers
Claude: [lists available papers, reads the one you select, discusses findings]

You:    /query-knowledge what attention mechanisms reduce quadratic complexity?
Claude: [searches knowledge base, groups results by paper, traces to source]

You:    /write-paper survey on efficient attention mechanisms
Claude: [creates project, proposes outline, writes sections with citations]
```

Claude reads the converted Markdown papers directly. The Go CLI handles downloading, conversion, and indexing. Claude handles everything that requires judgment: query formulation, paper comprehension, knowledge synthesis, and writing.

## Claude Skills

The researcher's primary interface is through Claude Code skills. Each skill is a slash command that Claude executes.

| Skill | Command | Purpose |
|-------|---------|---------|
| Search | `/search-papers` | Search academic APIs, recommend papers to acquire |
| Acquire | `/acquire-papers` | Download papers, convert to Markdown, optionally extract |
| Read | `/read-papers` | Browse and read converted Markdown papers |
| Query | `/query-knowledge` | Search the knowledge base, trace items to sources |
| Write | `/write-paper` | Create paper projects, write sections with citations |

## Infrastructure Commands

The Go CLI provides infrastructure that Claude invokes through skills. Each stage is a Cobra subcommand.

| Stage | Command | Status | PRD |
|-------|---------|--------|-----|
| Search | `research-engine search` | Implemented | [prd006-search](docs/specs/product-requirements/prd006-search.yaml) |
| Acquisition | `research-engine acquire` | Implemented | [prd001-acquisition](docs/specs/product-requirements/prd001-acquisition.yaml) |
| Conversion | `research-engine convert` | Implemented | [prd002-conversion](docs/specs/product-requirements/prd002-conversion.yaml) |
| Extraction | `research-engine extract` | Implemented | [prd003-extraction](docs/specs/product-requirements/prd003-extraction.yaml) |
| Knowledge Base | `research-engine knowledge` | Implemented | [prd004-knowledge-base](docs/specs/product-requirements/prd004-knowledge-base.yaml) |

### Search

Search queries arXiv, Semantic Scholar, OpenAlex, and PatentsView for papers and patents matching a research question.

```bash
research-engine search "transformer attention mechanisms"
research-engine search --query "neural networks" --author "Hinton" --max-results 10
research-engine search --keywords "LLM,reasoning" --from 2025-01-01 --json
research-engine search --query "diffusion models" --query-file results.yaml
research-engine search --query-file results.yaml   # reload saved results
research-engine search --query "attention" --csl    # CSL YAML output
research-engine search "neural network" --patents   # patent-only search
```

Flags:

| Flag | Description |
|------|-------------|
| `--query` | Free-text research question |
| `--author` | Filter by author name (or inventor last name for patents) |
| `--keywords` | Filter by keywords (comma-separated) |
| `--from` | Publication date range start (YYYY-MM-DD) |
| `--to` | Publication date range end (YYYY-MM-DD) |
| `--max-results` | Maximum number of results (default 20) |
| `--json` | Output results as JSON |
| `--csl` | Output results as CSL YAML for reference managers |
| `--recency-bias` | Boost recently published papers |
| `--query-file` | YAML file to save/load query and results |
| `--patents` | Search only PatentsView (disables academic backends) |
| `--patentsview-api-key` | PatentsView API key (or use `.secrets/patentsview-api-key`) |

When the PatentsView API key is configured, patent results appear alongside academic results automatically. Use `--patents` for patent-only searches.

### Acquire

Acquire downloads papers and patents from arXiv IDs, DOIs, US patent numbers, or direct PDF URLs.

```bash
research-engine acquire 2301.07041
research-engine acquire "10.1038/s41586-021-03819-2"
research-engine acquire https://example.com/paper.pdf
research-engine acquire US7654321 US20230012345A1
research-engine acquire 2301.07041 US11734097 --timeout 2m --delay 2s
```

Patent identifiers (US prefix followed by digits, with optional kind code) are auto-detected.

Flags:

| Flag | Description |
|------|-------------|
| `--timeout` | HTTP request timeout (default 60s) |
| `--delay` | Delay between consecutive downloads (default 1s) |
| `--papers-dir` | Base directory for papers (default "papers") |

### Convert

Convert transforms downloaded PDFs into structured Markdown. Requires a running container runtime with the `markitdown:latest` image.

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

### Extract

Extract identifies typed knowledge items (claims, methods, definitions, results) from converted Markdown using the Claude API.

```bash
research-engine extract --batch --model claude-sonnet-4-5-20250929 --api-key $ANTHROPIC_API_KEY
research-engine extract 2301.07041 --model claude-sonnet-4-5-20250929 --api-key $ANTHROPIC_API_KEY
```

### Knowledge Base

Store, retrieve, and export knowledge items.

```bash
research-engine knowledge store                          # ingest extracted items
research-engine knowledge retrieve "attention mechanism"  # full-text search
research-engine knowledge retrieve --type method --json   # filter by type
research-engine knowledge retrieve --trace ITEM_ID        # trace to source
research-engine knowledge export --format yaml            # export to YAML
```

## Project Structure

```text
research-engine/
  .claude/commands/       Claude research skills (search, acquire, read, query, write)
  .secrets/               API keys loaded at runtime (not committed to git)
  cmd/research-engine/    CLI entry point and Cobra subcommands
  internal/               Private implementation (one package per pipeline stage)
  pkg/types/              Shared data structures (Paper, SearchResult, KnowledgeItem)
  containers/markitdown/  Dockerfile for the markitdown conversion image
  magefiles/              Build automation and developer tooling
  scripts/                Helper scripts
  docs/                   VISION, ARCHITECTURE, PRDs, engineering guides
  papers/                 Working directory for acquired papers (per-project)
  knowledge/              Working directory for knowledge base (per-project)
  output/papers/          Paper projects written by Claude
```

## Development

Run tests:

```bash
go test -tags sqlite_fts5 ./...
```

Build automation (requires [Mage](https://magefile.org)):

```bash
go run github.com/magefile/mage@latest -l       # list available targets
go run github.com/magefile/mage@latest build     # compile CLI
go run github.com/magefile/mage@latest stats     # project statistics
go run github.com/magefile/mage@latest compile output/papers/my-survey  # PDF from paper project
```
