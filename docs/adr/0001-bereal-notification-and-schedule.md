# 0001 — BeReal-style notification noticeability and configurable scheduler

- Status: accepted
- Date: 2026-05-01

## Context

Two complaints surfaced about the BeReal-inspired challenge flow:

1. Linux desktop notifications were dismissed within seconds, no sound was emitted, and the prototype's terminal bell (`\a`) never made it into the production code. Users in IDEs missed the alert and the deadline passed without their awareness.
2. The challenge scheduler always fired hourly. The original BeReal experience is "one chance, when you least expect it" — the hourly cadence trains the user out of the surprise.

This ADR records the decisions made during a planning interview to address both complaints in a single change.

## Decisions

### D1. Daily mode known limitation
`DailySchedule.Next` does not persist a "last fired today" marker. If `git real start` is interrupted and restarted, the daily mode may fire again the same day. The risk is accepted for this iteration and documented in `README.md` and `commandStatus`. A future change can store `gitreal.lastFiredDate` in git config to suppress same-day re-fires.

### D2. Default `scheduleMode` for fresh `init`
Stays at `hourly`. Daily is opt-in until D1 is resolved, so users get the legacy behavior unless they explicitly opt into the BeReal-flavored mode. Avoids shipping a bug as the default.

### D3. Reminders during the grace window
Send two additional reminders inside the grace window: T-30s and T-10s before the deadline. If `graceSeconds` is too short for a reminder to be in the future, that reminder is skipped (e.g. with `graceSeconds=15`, both reminders are skipped; with `20`, only T-10 fires). Three total alerts give the user multiple chances to notice.

### D4. `gitreal.sound` opt-out
A boolean key, default `true`. When `false`, both the BEL byte to stderr and the best-effort `paplay`/`canberra-gtk-play` invocations are skipped. This is the only audio-related setting; we did not want to add per-platform tuning.

### D5. Linux urgency policy
Hard-coded `notify-send -u critical -t 0`. No config key for urgency. The whole point of this change is to fix "I cannot notice the notification", and a tunable urgency would defeat that. Documented as a deliberate design choice.

### D6. Schedule package location
New package `internal/schedule/` with a `Schedule` interface and three strategies (`HourlySchedule`, `DailySchedule`, `IntervalSchedule`). Keeps the `cli` package focused on command dispatch and makes the strategy reusable for a future `git real daemon` subcommand.

### D7. Late-grace concept
Postponed. BeReal's "posted late" semantics could be added later as a `gitreal.lateGraceSeconds` key, but it is orthogonal to noticeability and broadens the PR. Tracked separately.

### D8. Daily mode in this PR
Implement `DailySchedule` despite D1, because the configurability story without daily would be unsatisfying for users explicitly asking for "once a day at a random time". The known limitation is surfaced through `commandStatus` output and `README.md` so users can decide whether to opt in.

## Consequences

- New git config keys: `gitreal.scheduleMode`, `gitreal.dailyWindowStart`, `gitreal.dailyWindowEnd`, `gitreal.intervalMinutes`, `gitreal.sound`.
- `notify-send` is now invoked with `-u critical -t 0 -a git-real -i dialog-warning` on Linux. macOS notifications include `sound name "Sosumi"`. Windows Toast notifications also call `[console]::beep(880,300)`.
- The `notify` helper takes a `repository` argument so it can read `gitreal.sound`. All six call sites in `runChallenge` are updated.
- `nextRandomSlot` is preserved as a one-line shim that delegates to `schedule.HourlySchedule{}.Next` so the existing test still compiles.
- Future work: implement `gitreal.lastFiredDate` to fix D1, and consider `gitreal.lateGraceSeconds` for D7.
