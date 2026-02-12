<!-- Copyright Mesh Intelligence Inc., 2026. All rights reserved. -->

# Sample Research Prompts

We collect example research sessions that demonstrate how the five Claude skills compose into a workflow. Each example shows the slash command the researcher types, what Claude does in response, and the expected output. These examples serve as a starting point for researchers learning the tool and as test cases for skill behavior. They cover the common patterns: finding papers, searching patents, acquiring and reading, building a knowledge base, and writing.

## Finding Papers on a Topic

The researcher asks Claude to survey a research area. Claude expands the topic into multiple queries to cover different angles.

```text
You:    /search-papers dense passage retrieval for question answering

Claude:
  I will run three queries to cover this space:

  1. research-engine search "dense passage retrieval question answering" --max-results 20
  2. research-engine search "neural information retrieval QA" --max-results 15 --recency-bias
  3. research-engine search --keywords "DPR,retrieval augmented" --max-results 15

  [runs searches, deduplicates across backends]

  Found 38 unique results across arXiv, Semantic Scholar, and OpenAlex.

  Recommended papers:
  1. Karpukhin et al. 2020 — "Dense Passage Retrieval for Open-Domain Question Answering"
     Foundational DPR paper. Defines the bi-encoder architecture used in most subsequent work.
  2. Lewis et al. 2020 — "Retrieval-Augmented Generation for Knowledge-Intensive NLP Tasks"
     Introduces RAG, combining DPR with seq2seq generation.
  3. Izacard & Grave 2021 — "Leveraging Passage Retrieval with Generative Models"
     Fusion-in-Decoder: feeds multiple retrieved passages to the generator.

  Shall I acquire these papers, or refine the search?
```

Claude formulates three queries with different strategies: one broad, one biased toward recent work, one using keyword filters. It deduplicates results that appear across multiple backends and presents a curated list with explanations.

## Patent Landscape Search

The researcher wants to understand the IP landscape around a technology. Claude uses the `--patents` flag to search only PatentsView.

```text
You:    /search-papers patents on transformer-based language models

Claude:
  I will search the patent literature for transformer language models:

  1. research-engine search "transformer language model" --patents --max-results 20
  2. research-engine search "attention mechanism natural language" --patents --max-results 15
  3. research-engine search --keywords "transformer,pretraining" --patents --from 2017-06-01

  [runs patent searches]

  Found 28 unique patents from PatentsView.

  Notable patents:
  1. US10452978 — "Attention-Based Sequence Transduction Neural Networks"
     Inventors: Vaswani, Shazeer, et al. Filed 2017. The original transformer patent.
  2. US11334766 — "Pre-training Language Representations"
     Inventors: Devlin, Chang, et al. Filed 2018. Covers the BERT architecture.
  3. US20230082245A1 — "Large Language Model Training Methods"
     Published application, 2023. Covers distributed training techniques.

  Shall I acquire any of these patents, or search with different terms?
```

Claude recognizes the researcher's intent ("patents on...") and uses `--patents` to avoid mixing in academic results. It presents patent numbers, inventor names, and filing dates.

## Mixed Paper and Patent Research

The researcher wants both academic papers and patents on the same topic to understand the full landscape.

```text
You:    /search-papers graph neural networks for drug discovery, include patents

Claude:
  I will search both academic and patent sources:

  Academic:
  1. research-engine search "graph neural network drug discovery" --max-results 20
  2. research-engine search "molecular property prediction GNN" --max-results 15 --recency-bias

  Patents:
  3. research-engine search "graph neural network pharmaceutical" --patents --max-results 15
  4. research-engine search --author "Gilmer" --patents

  [runs all four searches]

  Academic papers (25 unique):
  - Gilmer et al. 2017 — "Neural Message Passing for Quantum Chemistry"
  - Stokes et al. 2020 — "A Deep Learning Approach to Antibiotic Discovery"
  ...

  Patents (12 unique):
  - US11087218 — "Graph Convolutional Network for Molecular Property Prediction"
  - US20220028491A1 — "Drug-Target Interaction Prediction Using GNN"
  ...

  Shall I acquire selected papers and patents together?
```

Claude runs separate academic and patent queries, presents results grouped by type, and offers to acquire them in a single batch.

## Acquiring and Reading Papers

The researcher selects papers from search results and wants to read them.

