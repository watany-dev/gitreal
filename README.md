# GitReal

GitReal is a Git subcommand that turns "I should push later" into a deadline.

When a challenge fires, you have 2 minutes to push your local commits. If you miss the window, GitReal can reset your branch back to its upstream state. By default, it stays in dry-run mode, so you can try the workflow before allowing destructive behavior.

The distributed binary is named `git-real`, and Git exposes it to users as:

```bash
git real
```

## Why You Would Use It

- You want a push habit, not just a reminder.
- You want the pressure of a timer without giving up recovery options.
- You want to try it safely before enabling real resets.

## Quick Start

Install:

```bash
go install github.com/watany-dev/gitreal/cmd/git-real@latest
```

Try it in a repository:

```bash
git real init
git real status
git real once
```

`git real once`, `git real start`, `git real arm`, and `git real disarm` require `git real init` first. `git real status` and `git real rescue ...` are still available before initialization.

If you want GitReal to run continuously in the foreground:

```bash
git real start
```

## What Happens In Practice

1. Run `git real init` once per repository.
2. GitReal stores repo-local config and starts in dry-run mode.
3. A challenge checks whether your current branch is ahead of its upstream.
4. If you push in time, nothing happens.
5. If you miss the deadline:

- in dry-run mode, GitReal only tells you what it would have reset
- in armed mode, GitReal backs up `HEAD` and resets the branch to `@{u}`

## Safety First

GitReal is intentionally conservative:

- Default mode is dry-run.
- Destructive behavior requires an explicit `git real arm`.
- Before any reset, GitReal stores the current `HEAD` under `refs/gitreal/backups/...`.
- `git real rescue restore <ref>` also backs up the current `HEAD` before restoring a backup.
- Dirty worktree changes are stashed and then restored when possible.

If you want real enforcement for the current repository:

```bash
git real arm
```

To go back to safe mode:

```bash
git real disarm
```

## Recovery

List available backups:

```bash
git real rescue list
```

Restore one:

```bash
git real rescue restore <ref>
```

## Commands

```bash
git real init
git real status
git real once [--grace-seconds=120]
git real start [--grace-seconds=120]
git real arm
git real disarm
git real rescue list
git real rescue restore <ref>
```

Command intent:

- `git real init`: enable GitReal for the current repository and write default config
- `git real status`: show current repo state, upstream, and ahead count
- `git real once`: run one challenge immediately
- `git real start`: stay in the foreground and schedule hourly random challenges over time
- `git real arm`: allow real resets for missed deadlines
- `git real disarm`: return to dry-run mode
- `git real rescue ...`: inspect and restore backup refs

## Current Limits

This beta currently expects:

- the current branch has an upstream branch
- the repository is not in detached `HEAD`
- desktop notifications may fail and fall back to terminal output
- `git real start` is the current scheduler entrypoint
- `git real daemon` is not implemented yet

## Configuration

GitReal stores settings in Git config:

```bash
git config --local gitreal.enabled true
git config --local gitreal.armed false
git config --local gitreal.graceSeconds 120
git config --local gitreal.scheduleMode hourly
git config --local gitreal.sound true
```

Current keys:

- `gitreal.enabled`
- `gitreal.armed`
- `gitreal.graceSeconds`
- `gitreal.scheduleMode` — `hourly` (default), `daily`, or `interval`
- `gitreal.dailyWindowStart` / `gitreal.dailyWindowEnd` — `HH:MM` window for daily mode (defaults `09:00` / `22:00`)
- `gitreal.intervalMinutes` — used when mode is `interval` (default `60`)
- `gitreal.sound` — when `true`, GitReal also writes a terminal bell and plays a system sound on Linux

### Schedule modes

- `hourly`: one challenge per hour at a random second within the hour. This is the legacy behavior.
- `daily`: one challenge per day at a random time within the configured window. **Known limitation: if `git real start` is interrupted and restarted on the same day, daily mode may fire again the same day.** This will be addressed in a future release.
- `interval`: one challenge every `gitreal.intervalMinutes` minutes with random jitter so it does not feel mechanical.

### Notification noticeability

On Linux, GitReal sends `notify-send -u critical -t 0` so the alert stays on screen until you dismiss it. macOS notifications include a system sound (`Sosumi`) and Windows Toast notifications also play a console beep. During the 2-minute grace window GitReal sends additional reminders at 30 seconds and 10 seconds before the deadline so a missed first alert is less likely to cost you. When `gitreal.sound=true` (the default), each alert is also accompanied by a terminal bell on stderr and a best-effort `paplay` / `canberra-gtk-play` on Linux.

## Build From Source

```bash
go build -o git-real ./cmd/git-real
```

## Development

Project checks:

```bash
go mod download
make fmt
make test
make check
```

`make check` runs formatting, linting, type-check compilation, dead-code detection, and the coverage gate.

## Releases

Current beta distribution targets:

- `go install`
- GitHub Releases for macOS, Linux, and Windows

Release archives are published as:

- `git-real_darwin_amd64.tar.gz`
- `git-real_darwin_arm64.tar.gz`
- `git-real_linux_amd64.tar.gz`
- `git-real_linux_arm64.tar.gz`
- `git-real_windows_amd64.zip`
- `SHA256SUMS`

## More Detail

Design notes and implementation rationale live in [docs/development-memo.md](/workspaces/gitreal/docs/development-memo.md).
