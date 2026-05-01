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

## Safety Model

- Default mode is dry-run.
- Destructive behavior must be explicitly armed with `git real arm`.
- Before any reset, GitReal stores the current `HEAD` under `refs/gitreal/backups/...`.
- Recovery is done with `git real rescue list` and `git real rescue restore <ref>`.

## Planned Implementation

- Language: Go
- Binary name: `git-real`
- Git integration: invoke Git commands directly instead of reading `.git` internals
- Scheduler model:
  - `git real once` runs one challenge immediately
  - `git real start` runs in the foreground with hourly random timing
  - `git real daemon` is the future background/service entrypoint

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

Initial distribution targets:

- GitHub Releases with per-OS archives
- Homebrew tap
- `go install`

Example commands:

```bash
brew install yourname/tap/git-real
go install github.com/yourname/git-real/cmd/git-real@latest
```

## Notes

Detailed design notes, command rationale, Git plumbing choices, and the initial single-file Go prototype are stored in [docs/development-memo.md](/workspaces/gitreal/docs/development-memo.md).
