<!-- Copyright Mesh Intelligence Inc., 2026. All rights reserved. -->

# Claude Research Skills

We define five research skills in `.claude/commands/` that form the primary interface between the researcher and the system. This guideline describes the design philosophy behind those skills, the conventions each skill follows, and how they compose into a research workflow. It exists because skills are the layer the researcher touches most. Understanding their design keeps the set consistent as we add new ones and helps contributors see where Claude's judgment ends and CLI infrastructure begins.

The five skills are search-papers, acquire-papers, read-papers, query-knowledge, and write-paper. Together they cover the full path from a research question to a cited paper. Each skill wraps one or more infrastructure stages with Claude's ability to reason about queries, interpret results, read papers, and write prose.

## Two-Layer Architecture

We split the system into two layers. The Go CLI handles deterministic, repeatable operations: searching academic APIs, downloading PDFs, converting them to Markdown, extracting knowledge items, and indexing them in SQLite. Claude handles work that requires judgment: deciding which queries to run, interpreting search results, reading and understanding papers, finding connections across sources, and writing cited text.

Skills sit at the boundary between these layers. A skill is a Markdown prompt in `.claude/commands/` that tells Claude how to combine CLI operations with its own reasoning to accomplish a researcher-facing goal. The researcher never needs to learn CLI flags; they invoke `/search-papers transformer efficiency` and Claude handles the rest.

Table 1 Layer Responsibilities

| Concern | Layer | Examples |
|---------|-------|----------|
| Query formulation | Claude | Expanding a topic into multiple search queries, choosing synonyms |
| API calls and downloads | Go CLI | `research-engine search`, `research-engine acquire` |
| PDF-to-Markdown conversion | Go CLI | `research-engine convert --batch` |
| Reading and comprehension | Claude | Reading Markdown files, summarizing, comparing across papers |
| Knowledge extraction | Go CLI | `research-engine extract --batch` |
| Knowledge retrieval | Go CLI | `research-engine knowledge retrieve` |
| Interpreting retrieved items | Claude | Grouping by theme, tracing to source, identifying gaps |
| Writing and citation | Claude | Composing prose, maintaining references.yaml |
| PDF compilation | Mage target | `mage compile output/papers/slug` |

The rule of thumb: if an operation produces the same output given the same input regardless of context, it belongs in the CLI. If it requires understanding the researcher's intent, reading surrounding text, or making a judgment call, it belongs in the skill.

## Design Philosophy

Three principles govern how we design skills.

We let Claude add judgment while the CLI handles mechanics. A skill does not reimplement what the CLI already does. When the researcher asks to search for papers, the skill formulates queries and calls `research-engine search`. It does not call academic APIs directly. When the researcher asks to acquire a paper, the skill calls `research-engine acquire` and `research-engine convert`. It does not download PDFs or run container commands. This separation keeps skills thin and lets us improve infrastructure without rewriting prompts.

We compose infrastructure stages into researcher-facing actions. The researcher thinks in terms of goals: "find papers on RAG," "read the Lewis et al. paper," "write a literature review." They do not think in terms of pipeline stages. A single skill may invoke multiple CLI commands in sequence. acquire-papers runs acquire, then convert, then optionally extract. write-paper reads papers, queries the knowledge base, and writes files. The skill hides the plumbing and presents a coherent action.

We follow a consistent input-workflow-output pattern. Every skill accepts input from `$ARGUMENTS` or conversation, executes a numbered workflow, and ends by presenting results with next-step suggestions that chain to other skills. This consistency means the researcher can predict how any skill works after using one of them. It also means new skills fit naturally into the set.

## Skill Anatomy

Every research skill follows the same structure in its Markdown file.

Table 2 Skill File Structure

| Section | Purpose |
|---------|---------|
| YAML frontmatter | `name` and `description` fields that register the skill as a slash command |
| H1 title | Short name matching the frontmatter |
| Opening paragraph | One-sentence summary of what the skill does |
| Input | What the researcher provides: `$ARGUMENTS`, conversation, or both |
| Workflow | Numbered steps from input to output, with CLI commands in fenced code blocks |
| Tips | Short list of practical advice for getting good results |

Some skills add extra sections when the workflow demands it. write-paper includes Phase subsections (Initialize, Outline, Write, Refine) and Citation Rules. query-knowledge includes an Examples section with concrete CLI invocations. These additions are appropriate when the workflow has distinct modes or when examples clarify flag combinations.

## The Five Research Skills

Table 3 Skill Overview

