# AGENTS.md

This repository uses a repo-local Codex guide and skills.

## Scope

This file defines the default working rules for Codex in this repository.
Repo-local skills live under `skills/`.

Note: the reference repository uses `.agents/skills/`, but this workspace mounts hidden top-level agent directories read-only. Keep the same skill content here under `skills/`.

## Project Overview

GitReal is a Git subcommand distributed as a `git-real` executable and invoked by users as `git real`.

The current product shape from `README.md` and `docs/development-memo.md` is:

- 2-minute push challenge triggered by notification timing
- default dry-run behavior
- destructive mode enabled only by explicit `git real arm`
- backup and recovery through `refs/gitreal/backups/...`
- planned implementation language: Go

Current MVP command set:

```text
git real init
git real status
git real once
git real start
git real arm
git real disarm
git real rescue list
git real rescue restore <ref>
```

## Current Source Of Truth

Until the Go implementation exists, treat these as the primary project documents:

- `README.md`
- `docs/development-memo.md`
- `AGENTS.md`

Keep those three aligned.

## Working Baseline

1. Inspect the repository before making assumptions about language, framework, build system, or test runner.
2. Prefer the commands and conventions already present in the repo over introducing new tooling.
3. Keep changes small, explicit, and easy to validate.
4. Update related docs when behavior, commands, or project structure change.

## Discovery Order

Before making changes, check the files that define how the project works:

- `README.md`
- `docs/development-memo.md`
- CI workflows under `.github/workflows/`
- language/build manifests such as `package.json`, `pyproject.toml`, `Cargo.toml`, `go.mod`, `build.zig`, `Makefile`
- source entry points and top-level docs under `docs/`

If those files do not exist, say so explicitly and proceed with the smallest reasonable assumption.

## Completion Requirements

Do not consider work complete until you have run the narrowest relevant validation available in the repository.

Examples:

- existing test command
- existing formatter or linter
- existing typecheck or build command

For this repository today:

- if only docs changed, verify `README.md`, `docs/development-memo.md`, and `AGENTS.md` stay consistent
- if Go scaffolding exists, prefer the repo-native commands first
- once `go.mod` and `cmd/git-real` exist, the expected baseline is `go test ./...` and `go build ./cmd/git-real`

If the repository does not yet define runnable validation commands, report that clearly instead of inventing a fake completion signal.

## Engineering Approach

### TDD When Practical

When the repo already has tests or a clear place for them:

1. write or update a failing test
2. implement the minimum change
3. refactor without changing behavior

### Tidy First

Separate structural cleanup from behavioral changes where practical.

- reduce nesting with guard clauses
- remove dead code when encountered
- extract helpers when they clarify intent
- normalize similar code paths
- keep comments short and only where code is not self-evident

### Iteration Size

Split work into the smallest meaningful increment and finish that increment completely before moving on.

## Planned Architecture

Use the current memo as the default implementation direction unless newer code or docs replace it:

- `cmd/git-real/main.go` for CLI entry
- `internal/git/` for Git command wrappers
- `internal/challenge/` for timing and challenge flow
- `internal/notify/` for desktop notifications
- `internal/config/` for Git config access
- `internal/daemon/` for foreground scheduler and later background support

Prefer invoking Git commands over reading `.git` internals directly.

## Documentation Rules

- `README.md`: user-facing usage, install, and operation
- `docs/`: design notes, ADRs, requirements, implementation details
- `AGENTS.md`: repo-specific instructions for Codex

When a command, workflow, or architecture decision changes, update the relevant document in the same task when possible.

For this repo in particular:

- user-facing command behavior belongs in `README.md`
- command rationale and architecture notes belong in `docs/development-memo.md`
- Codex workflow rules belong in `AGENTS.md`

## Planning Rules

When asked for a plan:

1. ground the plan in the current repo state
2. identify missing design or requirement inputs
3. keep steps independently verifiable
4. note risks, dependencies, and affected files

Run the `update-plan` skill before finalizing a substantial plan.

## Available Repo Skills

- `skills/update-design/`
- `skills/update-docs/`
- `skills/update-plan/`
- `skills/grill-me/`
