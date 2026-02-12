---
name: search-papers
description: Search for academic papers on a topic, recommend acquisitions
---

<!-- Copyright Mesh Intelligence Inc., 2026. All rights reserved. -->

# Search Papers

Search for academic papers and patents on a research topic. You formulate queries, run the search CLI, interpret results, and recommend which papers or patents to acquire.

## Input

The researcher provides a topic, question, or structured search criteria via `$ARGUMENTS`. If no arguments are given, ask the researcher what they want to search for.

## Workflow

1. **Formulate queries.** Based on the researcher's topic, construct one or more search queries. Consider synonyms, related terms, and different phrasings to maximize coverage. For broad topics, run multiple focused queries rather than one vague query. When the researcher asks about patents, prior art, IP landscape, or granted claims, include patent searches (see patent flags below).

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
   - `--csl`: output as CSL YAML for reference managers
   - `--recency-bias`: boost recent papers
   - `--patents`: search only PatentsView (disables academic backends)
   - `--patentsview-api-key`: PatentsView API key (loaded automatically from `.secrets/patentsview-api-key` when configured)
   - `--query-file`: YAML file to save/load query and results

   Use `--json` when you need to process results programmatically. Use `--recency-bias` when the researcher wants recent work. Use `--patents` for patent-only searches to avoid academic backend noise.

   When the PatentsView API key is configured (via `.secrets/patentsview-api-key` or `--patentsview-api-key`), patent results appear alongside academic results automatically. No extra flags are needed for combined searches.

3. **Interpret results.** For each set of results:
   - Identify the most relevant papers and patents based on title, authors/inventors, and abstract.
   - Group results by subtopic or theme when multiple queries were run.
   - Note any results that appear across multiple queries (high relevance signal).
   - Flag papers from well-known authors or venues in the field.
   - For patent results, note the patent number (e.g. US7654321), inventor names, and grant date.

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

## Patent Searches

When the researcher asks about patents, prior art, or the IP landscape around a technology, use patent search. Patent results use US patent numbers (e.g. US7654321) as identifiers and can be acquired with `/acquire-papers` the same way as academic papers.

Patent-only search (no academic backends):

```bash
research-engine search "neural network data processing" --patents --max-results 20
```

Combined search (academic papers and patents together):

```bash
research-engine search "transformer architecture" --max-results 20
```

When the PatentsView API key is configured, this returns both academic papers and patents in one result set. Patent results are labeled with source "patentsview" and identifiers prefixed with "US".

Filter patents by inventor and date range:

```bash
research-engine search "attention mechanism" --author "Vaswani" --from 2017-01-01 --patents
```

## Tips

- Start broad, then narrow. A first pass with general terms reveals the landscape; follow-up queries target specific subtopics.
- Use `--from` and `--to` to focus on a time period when the researcher cares about recency or historical context.
- When results are sparse, try alternative terminology. Different communities use different words for the same concept.
- Use `--patents` for patent-only searches when the researcher wants to focus on the IP landscape without academic results mixed in.
- Patent search uses inventor last names for the `--author` filter, not full names. Use the last name only for best results.
- When the researcher needs both academic context and patent coverage, run a combined search (no `--patents` flag) to see papers and patents ranked together.