| Skill | What the Researcher Says | What Claude Does | Infrastructure Used |
|-------|-------------------------|-----------------|-------------------|
| search-papers | "Find papers on RAG" | Formulates queries, runs search, recommends papers | `research-engine search` |
| acquire-papers | "Download 2005.11401" | Downloads PDF, converts to Markdown, reports results | `research-engine acquire`, `convert`, `extract` |
| read-papers | "Read the Lewis paper" | Lists papers, reads Markdown, discusses findings | Reads files directly |
| query-knowledge | "What methods exist for dense retrieval?" | Queries knowledge base, groups results, traces to source | `research-engine knowledge retrieve` |
| write-paper | "Write a survey on RAG" | Creates project, proposes outline, writes sections with citations | `knowledge retrieve`, reads papers directly |

### search-papers

The researcher provides a topic or question. Claude expands it into one or more queries, choosing synonyms and related terms to maximize coverage. It runs `research-engine search` for each query, interprets the combined results, and presents a ranked list with explanations of why each paper matters. The skill ends by offering to acquire recommended papers.

Example session:

```
Researcher: /search-papers dense passage retrieval for biomedical QA

Claude:
  I will run three queries to cover this space:
  1. "dense passage retrieval biomedical" --max-results 20
  2. "neural information retrieval clinical" --max-results 15
  3. "domain-specific dense retrieval medical" --recency-bias --max-results 15

  [runs searches, deduplicates, ranks]

  Recommended papers:
  - Xiong et al. 2021, "Approximate Nearest Neighbor..." — foundational DPR for biomedical
  - ...

  Shall I acquire these? Or refine the search?
```

### acquire-papers

The researcher provides one or more identifiers (arXiv IDs, DOIs, or PDF URLs). Claude runs `research-engine acquire` to download PDFs and create metadata, then `research-engine convert --batch` to produce Markdown. If the researcher wants to populate the knowledge base, Claude runs `research-engine extract --batch`. It reports which papers succeeded, which failed, and where files live on disk.

Example session:

```
Researcher: /acquire-papers 2005.11401 2411.18583

Claude:
  Acquiring 2 papers...
  [runs: research-engine acquire 2005.11401 2411.18583]
  Downloaded: 2005.11401 (Lewis et al.), 2411.18583 (Ali et al.)

  Converting to Markdown...
  [runs: research-engine convert --batch --papers-dir papers]
  Converted: 2005.11401.md, 2411.18583.md

  Both papers are ready. Shall I read one of them?
```

### read-papers

The researcher asks to read a paper by name, ID, or topic. Claude lists available papers from `papers/metadata/`, reads the selected Markdown file directly using the Read tool, and discusses its content. No CLI command is needed because Claude reads the files on disk. This is the skill that makes the "Claude reads papers" vision concrete.

Claude starts with structure: section headings, page markers, and overall organization. It then reads sections the researcher cares about, summarizes findings, answers questions, and compares across papers when the researcher has acquired multiple related works.

Example session:

```
Researcher: /read-papers

Claude:
  Available papers:
  1. Lewis et al. 2020 — Retrieval-Augmented Generation for Knowledge-Intensive NLP Tasks
  2. Ali et al. 2024 — Automated Literature Review Using NLP and LLM-Based RAG

  Which paper would you like to read?

Researcher: Read the Lewis paper, focus on the architecture

Claude:
  [reads papers/markdown/2005.11401.md]
  The paper proposes two RAG variants: RAG-Sequence and RAG-Token...
```

### query-knowledge

The researcher asks a question about extracted knowledge across all papers. Claude formulates a query, runs `research-engine knowledge retrieve --json`, and presents results grouped by paper and item type (claim, method, definition, result). When the researcher wants to verify a finding, Claude uses the `--trace` flag to show surrounding context from the source paper.

The skill helps the researcher see across papers: connections between related findings, gaps where evidence is missing, contradictions between claims, and recurring themes.

Example session:

```
Researcher: /query-knowledge what evaluation metrics are used for RAG systems?

Claude:
  [runs: research-engine knowledge retrieve "RAG evaluation metrics" --json]
  [runs: research-engine knowledge retrieve "retrieval augmented generation benchmark" --json]

  Found 12 items across 3 papers:

  Methods:
  - ROUGE scores for generation quality (Lewis 2020, Ali 2024)
  - Exact match accuracy for QA tasks (Lewis 2020)
  - BERTScore for semantic similarity (Ali 2024)
  - RAGAS framework for end-to-end evaluation (Wampler 2025)

  Shall I trace any of these to their source passage?
```

