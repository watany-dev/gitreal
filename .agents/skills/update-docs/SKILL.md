---
name: update-docs
description: Synchronize README, docs, and AGENT guidance with the actual codebase and tooling. Use when implementation changed commands, behavior, configuration, architecture, or project structure.
---

# update-docs

Update repository documentation to match the current implementation.

## Phase 1: Inspect the Current Source of Truth

Read the smallest set of files that define behavior:

1. entry points and changed source files
2. build and dependency manifests
3. CI workflows
4. `README.md`, `docs/`, `AGENT.md`
5. `docs/development-memo.md` when present

Do not update docs from memory. Derive every command and path from the repository.

## Phase 2: Update the Right Document

Use this split:

- `README.md`: how to install, run, configure, and use the project
- `docs/`: design rationale, ADRs, requirements, internal notes
- `AGENT.md`: repo-specific instructions for Codex

For GitReal specifically, keep `README.md` and `docs/development-memo.md` aligned on:

1. executable name `git-real`
2. user-facing command `git real`
3. safety model around `arm`, backups, and rescue
4. planned Go-based architecture

Common updates:

1. build, test, lint, format, or release commands
2. config file names and option descriptions
3. module layout and entry points
4. examples and sample outputs
5. workflow or validation requirements

## Phase 3: Consistency Check

Verify consistency across docs:

1. file paths
2. command names
3. option flags
4. module names
5. stated prerequisites

If the repo has no established command for something, say that plainly instead of inventing one.

## Phase 4: Report the Update

Summarize in this form:

```markdown
## Documentation Update

Updated:
- <file>: <what changed>

Open gaps:
- ...

Remaining assumptions:
- ...
```

## Editing Rules

- keep README user-facing and concise
- keep developer guidance close to the code it explains
- prefer exact commands over prose-only instructions
- remove stale claims instead of softening them
