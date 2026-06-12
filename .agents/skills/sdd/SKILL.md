---
name: sdd
version: 1.6.0
description: >
  Spec-driven development pipeline with 6 phases: Explore, Requirements,
  Design, Task Plan, Implementation, Review. Enforces human approval gates
  between phases. Also provides a standalone documentation workflow for
  generating or updating project docs without starting a feature pipeline.
  Use when user wants structured feature development, spec-first approach,
  or says "I want to add feature X", "new feature", "implement", "build",
  "generate documentation", "update docs", "actualize the documentation".
  Keywords: spec, requirements, design document, TDD plan, task plan,
  implementation, code review, pipeline, approval gates, WHEN/SHALL,
  generate docs, update docs, documentation queue.
---

# Spec-Driven Development

You are operating in **spec-driven development mode**.
This project uses a 6-phase pipeline with human approval gates between each phase.

## Pipeline

```
Explore → [APPROVE] → Requirements → [APPROVE] → Design → [APPROVE] → Task Plan → [APPROVE] → Implementation → [APPROVE] → Review → [APPROVE] → Done
```

Each phase has a dedicated prompt template. Read the template for the **current** phase before generating any output.

## Quick Reference

### Core Commands

| Action | Command |
|--------|---------|
| Check state | `sh ./scripts/pipeline.sh status` |
| Start feature | `sh ./scripts/pipeline.sh init <name>` |
| Register output | `sh ./scripts/pipeline.sh artifact [path]` |
| Advance phase | `sh ./scripts/pipeline.sh approve` (only after user says "approve") |
| Register task set | `sh ./scripts/pipeline.sh task-init T-1 T-2 ...` (implementation; creates `tasks/T-N.md` stubs) |
| Set task status | `sh ./scripts/pipeline.sh task T-N [done\|wip\|blocked]` |
| List / next tasks | `sh ./scripts/pipeline.sh tasks` · `task-next` · `task-reset` |
| View revisions | `sh ./scripts/pipeline.sh revisions [phase]` |
| List features | `sh ./scripts/pipeline.sh history` |
| Inject artifact | `sh ./scripts/pipeline.sh inject <phase> <path>` |
| Abandon pipeline | `sh ./scripts/pipeline.sh abandon [feature]` |
| Validate config | `sh ./scripts/pipeline.sh config-check` |
| Diagnose setup | `sh ./scripts/pipeline.sh doctor` |
| Multi-feature | Add `--feature <name>` before any command |
| All flags & help | `sh ./scripts/pipeline.sh help` |

### Decision Points

At these moments, **ask the user** before running a command:

#### Documentation updates

When `docs-check` reports issues, ASK the user (already described in Pre-flight Checklist step 3).

| User answer | Command |
|------------|--------|
| "generate docs" | `pipeline.sh docs-init --all` |
| "update docs" | `pipeline.sh docs-init --update` |
| "skip" / "пропустить" | (no command) |

**Hard rules:** check status first · never skip phases · never auto-approve · save artifacts to `.spec/features/<feature>/` · max 3 revisions then ask user

**Config:** `.spec/config.yaml` → `context`, `rules.<phase>`, `test_skill`, `test_reference`, `docs_dir`, `doc_freshness_days`

**Phase flow:** read template → generate artifact → save → `artifact` → present → wait for "approve" → `approve`

## Phases

| # | Phase          | Template                        | Produces                        |
|---|----------------|---------------------------------|---------------------------------|
| 1 | Explore        | `./templates/explore.md`        | Exploration & research document |
| 2 | Requirements   | `./templates/requirements.md`   | Formal requirements document    |
| 3 | Design         | `./templates/design.md`         | Architecture & design document  |
| 4 | Task Plan      | `./templates/task-plan.md`      | TDD implementation plan         |
| 5 | Implementation | `./templates/implementation.md` | Implementation report           |
| 6 | Review         | `./templates/review.md`         | Code review document            |

## State Machine

Pipeline state is managed by `./scripts/pipeline.sh` (POSIX sh, zero dependencies). All feature commands are listed in **Quick Reference → Core Commands** above. For the standalone documentation commands (`docs-init`, `docs-next`, `docs-done`, `docs-status`, `docs-reset`), see `./templates/docs-maintenance.md`.

### Parallel Pipelines

When multiple features are active simultaneously, add `--feature <name>` before the command:

```sh
sh ./scripts/pipeline.sh --feature auth-flow status
sh ./scripts/pipeline.sh --feature payment approve
```

