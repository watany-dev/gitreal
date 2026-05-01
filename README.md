# GitReal

BeReal, but for Git.  
When the notification hits, you have 2 minutes to push.  
Miss it, and your local commits become unreal.

## What It Is

GitReal is a Git subcommand distributed as a `git-real` executable. When that binary is on your `PATH`, Git automatically exposes it as:

```bash
git real
```

That means the user-facing command is `git real`, while the shipped executable name is `git-real`.

## MVP Commands

```bash
git real init
git real status
git real once
git real start
git real arm
git real disarm
git real rescue list
git real rescue restore <ref>
```

## Quick Start

```bash
# install with go install
go install github.com/watany-dev/gitreal/cmd/git-real@latest

# initialize the current repository
git real init

# inspect current state
git real status

# run one dry-run challenge now
git real once

# run the foreground scheduler
git real start
```

`git real once`, `git real start`, `git real arm`, and `git real disarm` require `git real init` first. `git real status` and `git real rescue ...` remain available before initialization.

## Safety Model

- Default mode is dry-run.
- Destructive behavior must be explicitly armed with `git real arm`.
- Before any reset, GitReal stores the current `HEAD` under `refs/gitreal/backups/...`.
- `git real rescue restore <ref>` also backs up the current `HEAD` before restoring the selected backup ref.
- Recovery is done with `git real rescue list` and `git real rescue restore <ref>`.

## Current Implementation

- Language: Go
- Binary name: `git-real`
- Git integration: invoke Git commands directly instead of reading `.git` internals
- Repository config: `gitreal.enabled`, `gitreal.armed`, `gitreal.graceSeconds`
- Scheduler model:
  - `git real once` runs one challenge immediately
  - `git real start` is the current foreground scheduler entrypoint with hourly random timing
  - `git real daemon` remains a future background/service entrypoint and is not part of the current beta

Current package layout:

- `cmd/git-real`: executable entrypoint
- `internal/cli`: command dispatch and challenge flow
- `internal/git`: Git command wrapper and backup helpers
- `internal/notify`: best-effort desktop notifications
- `internal/challenge`: grace-period constants and normalization

## Current Constraints

- An upstream branch is required for challenge execution.
- Detached `HEAD` is not supported.
- Desktop notifications are best-effort and fall back to terminal output when unavailable.
- `git real daemon` is not implemented in the current beta.

## Local Build Target

```bash
go build -o git-real ./cmd/git-real
```

## Development

Bootstrap and validate locally:

```bash
go mod download
make fmt
make test
make check
```

Checks included in `make check`:

- formatting check with `gofmt`
- linting with `go vet` and `staticcheck`
- type-check/compile with `go test -run '^$' ./...`
- dead code detection with `deadcode`
- coverage gate at `95%`

Property-based tests are included with the standard library `testing/quick`.

## Expected Install Paths

Current beta distribution targets:

- GitHub Releases with per-OS archives for macOS, Linux, and Windows
- `go install`

Example commands:

```bash
go install github.com/watany-dev/gitreal/cmd/git-real@latest
```

Release archives are published as:

- `git-real_darwin_amd64.tar.gz`
- `git-real_darwin_arm64.tar.gz`
- `git-real_linux_amd64.tar.gz`
- `git-real_linux_arm64.tar.gz`
- `git-real_windows_amd64.zip`
- `SHA256SUMS`

## Notes

Detailed design notes, command rationale, Git plumbing choices, and the original prototype notes are stored in [docs/development-memo.md](/workspaces/gitreal/docs/development-memo.md).
