---
name: acquire-papers
description: Download papers, convert to Markdown, and optionally extract knowledge
---

<!-- Copyright Mesh Intelligence Inc., 2026. All rights reserved. -->

# Acquire Papers

Download papers by identifier, convert them to readable Markdown, and optionally extract knowledge items. This combines three infrastructure stages (acquire, convert, extract) into one researcher-facing action.

## Input

The researcher provides one or more paper identifiers via `$ARGUMENTS`. Accepted identifier formats:
- arXiv IDs: `2301.01234` or `arxiv:2301.01234`
- DOIs: `10.1234/example` or `doi:10.1234/example`
- Direct PDF URLs: `https://example.com/paper.pdf`

If no arguments are given, ask the researcher which papers to acquire.

## Workflow

1. **Acquire papers.** Download PDFs and create metadata records:

   ```bash
   research-engine acquire IDENTIFIER1 IDENTIFIER2 ...
   ```

   Available flags:
   - `--papers-dir`: base directory for papers (default: `papers`)
   - `--timeout`: HTTP request timeout (default: 60s)
   - `--delay`: delay between downloads (default: 1s)

   The CLI resolves identifiers, downloads PDFs, and creates metadata YAML files. For DOIs, it tries OpenAlex first for open-access PDFs before falling back to publisher URLs.

2. **Convert to Markdown.** Transform downloaded PDFs into structured Markdown that you can read:

   ```bash
   research-engine convert --batch --papers-dir papers
   ```

   Or convert specific PDFs:

   ```bash
   research-engine convert papers/raw/paper-name.pdf --papers-dir papers
   ```

   Conversion requires a container runtime (Docker or Podman) for the MarkItDown backend.

3. **Optionally extract knowledge.** If the researcher wants to populate the knowledge base:

   ```bash
   research-engine extract --batch --papers-dir papers --knowledge-dir knowledge --model MODEL --api-key KEY
   ```

   Only offer extraction if the researcher asks for it or if they plan to query the knowledge base. Extraction calls the Claude API and costs tokens.

4. **Report results.** After each stage, report:
   - Which papers were acquired successfully and which failed.
   - Which papers were converted to Markdown.
   - Any papers that were skipped (already existed on disk).
   - The file paths where papers can be found.

5. **Offer next steps.** Suggest:
   - Reading the acquired papers with `/read-papers`.
   - Searching for more papers with `/search-papers`.
   - Extracting knowledge if not already done.

## Directory Structure

After acquisition and conversion, papers live at:

```
papers/
  raw/          # Downloaded PDF files
  metadata/     # YAML metadata per paper
  markdown/     # Converted Markdown files
```

## Tips

- Acquire in batches. The CLI handles multiple identifiers in one call with rate limiting between downloads.
- If conversion fails, check that Docker or Podman is running. The MarkItDown backend runs in a container.
- Papers that already exist on disk are skipped automatically. Re-running acquire is safe.
