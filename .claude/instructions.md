<!-- Copyright Mesh Intelligence Inc., 2026. All rights reserved. -->

# Research Pipeline Instructions

This repository contains tools and workflows for managing academic research papers - from consumption to production.

## Repository Purpose

This is a personal research toolkit that:
1. Downloads and organizes research papers
2. Converts PDFs to markdown format for better searchability
3. Extracts and structures knowledge from papers
4. Uses the accumulated knowledge base to brainstorm and write new papers

## Communication Style

- Keep explanations direct and technical
- Avoid unnecessary formality or marketing language
- Use clear, functional descriptions over creative flourishes
- Prefer straightforward terminology over jargon

## Code and Documentation Standards

- Prioritize clarity and maintainability
- Document the "why" behind non-obvious choices
- Keep scripts focused on single responsibilities
- Use meaningful variable and function names that explain intent

## Project Structure Principles

- Separate stages of the pipeline into distinct components
- Make each transformation step (PDF → MD → Knowledge → Paper) independently usable
- Maintain clean boundaries between data processing and knowledge extraction
- Keep configuration separate from code

## Knowledge Base Management

- Structure extracted knowledge for easy retrieval and reuse
- Design for both manual review and automated querying
- Maintain traceability back to source papers
- Support incremental updates as new papers are added

## Paper Generation Approach

- Treat the knowledge base as the authoritative source
- Enable both brainstorming (exploration) and drafting (synthesis)
- Support citation tracking from knowledge base to output
- Allow iterative refinement of generated content

## Build Tooling

- `mage` is installed globally. Call it directly (e.g. `mage stats`) — do not prepend PATH exports.

## When Working on This Repository

Focus on practical utility over theoretical perfection. This is a working system for real research workflows, not a demonstration project. Optimize for daily use and easy maintenance.