Without the flag, the pipeline auto-detects the active feature. If more than one is active, it will error and prompt you to use `--feature`.

## Project Configuration

If the file `.spec/config.yaml` exists in the project root, read it before starting any phase. See `.spec/config.yaml.example` for a template with all supported keys.

> **Format limitation:** the pipeline parser reads flat `key: value` pairs only. Nested YAML structures, multi-line values, and quoted strings are not supported.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `context` | string | — | Project-wide background for ALL phases |
| `rules.<phase>` | string | — | Phase-specific rules (supplement template) |
| `rules.docs` | string | — | Rules for documentation generation |
| `test_skill` | string | — | Skill name for delegated test generation |
| `test_reference` | string | — | Glob/paths to representative test files |
| `docs_dir` | string | `.spec` | Directory for project documentation |
| `doc_freshness_days` | integer | `30` | Days before a generated doc is stale |

Phase-specific rule keys: `rules.explore`, `rules.requirements`, `rules.design`, `rules.task-plan`, `rules.implementation`, `rules.review`, `rules.docs`.

Injection order: **context → phase rules → template instructions.**

If the file does not exist, skip this step.

## Standalone Documentation Workflow

If the user requests documentation generation or update **without referring to a feature** (e.g. *"generate docs"*, *"update documentation"*, *"actualize the docs"*, *"refresh AUTH.md"*) — **do NOT run `pipeline.sh init`**. This is a standalone workflow with its own state machine.

1. Read `./templates/docs-maintenance.md` § Standalone Documentation Workflow.
2. Run `pipeline.sh docs-init [--all|--update|<template>...]` based on user intent.
3. Choose execution strategy (subagent recommended when available, sequential as fallback).
4. Drive the queue: `docs-next` → generate → `docs-done` (sequential), or dispatch up to 3 subagents in parallel (subagent mode).

The standalone workflow is independent from feature pipelines — it does not create `.spec/features/<feature>/`, does not require approvals, and runs purely from `.spec/.docs-queue.kv` state.

## Pre-flight Checklist

Before starting any pipeline work, follow these steps in order:

1. **Check pipeline state**: run `pipeline.sh status`.
   - If exactly one active pipeline exists → resume from the current phase. Do NOT run `init` again.
   - If no active pipeline → proceed to step 2.
   - If multiple active pipelines → ask the user which feature to work on, then use `--feature <name>` with all subsequent commands.
2. **Read project config**: check if `.spec/config.yaml` exists.
   - If yes → read it, apply `context` to all phases, note `rules.*` for each phase.
   - If no → proceed without config (defaults apply).
3. **Check documentation** (MUST — do not skip this step): run `pipeline.sh docs-check`.
   - **Docs directory missing** → suggest: *"Project documentation (<docs_dir>/) not found. I can generate it to better understand your codebase. Say 'generate docs' or 'skip'."* **Wait for the user's response** before proceeding. This is a soft gate — the pipeline works without documentation, but the user must explicitly acknowledge (say 'generate docs' or 'skip').
   - **Docs exist, stale files found** → suggest: *"Some docs are outdated (<file>: <N> days old). Regenerate before starting? Say 'update docs' or 'skip'."* **Wait for the user's response.** If user agrees, read `./templates/docs-maintenance.md` for the Stale doc regeneration workflow.
   - **Docs exist, all fresh** → use as supplementary context for ALL phases. Read `<docs_dir>/README.md` for the documentation map.
   - If user says **"generate docs"** or **"update docs"**: read `./templates/docs-maintenance.md`, follow the workflow. Generated documentation files go to `<docs_dir>/` (default: `.spec/`), **NOT** to `.spec/features/<feature>/`.
   - **Do NOT proceed to step 4 until the user responds** to the documentation suggestion.
4. **Start pipeline**: run `pipeline.sh init <feature-name>`.

For documentation generation, staleness checks, and regeneration workflows, read `./templates/docs-maintenance.md`.

## When to Use This Pipeline

**Use the pipeline for:**
- New features ("add user authentication", "implement search")
- Significant changes to existing features (new behavior, API changes, schema migrations)
- Bug fixes that require investigation and design (root cause unknown, multiple components affected)

**Do NOT use the pipeline for:**
- Trivial changes: typo fixes, config tweaks, single-field additions, comment updates
- Dependency updates with no code changes
- Pure refactors with no behavioral change (unless they are large and risky)