```text
You:    /acquire-papers 2005.11401 US10452978

Claude:
  Acquiring 2 items (1 paper, 1 patent)...

  research-engine acquire 2005.11401 US10452978

  Downloaded:
  - 2005.11401 (Lewis et al. — Retrieval-Augmented Generation) → papers/raw/2005.11401.pdf
  - US10452978 (Vaswani et al. — Attention-Based Sequence Transduction) → papers/raw/US10452978.pdf

  Converting to Markdown...
  research-engine convert --batch --papers-dir papers

  Converted:
  - papers/markdown/2005.11401.md
  - papers/markdown/US10452978.md

  Both items are ready to read. Shall I read one of them?

You:    /read-papers the Lewis RAG paper

Claude:
  Reading papers/markdown/2005.11401.md...

  "Retrieval-Augmented Generation for Knowledge-Intensive NLP Tasks"
  Lewis et al., 2020

  Structure:
  - Abstract (p.1)
  - 1. Introduction (p.1-2)
  - 2. Methods (p.2-4): RAG-Sequence and RAG-Token models
  - 3. Experiments (p.4-7)
  - 4. Results (p.7-9)
  - 5. Related Work (p.9-10)

  The paper proposes two RAG variants. RAG-Sequence uses the same retrieved
  document for the entire output sequence. RAG-Token can use different
  documents for each output token...

  What aspect would you like to explore further?
```

## Building a Knowledge Base

The researcher acquires several papers with extraction enabled, then queries the knowledge base.

```text
You:    /acquire-papers 2005.11401 2009.14794 2110.08678
        Please extract knowledge items too.

Claude:
  Acquiring 3 papers...
  [downloads and converts all three]

  Extracting knowledge items...
  research-engine extract --batch --model claude-sonnet-4-5-20250929 --api-key $ANTHROPIC_API_KEY

  Extracted:
  - 2005.11401: 23 items (8 claims, 6 methods, 4 definitions, 5 results)
  - 2009.14794: 18 items (5 claims, 7 methods, 2 definitions, 4 results)
  - 2110.08678: 15 items (4 claims, 5 methods, 3 definitions, 3 results)

  Storing in knowledge base...
  research-engine knowledge store

  56 items indexed. Ready for queries.

You:    /query-knowledge what retrieval methods are used in RAG systems?

Claude:
  research-engine knowledge retrieve "retrieval methods RAG" --json
  research-engine knowledge retrieve "dense passage retrieval architecture" --json

  Found 14 items across 3 papers:

  Methods:
  - Dense Passage Retrieval (DPR) bi-encoder with BERT (Lewis 2020, Karpukhin 2020)
  - Maximum Inner Product Search with FAISS index (Lewis 2020)
  - Contrastive learning for passage encoder training (Karpukhin 2020)
  - ColBERT late interaction for efficient retrieval (Khattab 2020)

  Claims:
  - "RAG-Token outperforms RAG-Sequence on generation tasks" (Lewis 2020, Section 4)
  - "DPR outperforms BM25 on 4 of 5 QA benchmarks" (Karpukhin 2020, Section 5)

  Shall I trace any of these to their source passage?
```

## Writing a Literature Review

The researcher asks Claude to write a survey based on acquired papers.

```text
You:    /write-paper survey on retrieval-augmented generation

Claude:
  Creating project: output/papers/rag-survey/

  [creates 00-title-page.md with YAML frontmatter]
  [queries knowledge base for available material]
  [reads key papers for context]

  Available material: 56 knowledge items across 3 papers.

  Proposed outline:
  01 - Introduction: motivation for combining retrieval with generation
  02 - Background: language model limitations and the retrieval paradigm
  03 - Core Architecture: DPR retriever and RAG generator
  04 - Variants: RAG-Sequence, RAG-Token, Fusion-in-Decoder
  05 - Training: joint and independent training strategies
  06 - Evaluation: metrics and benchmarks
  07 - Conclusion: open problems and future directions

  Shall I proceed with this outline, or adjust it?

You:    Proceed. Start with the introduction.

Claude:
  Writing output/papers/rag-survey/01-introduction.md...

  # Introduction

  Large language models store factual knowledge in their parameters during
  pretraining, but this knowledge is static and incomplete. When a model
  encounters a question about recent events or specialized domains, it must
  either hallucinate an answer or decline to respond. Retrieval-augmented
  generation addresses this limitation by coupling the language model with
  an external knowledge source that can be updated independently
  [Lewis2020].

  Lewis et al. introduced the RAG framework, which retrieves relevant
  passages from a document corpus and conditions the generator on both the
  input query and the retrieved context [Lewis2020]. This approach yields
  two benefits: the model can access knowledge beyond its training data,
  and its outputs can be grounded in verifiable sources...

  [continues writing with citations]

  Section complete. Shall I continue with section 02, or revise this one?
```

## References

- [eng01-claude-research-skills](eng01-claude-research-skills.md) for skill design conventions
- [eng02-patent-search](eng02-patent-search.md) for patent search setup and usage
- [ARCHITECTURE.md](../ARCHITECTURE.md) for system design
