---
name: search-papers
description: Search for academic papers on a topic, recommend acquisitions
---

# Search Papers

Search for academic papers on a research topic. You formulate queries, run the search CLI, interpret results, and recommend which papers to acquire.

## Input

The researcher provides a topic, question, or structured search criteria via `$ARGUMENTS`. If no arguments are given, ask the researcher what they want to search for.

## Workflow

1. **Formulate queries.** Based on the researcher's topic, construct one or more search queries. Consider synonyms, related terms, and different phrasings to maximize coverage. For broad topics, run multiple focused queries rather than one vague query.

2. **Run searches.** Execute the search CLI for each query:

   ```bash
   research-engine search "query text" --max-results 20
   ```

   Available flags:
   - `--query` or positional argument: free-text research question
   - `--author`: filter by author name
   - `--keywords`: filter by keywords (comma-separated)
   - `--from` / `--to`: publication date range (YYYY-MM-DD)
   - `--max-results`: maximum results (default 20)
   - `--json`: output as JSON for parsing
   - `--recency-bias`: boost recent papers

   Use `--json` when you need to process results programmatically. Use `--recency-bias` when the researcher wants recent work.

3. **Interpret results.** For each set of results:
   - Identify the most relevant papers based on title, authors, and abstract.
   - Group results by subtopic or theme when multiple queries were run.
   - Note any papers that appear across multiple queries (high relevance signal).
   - Flag papers from well-known authors or venues in the field.

4. **Recommend acquisitions.** Present a curated list of papers worth acquiring:
   - Explain why each paper is relevant to the researcher's topic.
   - Highlight papers that seem foundational, recent, or uniquely relevant.
   - Note any gaps in the search results (topics that had few or no results).

5. **Offer next steps.** Ask the researcher if they want to:
   - Acquire specific papers (proceed to `/acquire-papers`).
   - Refine the search with different queries or filters.
   - Save the results for later review using `--query-file`.

## Saving Results

To save search results for later review:

```bash
research-engine search "query" --query-file papers/queries/topic-name.yaml
```

To reload saved results:

```bash
research-engine search --query-file papers/queries/topic-name.yaml
```

## Tips

- Start broad, then narrow. A first pass with general terms reveals the landscape; follow-up queries target specific subtopics.
- Use `--from` and `--to` to focus on a time period when the researcher cares about recency or historical context.
- When results are sparse, try alternative terminology. Different communities use different words for the same concept.
