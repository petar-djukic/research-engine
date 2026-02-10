# Research Engine

## Executive Summary

We build a personal research pipeline that takes academic papers from search through knowledge extraction to new paper generation. The system is a Go command-line tool backed by Mage build targets, designed to run locally on a single machine. The pipeline begins with search: the researcher describes a topic or question, and the system finds relevant papers across arXiv, Semantic Scholar, and other sources. Search results feed directly into acquisition, which downloads the PDFs. From there, each stage transforms data into a more useful form: raw PDFs become structured text, structured text becomes extracted knowledge, and extracted knowledge feeds into new writing.

We are not a citation manager, a paper database, or a hosted service. We are a local, opinionated pipeline that a single researcher operates to turn reading into writing.

## Introduction

### Research Context

Researchers read papers to build understanding and produce new work. The path from a research question to new writing involves several distinct activities: searching for relevant papers, downloading them, converting PDFs into machine-readable text, extracting claims and methods from that text, organizing extracted knowledge, and composing new documents that draw on that knowledge. Each activity has its own tools, formats, and friction points.

### The Problem

Managing research papers end-to-end requires stitching together disconnected tools. A researcher might use a browser to find papers, a PDF viewer to read them, a note-taking app to capture ideas, a reference manager to track citations, and a word processor to write. Each transition between tools loses context. Extracted insights live in one place while the source material lives in another. When it comes time to write, the researcher must manually reconnect ideas to their origins.

Existing tools optimize for one stage. Zotero manages references. Semantic Scholar helps discover papers. Various Generative AI tools can summarize text. No single tool handles the full path from acquisition to generation with a consistent data model and local-first operation.

### What We Do

We define a pipeline with six stages: search, acquire, convert, extract, store, and generate. A Go CLI orchestrates these stages through Mage build targets. The researcher starts with a topic or question; search finds relevant papers across academic sources and feeds their identifiers into acquisition. From there, papers move through each stage, accumulating structured metadata and extracted knowledge along the way. The pipeline stores everything locally in a well-defined directory structure. When the researcher is ready to write, the generation stage draws on the accumulated knowledge base to produce drafts that cite their sources.

The pipeline treats each stage as an independent, composable step. A researcher can run the full pipeline or invoke individual stages. All intermediate artifacts persist on disk in human-readable formats.

## Why This Project

A personal research pipeline fits a specific need that commercial tools do not address well. Commercial reference managers focus on bibliography and annotation, not on transforming paper content into reusable knowledge. Generative AI writing tools operate on prompts, not on structured knowledge bases built from specific papers. The gap between "I have read these papers" and "I can write with these papers' knowledge" remains wide.

We build this because we need it ourselves. The target user is a single researcher who reads 10-50 papers for a project and wants to produce a literature review, survey, or original paper informed by those sources. The pipeline runs on the researcher's machine, stores data locally in formats they can inspect and version-control, and requires internet access only for searching academic sources, downloading papers, and calling the Claude API during extraction and generation.

Table 1 Relationship to Existing Tools

| Tool Category | What It Does | What We Add |
|---------------|-------------|-------------|
| Reference managers (Zotero, Mendeley) | Store PDFs, manage citations | Full text extraction, knowledge modeling, generation |
| Discovery services (Semantic Scholar, arXiv) | Find papers | Integrated search that feeds directly into acquisition and the rest of the pipeline |
| PDF converters (GROBID, pdf2text) | Extract text from PDFs | Integration with downstream extraction and storage |
| Generative AI writing tools | Generate text from prompts | Grounded generation from a structured knowledge base |
| Note-taking apps (Obsidian, Notion) | Manual knowledge capture | Automated extraction with provenance tracking |

## Planning and Implementation

### Success Criteria

We measure success along three dimensions: pipeline completeness, knowledge quality, and generation usefulness. Pipeline completeness means every stage works end-to-end without manual intervention between stages. Knowledge quality means extracted facts, claims, and methods trace back to specific passages in specific papers. Generation usefulness means a researcher can produce a first draft that saves real editing effort compared to writing from scratch.

