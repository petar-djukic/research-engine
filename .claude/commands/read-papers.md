---
name: read-papers
description: Browse and read converted Markdown papers
---

# Read Papers

Browse and read papers that have been converted to Markdown. You read the files directly using the Read tool. No CLI command is needed.

## Input

The researcher may specify a paper name, topic, or section via `$ARGUMENTS`. If no arguments are given, list available papers.

## Workflow

1. **List available papers.** Check what papers have been converted:
   - Read metadata files from `papers/metadata/` to get titles, authors, and dates.
   - List Markdown files in `papers/markdown/` to show what is readable.
   - Present a numbered list of available papers with their titles.

2. **Read a paper.** When the researcher selects a paper:
   - Read the Markdown file from `papers/markdown/PAPER-ID.md`.
   - Present the paper's structure (section headings and page markers).
   - Offer to read specific sections or the full paper.

3. **Discuss the paper.** After reading:
   - Summarize the paper's contributions, methods, and findings.
   - Answer questions about specific sections or claims.
   - Identify connections to other papers the researcher has acquired.
   - Note limitations, assumptions, or open questions raised by the paper.

4. **Compare across papers.** When the researcher has multiple papers:
   - Compare methods, results, or conclusions across papers.
   - Identify common themes, disagreements, or complementary findings.
   - Suggest which papers address similar questions from different angles.

## File Locations

```
papers/
  metadata/     # YAML files with paper metadata (title, authors, DOI, etc.)
  markdown/     # Converted Markdown files (section headings, paragraphs, page markers)
```

## Reading Strategy

- **Start with structure.** Read the section headings first to understand the paper's organization.
- **Focus on what matters.** Read the abstract and conclusion first, then dive into specific sections the researcher cares about.
- **Track page numbers.** The Markdown files include page markers (e.g., `<!-- page 5 -->`). Reference these when discussing specific passages so the researcher can find them in the original PDF.
- **Read metadata.** The YAML metadata file has title, authors, date, DOI, and source URL for citation purposes.

## Tips

- For long papers, offer to summarize each section rather than reading the entire text.
- When comparing papers, focus on the specific dimension the researcher cares about (methods, results, theoretical framework, etc.).
- If a paper has not been converted yet, suggest using `/acquire-papers` to download and convert it.
