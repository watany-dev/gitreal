---
name: update-design
description: Review or update repository design documents, compare them with the actual codebase, and surface concrete gaps before implementation proceeds. Use when working on docs/design, ADRs, architecture notes, or when a plan depends on design quality.
---

# update-design

Review design documents against the current repository state and improve them before implementation depends on them.

## Phase 1: Gather Context

Read only what is relevant:

1. `docs/design/*.md`, `docs/adr/*.md`, and architecture notes under `docs/`
2. `docs/development-memo.md` when present
3. the source directories touched by the design
4. build manifests and CI workflows that constrain implementation
5. `AGENTS.md`

If design docs do not exist, report that first and switch to proposing a minimal design outline derived from the current memo and README.

## Phase 2: Evaluate the Design

Score each relevant document across these categories:

1. module and ownership boundaries
2. data flow and control flow
3. interfaces, types, and configuration surface
4. failure modes, validation, and observability
5. testing and rollout strategy

Use a 100-point scale. Treat 90+ as ready for implementation.

## Phase 3: Check Against the Code

Confirm whether the design matches reality:

1. documented modules exist
2. names and paths match the codebase
3. described behaviors are implemented or clearly marked as planned
4. validation steps match the actual tooling

Call out both directions:

- documented but not implemented
- implemented but undocumented

## Phase 4: Produce Improvements

Return a concise report in this shape:

```markdown
## Design Review

### <document>
| Category | Score |
|---|---:|
| Module boundaries | XX |
| Data flow | XX |
| Interfaces | XX |
| Failure modes | XX |
| Testing and rollout | XX |

Average: XX.X

Issues:
- ...

Recommended edits:
- ...

Code mismatch:
- ...
```

## Editing Rules

- keep design docs implementation-oriented
- prefer concrete file paths, types, commands, and invariants
- do not restate obvious code when a short reference is enough
- if a design decision matters later, suggest an ADR under `docs/adr/`
