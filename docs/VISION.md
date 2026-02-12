# Research Engine

## Executive Summary

We build a Claude-powered research tool that helps a single researcher go from a research question to a cited paper. Claude acts as the researcher's partner: it searches for papers, reads them, builds understanding from extracted knowledge, and writes new work grounded in what it has read. A Go CLI provides the infrastructure that makes this possible: searching academic APIs, downloading PDFs, converting them to Markdown that Claude can read, extracting typed knowledge items, and storing them in a queryable local database. Claude drives the workflow through skills; the CLI handles the mechanical operations.

We are not a citation manager, a paper database, or a hosted service. We are a local research tool where Claude and the researcher collaborate to turn reading into writing.

## Introduction

### Research Context

Researchers read papers to build understanding and produce new work. The path from a research question to new writing involves several distinct activities: searching for relevant papers, downloading them, converting PDFs into readable text, extracting claims and methods, organizing knowledge, and composing new documents that draw on that knowledge. Each activity has its own tools, formats, and friction points.

### The Problem

Managing research papers end-to-end requires stitching together disconnected tools. A researcher might use a browser to find papers, a PDF viewer to read them, a note-taking app to capture ideas, a reference manager to track citations, and a word processor to write. Each transition between tools loses context. Extracted insights live in one place while the source material lives in another. When it comes time to write, the researcher must manually reconnect ideas to their origins.

Existing tools optimize for one stage. Zotero manages references. Semantic Scholar helps discover papers. Generative AI tools can summarize text. No single tool handles the full path from search to writing with a consistent data model, local-first operation, and an intelligent partner that can read, reason, and write.

### What We Do

We combine a Go CLI infrastructure layer with Claude skills that drive the research workflow. The infrastructure handles deterministic, repeatable operations: searching academic APIs, downloading PDFs, converting them to structured Markdown, extracting knowledge items, and indexing them in a local database. Claude handles the work that requires judgment: deciding which papers to read, understanding their content, finding connections across sources, and writing new text grounded in what it has read.

The researcher interacts primarily through Claude skills. When the researcher asks Claude to search for papers on a topic, Claude formulates queries, invokes the search CLI, and recommends which papers to acquire. When the researcher asks Claude to read a paper, Claude reads the Markdown directly and discusses findings. When the researcher asks Claude to write a section, Claude queries the knowledge base, reads relevant source passages, and produces cited prose.

Papers are converted to Markdown so that Claude can read them. This is the bridge between the PDF world and Claude's ability to reason about text. The knowledge base provides structured retrieval so that Claude can find relevant items across many papers without reading everything from scratch each time.

All data lives on disk in human-readable, version-controllable formats: PDFs, Markdown, YAML, and SQLite.

## Why This Project

A Claude-powered research tool fills a gap that neither reference managers nor standalone AI writing tools address. Reference managers focus on bibliography, not on transforming paper content into reusable knowledge. Standalone AI writing tools operate on prompts without access to the researcher's specific paper collection. The gap between "I have these papers" and "I can write informed by these papers" remains wide.

We build this because Claude can read, understand, and write, but it needs infrastructure to access papers in a format it can process, and a knowledge base to retrieve specific items efficiently. The target user is a single researcher who works with 10-50 papers per project and wants to produce a literature review, survey, or original paper. The tool runs locally, stores data in inspectable formats, and requires internet access only for searching academic sources, downloading papers, and Claude API calls during extraction.

Table 1 Relationship to Existing Tools

| Tool Category | What It Does | What We Add |
|---------------|-------------|-------------|
| Reference managers (Zotero, Mendeley) | Store PDFs, manage citations | Claude reads papers and writes with their knowledge |
| Discovery services (Semantic Scholar, arXiv) | Find papers | Claude-driven search that reasons about relevance and recommends acquisitions |
| PDF converters (GROBID, pdf2text) | Extract text from PDFs | Conversion to Markdown so Claude can read and reason about paper content |
| Generative AI writing tools | Generate text from prompts | Grounded writing from a structured knowledge base with provenance tracking |
| Note-taking apps (Obsidian, Notion) | Manual knowledge capture | Automated extraction with provenance; Claude reads papers directly for deeper understanding |
| Claude Code skills | N/A | Research workflow skills that combine infrastructure with Claude's reading and writing |

## Planning and Implementation

### Success Criteria

