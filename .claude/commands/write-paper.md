---
name: write-paper
description: Create a structured paper project and write sections with citations
---

<!-- Copyright Mesh Intelligence Inc., 2026. All rights reserved. -->

# Write Paper

Create a structured paper project and iteratively write sections with inline citations grounded in the knowledge base and source papers. The researcher and Claude develop the paper together through four phases: initialize, outline, write, and refine.

## Input

The researcher describes what paper they want to write via `$ARGUMENTS`. This may be a topic, a title, a type of paper (survey, literature review, position paper), or instructions to continue working on an existing project.

## Paper Project Structure

Each paper lives in a directory under `output/papers/` with this layout:

```
output/papers/[slug]/
  00-title-page.md       # YAML frontmatter: title, authors, date, type, abstract, keywords
  01-introduction.md     # First section
  02-related-work.md     # Second section
  ...                    # Additional numbered sections
  outline.yaml           # Section tracking: number, title, file, description, status
  references.yaml        # All cited papers: citation_key, paper_id, title, authors, year
```

## Phase 1: Initialize

Create the project directory and title page.

1. Choose a URL-safe slug for the directory name (e.g., `transformer-efficiency-survey`).
2. Create `output/papers/[slug]/`.
3. Create `00-title-page.md` with YAML frontmatter:

   ```markdown
   ---
   title: "Paper Title"
   authors:
     - name: "Researcher Name"
       affiliation: "Institution"
   date: YYYY-MM-DD
   type: survey  # survey, literature-review, original-research, position-paper
   abstract: ""
   keywords:
     - keyword1
     - keyword2
   ---
   ```

4. Create empty `outline.yaml` and `references.yaml`.

## Phase 2: Outline

Propose a paper structure based on the topic and available knowledge.

1. Query the knowledge base to understand what material is available:

   ```bash
   research-engine knowledge retrieve "topic" --json
   ```

2. Read relevant papers from `papers/markdown/` to understand the landscape.

3. Propose numbered sections with descriptions. Create `outline.yaml`:

   ```yaml
   sections:
     - number: "01"
       title: Introduction
       file: 01-introduction.md
       description: "Motivates the survey, states the research question, and previews the structure."
       status: outline
     - number: "02"
       title: Related Work
       file: 02-related-work.md
       description: "Reviews prior surveys and positions this work relative to them."
       status: outline
   ```

4. Create each section file with the heading, a brief description of what the section will cover, and placeholder subsection headings:

   ```markdown
   # Introduction

   This section motivates the survey by describing the growth of transformer
   architectures and the need for efficiency improvements.

   ## Background
   ## Research Question
   ## Paper Organization
   ```

5. Present the outline to the researcher for approval before writing content.

## Phase 3: Write

Compose section content with inline citations.

1. For each section, query the knowledge base for relevant items:

   ```bash
   research-engine knowledge retrieve "section topic" --json
   ```

2. Read source papers directly from `papers/markdown/` for additional context and nuance beyond what extraction captured.

3. Write section content in the section's Markdown file. Use inline citations in square-bracket format:

   ```markdown
   Transformer models have achieved state-of-the-art results across NLP tasks
   [Vaswani2017]. Recent work has focused on reducing the quadratic complexity
   of self-attention [Tay2022; Kitaev2020].
   ```

4. Maintain `references.yaml` with entries for every cited paper:

   ```yaml
   papers:
     - citation_key: Vaswani2017
       paper_id: attention-is-all-you-need
       title: "Attention Is All You Need"
       authors:
         - Vaswani
         - Shazeer
         - Parmar
       year: 2017
       venue: NeurIPS
     - citation_key: Tay2022
       paper_id: efficient-transformers-survey
       title: "Efficient Transformers: A Survey"
       authors:
         - Tay
         - Dehghani
         - Bahri
       year: 2022
       venue: ACM Computing Surveys
   ```

5. Update `outline.yaml` status from `outline` to `draft` as sections are written.

## Phase 4: Refine

Revise sections based on researcher feedback.

1. The researcher identifies sections to improve and provides direction (e.g., "expand the methods comparison", "add more recent work", "strengthen the argument in the conclusion").

2. Re-query the knowledge base for additional items. Check for newly extracted papers.

3. Revise the section content while preserving the overall structure.

4. Update `outline.yaml` status from `draft` to `revised`.

5. Update the abstract in `00-title-page.md` to reflect the paper's content as it develops.

## Citation Rules

- Every factual claim must have an inline citation: `[AuthorYear]`.
- Multiple citations use semicolons: `[Author2017; Author2020]`.
- Citation keys follow AuthorYear format (first author surname + publication year).
- Every citation key must have a matching entry in `references.yaml`.
- The `paper_id` in references.yaml links to the acquired paper in `papers/`.

## Continuing an Existing Project

When the researcher wants to continue work on an existing paper:

1. Read `outline.yaml` to understand the current structure and status.
2. Read existing section files to understand what has been written.
3. Read `references.yaml` to know which papers are already cited.
4. Ask the researcher what they want to work on next.

## Tips

- Write one section at a time. Present it to the researcher for feedback before moving to the next.
- Draw on both knowledge base items (for specific claims and citations) and direct paper reading (for broader context and synthesis).
- When the knowledge base lacks items for a section, read the source papers directly and note which papers you are drawing from.
- Keep the abstract updated as the paper develops. A good abstract reflects the final content.
