<!-- Copyright Mesh Intelligence Inc., 2026. All rights reserved. -->

# Patent Search

We integrate US patent search into the research engine so that researchers can explore the patent landscape alongside academic literature. This guide covers the PatentsView API, API key setup, patent identifier formats, PDF resolution, and common query patterns. It exists because patent search introduces concepts (kind codes, patent numbers, inventor-based filtering) that differ from academic paper search, and researchers need a reference for setting up and using the feature.

## PatentsView API Overview

We query patents through the PatentsView API, a public interface to the USPTO patent database hosted at `https://search.patentsview.org/api/v1/patent/`. The API accepts structured JSON queries and returns patent metadata including titles, abstracts, inventors, dates, and classification codes.

Table 1 PatentsView Query Operators

| Operator | Purpose | Example |
|----------|---------|---------|
| `_text_any` | Full-text search (any word matches) | `{"_text_any":{"patent_title":"neural network"}}` |
| `_text_all` | Full-text search (all words must match) | `{"_text_all":{"patent_abstract":"transformer attention"}}` |
| `_contains` | Substring match on a field | `{"_contains":{"inventors.inventor_name_last":"Smith"}}` |
| `_gte` / `_lte` | Date range filters | `{"_gte":{"patent_date":"2020-01-01"}}` |
| `_and` / `_or` | Combine multiple conditions | `{"_and":[...conditions...]}` |

The API returns JSON with a `patents` array, a `count` of results in the current page, and `total_patent_count` across all pages. We request these fields per patent: `patent_id`, `patent_title`, `patent_abstract`, `patent_date`, `patent_type`, `patent_num_claims`, and `inventors.inventor_name_last`.

Rate limits: PatentsView allows 45 requests per minute. The search backend respects this limit through the inter-backend delay configuration. When the API returns HTTP 429 (Too Many Requests), we read the `Retry-After` header and report the wait time in the error message.

## API Key Setup

PatentsView requires an API key for authenticated access. We obtain one through their support portal and store it locally.

1. Visit the PatentsView API key request page at `https://patentsview-support.atlassian.net/servicedesk/customer/portal/1/group/1/create/18`.
2. Fill in your name, email, and a brief description of your use case (e.g. "Academic research tool for patent landscape analysis").
3. Wait for the confirmation email with your API key.
4. Create the secrets directory if it does not exist:

   ```bash
   mkdir -p .secrets
   ```

5. Save the API key to a file:

   ```bash
   echo "YOUR_API_KEY_HERE" > .secrets/patentsview-api-key
   ```

6. Verify the key is loaded by running a test search:

   ```bash
   research-engine search "neural network" --patents --max-results 1
   ```

The CLI loads API keys from `.secrets/` automatically at startup. The `.secrets/` directory is excluded from git through `.gitignore`. You can also pass the key directly with `--patentsview-api-key KEY`, but we recommend the file-based approach to avoid exposing keys in shell history.

## Patent Identifiers

US patents use a numbering system that differs from academic identifiers like arXiv IDs or DOIs.

Table 2 Patent Identifier Formats

| Format | Type | Example |
|--------|------|---------|
| `US` + 7 digits | Granted utility patent | US7654321 |
| `US` + 7-8 digits + kind code | Granted patent with kind code | US7654321B2 |
| `US` + 11 digits | Published application | US20230012345 |
| `US` + 11 digits + kind code | Published application with kind code | US20230012345A1 |

Kind codes indicate the publication stage of a patent document. The most common codes are B1 (granted, no prior publication), B2 (granted, with prior publication), and A1 (published application). The CLI strips kind codes for deduplication, so US7654321 and US7654321B2 refer to the same patent.

The acquisition stage auto-detects patent identifiers by their format: a "US" prefix followed by digits, with an optional alphabetic kind code suffix. No `--type` flag is required. Patent identifiers can be mixed freely with arXiv IDs and DOIs in the same acquire command.

## PDF Resolution

We resolve patent numbers to downloadable PDFs through Google Patents storage. The URL pattern is:

```
https://patentimages.storage.googleapis.com/pdfs/{patent_id}.pdf
```

For a patent like US7654321, the PDF URL is `https://patentimages.storage.googleapis.com/pdfs/US7654321.pdf`. When this URL returns a non-200 status (the PDF is not available in Google's storage), we fall back to the Google Patents HTML page at `https://patents.google.com/patent/US7654321/en` and report the fallback to stderr.

Patent PDFs are stored in `papers/raw/` alongside academic paper PDFs, named by their patent identifier slug (e.g. `US7654321.pdf`). Metadata YAML files go to `papers/metadata/` with the same naming convention.

## Query Examples

Search for patents by keyword:

```bash
research-engine search "neural network data processing" --patents --max-results 20
```

Search by inventor last name:

```bash
research-engine search --author "Hinton" --patents
```

Combine keyword and inventor search with a date range:

```bash
research-engine search "attention mechanism" --author "Vaswani" --from 2017-01-01 --to 2023-12-31 --patents
```

Combined academic and patent search (when API key is configured):

```bash
research-engine search "transformer architecture" --max-results 30
```

Save patent search results for later:

```bash
research-engine search "graph neural network" --patents --query-file papers/queries/gnn-patents.yaml
```

Acquire a patent PDF:

```bash
research-engine acquire US7654321 US20230012345A1
```

Mixed batch of papers and patents:

```bash
research-engine acquire 2301.07041 US11734097 "10.1038/s41586-021-03819-2"
```

## Troubleshooting

Table 3 Common Issues

| Symptom | Cause | Resolution |
|---------|-------|------------|
| HTTP 403 from PatentsView | Invalid or expired API key | Verify your key in `.secrets/patentsview-api-key` matches what PatentsView issued. Request a new key if expired. |
| HTTP 429 from PatentsView | Rate limit exceeded (45 req/min) | Wait for the period indicated in the error message. Reduce `--max-results` or increase `--delay` between searches. |
| Empty results for a known patent | Patent may use different terminology or the query is too narrow | Try broader terms. Search by patent number directly with `research-engine acquire US7654321`. |
| Google Patents PDF returns 404 | PDF not available in Google storage | The CLI falls back to the HTML page URL automatically. You can also download the PDF manually from Google Patents. |
| "empty PatentsView query" error | No searchable terms provided (only date range) | Add a keyword, inventor name, or free-text query alongside the date range. |
| Patent not recognized by acquire | Identifier does not match US patent format | Ensure the identifier starts with "US" followed by at least 6 digits. Kind codes (B2, A1) are optional. |

## References

- PatentsView API documentation: `https://search.patentsview.org/docs/`
- PatentsView Swagger UI: `https://search.patentsview.org/swagger-ui/`
- [prd008-patent-search](../specs/product-requirements/prd008-patent-search.yaml) for full requirements
- [ARCHITECTURE.md](../ARCHITECTURE.md) for system design and component descriptions
