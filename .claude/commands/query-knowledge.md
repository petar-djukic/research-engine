---
name: query-knowledge
description: Search the knowledge base for extracted items across papers
---

<!-- Copyright Mesh Intelligence Inc., 2026. All rights reserved. -->

# Query Knowledge

Search the knowledge base to find extracted knowledge items across all papers. You run the retrieve CLI, interpret results, and help the researcher find connections, trace items to sources, and identify themes.

## Input

The researcher provides a query, topic, or question via `$ARGUMENTS`. If no arguments are given, ask what they want to find.

## Workflow

1. **Formulate the query.** Based on the researcher's question, determine which search approach to use:
   - **Full-text search**: for topic-based queries ("what methods exist for X?")
   - **Type filter**: for finding specific item types (claims, methods, definitions, results)
   - **Paper filter**: for finding all items from a specific paper
   - **Tag filter**: for finding items tagged with a specific concept
   - **Combined**: mix full-text and filters for precise retrieval

2. **Run the query.** Execute the retrieve CLI:

   ```bash
   research-engine knowledge retrieve "query text" --json
   ```

   Available flags:
   - Positional argument or `--query`: full-text search query
   - `--type`: filter by item type (claim, method, definition, result)
   - `--tag`: filter by tag
   - `--paper`: filter by paper ID
   - `--limit`: maximum results
   - `--trace ITEM_ID`: show source context for a specific item
   - `--json`: output as JSON for detailed parsing
   - `--knowledge-dir`: knowledge base directory (default: `knowledge`)
   - `--papers-dir`: papers directory (default: `papers`)

   Use `--json` to get full item details including content, provenance, and tags.

3. **Present results.** Group and interpret the results:
   - Group items by paper and type for overview.
   - Highlight items most relevant to the researcher's question.
   - Show provenance (paper, section, page) for each item.

4. **Trace to source.** When the researcher wants to verify or read more context:

   ```bash
   research-engine knowledge retrieve --trace ITEM_ID
   ```

   This shows the surrounding text from the source Markdown file. You can also read the source paper directly from `papers/markdown/PAPER-ID.md` for broader context.

5. **Identify patterns.** Help the researcher see across papers:
   - **Connections**: items from different papers that address the same topic or method.
   - **Gaps**: questions or methods that appear in some papers but lack results or evaluation.
   - **Themes**: recurring concepts, approaches, or findings across the knowledge base.
   - **Contradictions**: claims from different papers that disagree.

6. **Offer next steps.** Suggest:
   - Reading specific papers for more context (`/read-papers`).
   - Running additional queries to explore a theme further.
   - Using findings for paper writing (`/write-paper`).

## Examples

Find all methods related to attention mechanisms:
```bash
research-engine knowledge retrieve "attention mechanism" --type method --json
```

Find all claims from a specific paper:
```bash
research-engine knowledge retrieve --paper transformer-efficiency-2024 --type claim --json
```

Trace an item to its source passage:
```bash
research-engine knowledge retrieve --trace item-abc123
```

## Tips

- Use `--json` output for detailed results. The table format truncates content.
- Combine queries with type filters for precision. "transformer" alone returns everything; "transformer" with `--type method` returns only methods.
- Run multiple queries with different phrasings. The FTS5 engine matches exact terms; synonyms require separate queries.
- When the knowledge base is empty, suggest using `/acquire-papers` followed by extraction.