### write-paper

The researcher describes a paper they want to write. Claude creates a project directory under `output/papers/`, proposes an outline, writes section content with inline citations, and refines based on feedback. The workflow has four phases.

In the Initialize phase, Claude creates the directory, a title page with YAML frontmatter, and empty tracking files (outline.yaml, references.yaml). In the Outline phase, Claude queries the knowledge base and reads papers to understand available material, then proposes numbered section files with descriptions. In the Write phase, Claude composes each section with `[AuthorYear]` citations, maintaining references.yaml as citations accumulate. In the Refine phase, the researcher directs revisions to specific sections.

Example session:

```
Researcher: /write-paper literature review on retrieval-augmented generation

Claude:
  Creating project: output/papers/rag-literature-review/

  [creates 00-title-page.md with frontmatter]
  [queries knowledge base for available material]
  [reads key papers]

  Proposed outline:
  01 - Introduction: motivation and scope
  02 - Background: LLM limitations and the retrieval paradigm
  03 - Canonical RAG: DPR + BART architecture
  04 - Advanced Architectures: multi-hop, agentic, self-correcting
  05 - Evaluation: metrics, benchmarks, frameworks
  06 - Applications: QA, literature review, domain-specific
  07 - Conclusion: open problems and future directions

  Shall I proceed with this outline, or adjust it?
```

## The Research Workflow

The five skills connect into a workflow that moves from question to paper. A typical session follows this path:

1. The researcher starts with a topic and invokes `/search-papers` to survey the landscape.
2. From the search results, they select papers and invoke `/acquire-papers` to download and convert them.
3. They use `/read-papers` to read converted papers, discuss findings, and build understanding.
4. They use `/query-knowledge` to search extracted items across all papers, finding connections and gaps.
5. They invoke `/write-paper` to produce a cited document grounded in what they have read.

This path is not strictly linear. Reading a paper often reveals related work worth searching for, which loops back to step 1. Writing a section may expose a gap in the knowledge base, which sends the researcher back to step 2 to acquire more papers or step 4 to check what has already been extracted. The skills support these loops because each one ends with next-step suggestions that chain to other skills.

Table 4 Workflow Transitions

| From | To | When |
|------|----|------|
| search-papers | acquire-papers | Researcher selects papers from search results |
| acquire-papers | read-papers | Papers are downloaded and converted |
| read-papers | search-papers | Reading reveals related work to find |
| read-papers | query-knowledge | Researcher wants to check a claim across papers |
| query-knowledge | read-papers | Researcher wants full context for an extracted item |
| query-knowledge | write-paper | Researcher has enough material to start writing |
| write-paper | query-knowledge | Writing reveals a gap; need more evidence |
| write-paper | acquire-papers | Writing needs a paper not yet acquired |

## Writing New Skills

When adding a new skill to `.claude/commands/`, we follow these conventions.

Use the existing anatomy. Every skill starts with YAML frontmatter (`name`, `description`), an H1 title, an opening paragraph, an Input section, a numbered Workflow section, and a Tips section. Deviating from this structure makes the skill harder to read and breaks the consistency the researcher expects.

Delegate to the CLI. If the Go CLI provides an operation, call it rather than reimplementing the logic in the prompt. This keeps the skill short and ensures the researcher benefits from CLI improvements without skill rewrites.

Chain to other skills. End the workflow with next-step suggestions that reference other skills by slash-command name. This helps the researcher discover the full workflow and move between skills without memorizing the sequence.

Stay focused. Each skill handles one researcher-facing action. If a skill grows to cover two distinct goals, split it into two skills. acquire-papers combines three CLI stages (acquire, convert, extract) because the researcher sees them as one action: "get this paper ready to read." That is one goal with multiple steps, not multiple goals.

Document CLI flags. When a skill invokes a CLI command, list the available flags in a code block so Claude knows the full interface. Flags change as the CLI evolves; keeping the flag list in the skill ensures Claude uses current options.

## References

- `docs/ARCHITECTURE.md` section "Claude Skills" and "Design Decision 6: Claude as Researcher"
- `docs/VISION.md` section "What We Do"
- `docs/specs/product-requirements/prd007-paper-writing.yaml` for write-paper conventions
- `.claude/commands/search-papers.md`
- `.claude/commands/acquire-papers.md`
- `.claude/commands/read-papers.md`
- `.claude/commands/query-knowledge.md`
- `.claude/commands/write-paper.md`