We measure success along three dimensions: infrastructure completeness, knowledge quality, and writing usefulness. Infrastructure completeness means every stage works without manual intervention between stages. Knowledge quality means extracted facts, claims, and methods trace back to specific passages in specific papers. Writing usefulness means Claude, guided by the researcher, can produce first drafts that save real editing effort compared to writing from scratch.

Table 2 Measurable Outcomes

| Dimension | Metric | Target |
|-----------|--------|--------|
| Infrastructure completeness | Stages that run without manual handoff | 5 of 5 (search, acquire, convert, extract, knowledge) |
| Search | Relevant papers found for a research query | Top-10 results contain papers a domain expert would select |
| Acquisition | Papers downloaded from a URL or identifier | Supports arXiv, DOI, direct PDF URLs, OpenAlex open-access |
| Conversion accuracy | Structured text matches source PDF content | Section headings, paragraphs, and figures preserved |
| Extraction coverage | Claims and methods captured per paper | At least 80% of manually identified items |
| Knowledge retrieval | Relevant items returned for a query | Top-5 results contain the target item |
| Writing grounding | Claims in written text traceable to sources | Every factual claim cites a specific paper |
| Skill usability | Research workflow driven through Claude skills | All five skills functional and composable |

### What "Done" Looks Like

A researcher asks Claude to explore a research topic. Claude searches academic sources, recommends papers, and downloads the ones the researcher selects. Claude reads the converted Markdown, discusses what it found, and extracts knowledge items into the local database. When the researcher is ready to write, Claude creates a paper project with an outline and numbered section files. Claude writes each section grounded in the papers it has read, with inline citations tracing every claim to its source. The researcher refines the draft with Claude iteratively, section by section. When done, the project compiles into a single document ready for submission. All data lives on disk in version-controllable formats.

The search, acquisition, extraction, and writing stages require internet access for API calls (academic search APIs and Claude). Conversion and knowledge base storage run fully offline.

### Implementation Phases

Table 3 Implementation Phases

| Phase | Focus | Deliverables |
|-------|-------|-------------|
| Foundation | Documentation and scaffolding | VISION, ARCHITECTURE, PRDs for each stage, Cobra CLI skeleton |
| Core Pipeline | Search, acquisition, and PDF conversion | Search stage (arXiv, Semantic Scholar queries), Acquire stage (URL/DOI/arXiv/OpenAlex download), Convert stage (PDF to structured text) |
| Knowledge | Extraction and storage | Extract stage (claims, methods, definitions from text), Knowledge base (local storage with retrieval) |
| Skills | Claude research workflow | Five skills (search-papers, acquire-papers, read-papers, query-knowledge, write-paper), paper writing workflow, Mage compile target |

### Risks and Mitigations

Table 4 Risks and Mitigations

| Risk | Impact | Likelihood | Mitigation |
|------|--------|------------|------------|
| PDF conversion quality varies across paper formats | Claude reads noisy or incomplete text | High | Support multiple conversion backends; allow manual correction of converted text |
| Generative AI extraction misses or hallucinates claims | Knowledge base contains false or incomplete data | Medium | Require provenance links; Claude can also read papers directly for deeper understanding |
| Paper sources block automated downloads | Acquisition stage fails | Medium | Support manual PDF import as fallback; prefer open-access via OpenAlex; respect rate limits |
| Context window limits for long papers | Claude cannot read an entire paper in one pass | Medium | Read section by section; use knowledge base for cross-paper retrieval |
| Scope creep toward a full reference manager | Project loses focus | Medium | The "What This Is NOT" section defines boundaries; review scope at each phase |

## What This Is NOT

We are not a reference manager. We do not manage bibliographic metadata for large libraries or sync across devices. Use Zotero or Mendeley for that.

We are not a literature crawler. We search academic APIs on demand but do not crawl, mirror, or index the full literature. We find papers relevant to a specific query, not build a comprehensive database.

We are not a hosted service. We do not run in the cloud, store data remotely, or require user accounts. Everything runs locally.

We are not a fully autonomous paper generator. Claude writes with the researcher's guidance. The researcher sets direction, approves outlines, and makes editorial decisions. Claude provides grounded first drafts; the researcher refines them.

We are not a collaborative platform. We serve a single researcher working on their own machine. Multi-user access and sharing are out of scope.

We are not an annotation tool. We do not provide a UI for highlighting PDFs or taking margin notes. Claude reads papers programmatically through Markdown conversion.

## References

See ARCHITECTURE.md for system components and data flow. See PRDs in docs/specs/product-requirements/ for detailed requirements per pipeline stage and paper writing workflow.