Table 2 Measurable Outcomes

| Dimension | Metric | Target |
|-----------|--------|--------|
| Pipeline completeness | Stages that run without manual handoff | 6 of 6 |
| Search | Relevant papers found for a research query | Top-10 results contain papers a domain expert would select |
| Acquisition | Papers downloaded from a URL or identifier | Supports arXiv, DOI, direct PDF URLs |
| Conversion accuracy | Structured text matches source PDF content | Section headings, paragraphs, and figures preserved |
| Extraction coverage | Claims and methods captured per paper | At least 80% of manually identified items |
| Knowledge retrieval | Relevant items returned for a query | Top-5 results contain the target item |
| Generation grounding | Claims in generated text traceable to sources | Every factual claim cites a specific paper |

### What "Done" Looks Like

A researcher types a research question into the CLI. The pipeline searches academic sources, presents relevant papers, and downloads the selected PDFs. It converts each PDF to structured text, extracts knowledge items, and stores them in a local knowledge base. When the researcher asks the system to draft a section on a topic, it retrieves relevant knowledge items and produces a paragraph with inline citations. The researcher can verify every claim by following the citation back to the source passage. All data lives on disk in version-controllable formats. The search, extraction, and generation stages require internet access for API calls (academic search APIs and Claude); conversion and storage run fully offline.

### Implementation Phases

Table 3 Implementation Phases

| Phase | Focus | Deliverables |
|-------|-------|-------------|
| Foundation | Documentation and scaffolding | VISION, ARCHITECTURE, PRDs for each stage, Mage project skeleton |
| Core Pipeline | Search, acquisition, and PDF conversion | Search stage (arXiv, Semantic Scholar queries), Acquire stage (URL/DOI/arXiv download), Convert stage (PDF to structured text) |
| Knowledge | Extraction and storage | Extract stage (claims, methods, definitions from text), Knowledge base (local storage with retrieval) |
| Generation | Paper writing from knowledge | Generate stage (drafts from knowledge base with citations), Output formatting |

### Risks and Mitigations

Table 4 Risks and Mitigations

| Risk | Impact | Likelihood | Mitigation |
|------|--------|------------|------------|
| PDF conversion quality varies across paper formats | Downstream extraction suffers from noisy input | High | Support multiple conversion backends; allow manual correction of converted text |
| Generative AI extraction misses or hallucinates claims | Knowledge base contains false or incomplete data | Medium | Require provenance links; let the researcher verify and edit extracted items |
| Paper sources block automated downloads | Acquisition stage fails | Medium | Support manual PDF import as fallback; respect rate limits and access policies |
| Knowledge base grows unwieldy across many projects | Retrieval quality degrades | Low | Partition knowledge by project; support pruning and archiving |
| Scope creep toward a full reference manager | Project loses focus on the pipeline | Medium | The "What This Is NOT" section defines boundaries; review scope at each phase |

## What This Is NOT

We are not a reference manager. We do not manage bibliographic metadata for large libraries or sync across devices. Use Zotero or Mendeley for that.

We are not a literature crawler. We search academic APIs on demand but do not crawl, mirror, or index the full literature. We find papers relevant to a specific query, not build a comprehensive database.

We are not a hosted service. We do not run in the cloud, store data remotely, or require user accounts. Everything runs locally.

We are not a general-purpose Generative AI writing tool. We generate text grounded in a specific knowledge base, not from open-ended prompts. The generation stage is constrained by what the pipeline has extracted.

We are not a collaborative platform. We serve a single researcher working on their own machine. Multi-user access and sharing are out of scope.

We are not an annotation tool. We do not provide a UI for highlighting PDFs or taking margin notes. We extract knowledge programmatically.

## References

See ARCHITECTURE.md for system components and data flow. See PRDs in docs/specs/product-requirements/ for detailed requirements per pipeline stage.
