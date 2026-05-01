---
name: update-plan
description: Stress-test an implementation plan against the current codebase, design docs, and repo constraints before presenting it. Use for substantial plans, phased work, refactors, or multi-file features.
---

# update-plan

Run this immediately before presenting a substantial implementation plan.

## Phase 1: Gather Inputs

Collect the minimum relevant context:

1. the current draft plan
2. `AGENTS.md`
3. related design docs, ADRs, requirements, roadmap items, issue notes, and `docs/development-memo.md`
4. the source files and manifests the plan touches

## Phase 2: Validate the Plan

Check the plan for:

1. correct assumptions about the current codebase
2. independently verifiable steps
3. separation of structural cleanup from behavior changes
4. explicit validation for each risky step
5. impact on docs, config, CI, and tests

## Phase 3: Find Gaps

Flag these issues by priority:

- `P0`: plan contradicts code or requirements
- `P1`: step ordering, missing validation, or unclear ownership
- `P2`: useful follow-up work or documentation debt

## Phase 4: Rewrite the Plan

Return the improved plan with:

1. ordered steps
2. affected files or directories
3. validation per step
4. risks or dependencies
5. missing design or documentation prerequisites

Use this output shape:

```markdown
## Plan Check

Issues:
- P0: ...
- P1: ...
- P2: ...

Revised plan:
1. ...
2. ...
3. ...
```

## Plan Quality Bar

- each step should change one coherent thing
- each step should have an observable completion signal
- broad refactors must justify why they cannot be split
- if design is too vague, route back through `update-design`