For trivial changes, just make the change directly — no pipeline needed. The skill is designed for work that **benefits from structured thinking before coding**.

### Fast-track mode

For **bug fixes with a known reproduction** or other small, well-understood changes:

- All 6 phases still apply — do not skip phases.
- Each phase produces a **minimal artifact**: 1-paragraph exploration, 1–2 requirements, focused design (CPs only for the bug scenario), 4–5 tasks (RED→GREEN→CODE→VERIFY→GATE), brief implementation report, short review.
- Each template contains a "Fast-track mode" section with phase-specific minimums. Follow those rules when fast-track applies.

**When to activate:** The agent activates fast-track when the user describes a bug with a known reproduction step, or a small, scoped change where investigation is unnecessary. At the start, announce: *"Using fast-track mode — all 6 phases, minimal artifacts."* If the user says "full pipeline", switch to the standard (non-abbreviated) flow.

**Scope:** This pipeline is designed for a **single project or monorepo**. It is not intended for features that span multiple independent repositories. Within a monorepo, use one `.spec/` directory at the repository root.

## Rules

1. **MUST check status first.** Run `pipeline.sh status` before doing anything. Never generate phase output without checking status. If multiple active pipelines exist, use `--feature <name>` with all commands.
2. **Never skip phases.** Follow the order: explore → requirements → design → task-plan → implementation → review.
3. **Never auto-approve.** Wait for the user to explicitly say "approve" or equivalent.
4. **Read the template.** Before generating output for a phase, read the corresponding template file.
5. **Save artifacts.** Save phase artifacts (explore, requirements, design, task-plan, implementation, review) to `.spec/features/<feature>/` and register them with `pipeline.sh artifact`. **Project documentation** (README.md, ARCHITECTURE.md, DOMAIN.md, etc.) goes to `<docs_dir>/` (default: `.spec/`), NOT to `.spec/features/<feature>/` — these are separate directories with separate purposes.
6. **Each phase produces one artifact** that becomes input for the next phase.
7. **Artifacts are cumulative.** Each phase reads all prior artifacts.
8. **Revision limit.** If the user rejects the same artifact 3 times in a row, stop generating and ask: "We've gone through 3 revisions — could you clarify what's missing or what direction you'd prefer?" Do not continue revising without explicit guidance.
9. **Surface uncertainty.** If you are unsure about intent, scope, or technical approach — say so explicitly. State the assumption you would make and ask the user to confirm or correct it. Never silently assume.
10. **Write in the user's language.** Detect the user's language from their first message and use it for ALL pipeline artifacts and conversational replies. What stays in English:
    - Formal grammar keywords: `WHEN`, `SHALL`, `the system`
    - Requirement IDs: `REQ-X.Y`
    - Task IDs: `T-N`
    - Instruction keywords: `CRITICAL`, `IMPORTANT`, `NOTE`, `DO NOT`, `GOAL`
    - Correctness Property format: `Property N`, `Category`, `For all`, `Validates`
    - Code identifiers, file paths, shell commands, Mermaid node labels
    - Documentation in `<docs_dir>/` (`.spec/`) — always English (see `templates/docs/README.md`)

    Everything else — prose, section headers, descriptions, interview questions, explanations — is written in the user's language.

## Error Recovery

- **Revising an artifact:** Overwrite the file, re-register with `pipeline.sh artifact`, and present the updated version to the user. The previous version is automatically saved as a revision in the feature’s `revisions/` directory. Use `pipeline.sh revisions` to view past revisions.
- **Diagnosing setup issues:** Run `pipeline.sh doctor` to verify required tools, phase templates, `.spec` writability, and git availability.


## Documentation Maintenance

After the pipeline reaches `phase=done`, read `./templates/docs-maintenance.md` § Documentation Maintenance to check if project documentation needs updating.

## Quick Start (for the agent)

When the user says something like "I want to add feature X":

1. Run the **Pre-flight Checklist** (status → config → docs-check → init).
2. For each phase in order (Explore → Requirements → Design → Task Plan → Implementation → Review): read the phase template (see **Phases**) → do the work → save to `.spec/features/<feature>/<phase>.md` → `pipeline.sh artifact` → present to the user → wait for "approve" → `pipeline.sh approve`.
3. **Review is informational:** present the findings and verdict, then wait for instructions. Fix only what the user asks, regenerate the review, and re-present before the final `approve`.
4. After the final `approve` (Review → done): check documentation (see **Documentation Maintenance**). Branch/PR/merge handling is left to the user — the skill does not run git.
