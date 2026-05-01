package cli

import (
	"flag"
	"fmt"
	"io"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/watany-dev/gitreal/internal/challenge"
	ggit "github.com/watany-dev/gitreal/internal/git"
	"github.com/watany-dev/gitreal/internal/notify"
)

type repository interface {
	Root() string
	SetConfigBool(key string, value bool) error
	SetConfigInt(key string, value int) error
	ConfigBool(key string, fallback bool) bool
	ConfigInt(key string, fallback int) int
	CurrentBranch() (string, error)
	Upstream() (string, error)
	FetchQuiet() error
	AheadCount() (int, error)
	BackupHead(branch string, now time.Time) (string, error)
	StashDirtyWorktree(message string) (bool, error)
	StashPop() error
	ResetHard(ref string) error
	RescueRefs() ([]string, error)
}

type app struct {
	discoverRepo     func(path string) (repository, error)
	now              func() time.Time
	sleep            func(time.Duration)
	sendNotification func(title, message string) error
	rng              *rand.Rand
	stdout           io.Writer
	stderr           io.Writer
	startIterations  int
}

func Run(args []string, stdout, stderr io.Writer) int {
	return newApp(stdout, stderr).run(args)
}

func newApp(stdout, stderr io.Writer) *app {
	return &app{
		discoverRepo: func(path string) (repository, error) {
			return ggit.Discover(path)
		},
		now:              time.Now,
		sleep:            time.Sleep,
		sendNotification: notify.Send,
		rng:              rand.New(rand.NewSource(time.Now().UnixNano())),
		stdout:           stdout,
		stderr:           stderr,
	}
}

func (a *app) run(args []string) int {
	if len(args) == 0 {
		printHelp(a.stdout)
		return 0
	}

	switch args[0] {
	case "help", "-h", "--help":
		printHelp(a.stdout)
		return 0
	case "init":
		return a.commandInit()
	case "status":
		return a.commandStatus()
	case "once":
		return a.commandOnce(args[1:])
	case "start":
		return a.commandStart(args[1:])
	case "arm":
		return a.commandArm()
	case "disarm":
		return a.commandDisarm()
	case "rescue":
		return a.commandRescue(args[1:])
	default:
		fmt.Fprintf(a.stderr, "git-real: unknown command: %s\n", args[0])
		printHelp(a.stderr)
		return 2
	}
}

func (a *app) commandInit() int {
	repo, err := a.discoverRepo(".")
	if err != nil {
		return a.fail(err)
	}

	if err := repo.SetConfigBool("gitreal.enabled", true); err != nil {
		return a.fail(err)
	}

	if err := repo.SetConfigBool("gitreal.armed", false); err != nil {
		return a.fail(err)
	}

	if err := repo.SetConfigInt("gitreal.graceSeconds", challenge.DefaultGraceSeconds); err != nil {
		return a.fail(err)
	}

	fmt.Fprintf(a.stdout, "GitReal initialized for: %s\n", repo.Root())
	fmt.Fprintln(a.stdout, "Mode: dry-run")
	fmt.Fprintln(a.stdout, "Run: git real once")
	return 0
}

func (a *app) commandStatus() int {
	repo, err := a.discoverRepo(".")
	if err != nil {
		return a.fail(err)
	}

	branch := "<unknown>"
	if value, err := repo.CurrentBranch(); err == nil {
		branch = value
	}

	upstream := "<none>"
	aheadText := "unknown"
	if value, err := repo.Upstream(); err == nil {
		upstream = value
		_ = repo.FetchQuiet()
		if ahead, err := repo.AheadCount(); err == nil {
			aheadText = strconv.Itoa(ahead)
		}
	}

	fmt.Fprintf(a.stdout, "repo: %s\n", repo.Root())
	fmt.Fprintf(a.stdout, "enabled: %t\n", repo.ConfigBool("gitreal.enabled", false))
	fmt.Fprintf(a.stdout, "armed: %t\n", repo.ConfigBool("gitreal.armed", false))
	fmt.Fprintf(a.stdout, "grace-seconds: %d\n", challenge.NormalizeGraceSeconds(repo.ConfigInt("gitreal.graceSeconds", challenge.DefaultGraceSeconds)))
	fmt.Fprintf(a.stdout, "branch: %s\n", branch)
	fmt.Fprintf(a.stdout, "upstream: %s\n", upstream)
	fmt.Fprintf(a.stdout, "ahead: %s\n", aheadText)
	return 0
}

func (a *app) commandOnce(args []string) int {
	repo, err := a.discoverRepo(".")
	if err != nil {
		return a.fail(err)
	}

	graceSeconds, err := resolveGraceSeconds(args, repo, a.stderr)
	if err != nil {
		return 2
	}

	if err := a.requireInitialized(repo); err != nil {
		return a.fail(err)
	}

	if err := a.runChallenge(repo, graceSeconds, repo.ConfigBool("gitreal.armed", false)); err != nil {
		return a.fail(err)
	}

	return 0
}

func (a *app) commandStart(args []string) int {
	repo, err := a.discoverRepo(".")
	if err != nil {
		return a.fail(err)
	}

	graceSeconds, err := resolveGraceSeconds(args, repo, a.stderr)
	if err != nil {
		return 2
	}

	if err := a.requireInitialized(repo); err != nil {
		return a.fail(err)
	}

	return a.runStart(repo, graceSeconds, a.startIterations)
}

func (a *app) runStart(repo repository, graceSeconds int, iterations int) int {
	base := a.now()
	fmt.Fprintf(a.stdout, "GitReal started for %s\n", repo.Root())

	completed := 0
	for iterations <= 0 || completed < iterations {
		next := nextRandomSlot(base, a.rng)
		fmt.Fprintf(a.stdout, "next challenge: %s\n", next.Format(time.RFC3339))
		a.sleepUntil(next)

		if err := a.runChallenge(repo, graceSeconds, repo.ConfigBool("gitreal.armed", false)); err != nil {
			fmt.Fprintf(a.stderr, "git-real: %v\n", err)
		}

		base = next.Add(time.Hour)
		completed++
	}

	return 0
}

func (a *app) commandArm() int {
	repo, err := a.discoverRepo(".")
	if err != nil {
		return a.fail(err)
	}

	if err := a.requireInitialized(repo); err != nil {
		return a.fail(err)
	}

	if err := repo.SetConfigBool("gitreal.armed", true); err != nil {
		return a.fail(err)
	}

	fmt.Fprintln(a.stdout, "GitReal is now armed for this repository.")
	return 0
}

func (a *app) commandDisarm() int {
	repo, err := a.discoverRepo(".")
	if err != nil {
		return a.fail(err)
	}

	if err := a.requireInitialized(repo); err != nil {
		return a.fail(err)
	}

	if err := repo.SetConfigBool("gitreal.armed", false); err != nil {
		return a.fail(err)
	}

	fmt.Fprintln(a.stdout, "GitReal is now in dry-run mode for this repository.")
	return 0
}

func (a *app) commandRescue(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(a.stderr, "git-real rescue: expected subcommand list or restore <ref>")
		return 2
	}

	repo, err := a.discoverRepo(".")
	if err != nil {
		return a.fail(err)
	}

	switch args[0] {
	case "list":
		if len(args) != 1 {
			fmt.Fprintln(a.stderr, "git-real rescue list: unexpected arguments")
			return 2
		}

		refs, err := repo.RescueRefs()
		if err != nil {
			return a.fail(err)
		}

		if len(refs) == 0 {
			fmt.Fprintln(a.stdout, "No GitReal backup refs found.")
			return 0
		}

		fmt.Fprintln(a.stdout, strings.Join(refs, "\n"))
		return 0
	case "restore":
		if len(args) != 2 {
			fmt.Fprintln(a.stderr, "git-real rescue restore: expected exactly one backup ref")
			return 2
		}

		backupRef := args[1]
		if !ggit.IsBackupRef(backupRef) {
			fmt.Fprintln(a.stderr, "git-real rescue restore: ref must start with refs/gitreal/backups/")
			return 2
		}

		return a.restoreBackupRef(repo, backupRef)
	default:
		fmt.Fprintf(a.stderr, "git-real rescue: unknown subcommand: %s\n", args[0])
		return 2
	}
}

func (a *app) restoreBackupRef(repo repository, backupRef string) int {
	branch, err := repo.CurrentBranch()
	if err != nil {
		return a.fail(err)
	}

	currentBackupRef, err := repo.BackupHead(branch, a.now())
	if err != nil {
		return a.fail(err)
	}

	stashMessage := fmt.Sprintf("gitreal preserve worktree before rescue restore %s", currentBackupRef)
	stashed, err := repo.StashDirtyWorktree(stashMessage)
	if err != nil {
		return a.fail(err)
	}

	if err := repo.ResetHard(backupRef); err != nil {
		return a.fail(err)
	}

	if stashed {
		if err := repo.StashPop(); err != nil {
			fmt.Fprintln(a.stdout, "stash pop failed; your stash remains available via git stash list")
		}
	}

	fmt.Fprintf(a.stdout, "Current branch reset to backup ref: %s\n", backupRef)
	fmt.Fprintf(a.stdout, "previous HEAD backed up to: %s\n", currentBackupRef)
	return 0
}

func (a *app) runChallenge(repo repository, graceSeconds int, armed bool) error {
	branch, err := repo.CurrentBranch()
	if err != nil {
		return err
	}

	upstream, err := repo.Upstream()
	if err != nil {
		return err
	}

	if err := repo.FetchQuiet(); err != nil {
		fmt.Fprintf(a.stdout, "preflight fetch failed; continuing with last known upstream state: %v\n", err)
	}

	ahead, err := repo.AheadCount()
	if err != nil {
		return err
	}

	fmt.Fprintf(a.stdout, "repo: %s\n", repo.Root())
	fmt.Fprintf(a.stdout, "branch: %s\n", branch)
	fmt.Fprintf(a.stdout, "upstream: %s\n", upstream)
	fmt.Fprintf(a.stdout, "ahead: %d\n", ahead)

	if ahead == 0 {
		a.notify("GitReal", "No unpushed commits. Nothing to do.")
		fmt.Fprintln(a.stdout, "nothing to do: no unpushed commits")
		return nil
	}

	deadline := a.now().Add(time.Duration(graceSeconds) * time.Second)
	fmt.Fprintf(a.stdout, "deadline: %s\n", deadline.Format(time.RFC3339))
	a.notify("GitReal", fmt.Sprintf("%s has %d unpushed commit(s). Push before %s.", branch, ahead, deadline.Format("15:04:05")))

	a.sleepUntil(deadline)

	if err := repo.FetchQuiet(); err != nil {
		a.notify("GitReal", "fetch failed; punishment skipped for safety.")
		fmt.Fprintln(a.stdout, "fetch failed after deadline; punishment skipped for safety")
		return nil
	}

	aheadAfter, err := repo.AheadCount()
	if err != nil {
		return err
	}

	if aheadAfter == 0 {
		a.notify("GitReal", "Push confirmed. You are GitReal.")
		fmt.Fprintln(a.stdout, "push confirmed")
		return nil
	}

	if !armed {
		a.notify("GitReal dry-run", fmt.Sprintf("%d commit(s) would be reset.", aheadAfter))
		fmt.Fprintf(a.stdout, "dry-run: would reset %d commit(s) to @{u}\n", aheadAfter)
		return nil
	}

	backupRef, err := repo.BackupHead(branch, a.now())
	if err != nil {
		return err
	}

	stashMessage := fmt.Sprintf("gitreal preserve worktree before penalty %s", backupRef)
	stashed, err := repo.StashDirtyWorktree(stashMessage)
	if err != nil {
		return err
	}

	if err := repo.ResetHard("@{u}"); err != nil {
		return err
	}

	if stashed {
		if err := repo.StashPop(); err != nil {
			fmt.Fprintln(a.stdout, "stash pop failed; your stash remains available via git stash list")
		}
	}

	a.notify("GitReal", fmt.Sprintf("Local commits made unreal. Backup: %s", backupRef))
	fmt.Fprintf(a.stdout, "backup ref: %s\n", backupRef)
	fmt.Fprintf(a.stdout, "restore: git real rescue restore %s\n", backupRef)
	return nil
}

func (a *app) notify(title, message string) {
	if err := a.sendNotification(title, message); err != nil {
		fmt.Fprintf(a.stdout, "notification: %s: %s\n", title, message)
	}
}

func (a *app) sleepUntil(target time.Time) {
	duration := target.Sub(a.now())
	if duration > 0 {
		a.sleep(duration)
	}
}

func resolveGraceSeconds(args []string, repo repository, stderr io.Writer) (int, error) {
	graceSeconds, explicit, err := parseGraceSeconds(args, stderr)
	if err != nil {
		return 0, err
	}

	if explicit {
		return graceSeconds, nil
	}

	return challenge.NormalizeGraceSeconds(repo.ConfigInt("gitreal.graceSeconds", challenge.DefaultGraceSeconds)), nil
}

func parseGraceSeconds(args []string, stderr io.Writer) (int, bool, error) {
	fs := flag.NewFlagSet("git-real", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	graceSeconds := fs.Int("grace-seconds", challenge.DefaultGraceSeconds, "challenge window in seconds")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "git-real: %v\n", err)
		return 0, false, err
	}

	if fs.NArg() != 0 {
		err := fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
		fmt.Fprintf(stderr, "git-real: %v\n", err)
		return 0, false, err
	}

	explicit := false
	fs.Visit(func(flag *flag.Flag) {
		if flag.Name == "grace-seconds" {
			explicit = true
		}
	})

	return challenge.NormalizeGraceSeconds(*graceSeconds), explicit, nil
}

func nextRandomSlot(base time.Time, rng *rand.Rand) time.Time {
	windowStart := base.Truncate(time.Hour)
	offset := time.Duration(rng.Intn(3600)) * time.Second
	slot := windowStart.Add(offset)
	if !slot.After(base) {
		slot = windowStart.Add(time.Hour + time.Duration(rng.Intn(3600))*time.Second)
	}

	return slot
}

func printHelp(w io.Writer) {
	fmt.Fprintln(w, `git-real - BeReal-inspired punishment CLI for Git

Usage:
  git real init
  git real status
  git real once [--grace-seconds=120]
  git real start [--grace-seconds=120]
  git real arm
  git real disarm
  git real rescue list
  git real rescue restore <backup-ref>`)
}

func (a *app) fail(err error) int {
	fmt.Fprintf(a.stderr, "git-real: %v\n", err)
	return 1
}

func (a *app) requireInitialized(repo repository) error {
	if repo.ConfigBool("gitreal.enabled", false) {
		return nil
	}

	return fmt.Errorf("repository is not initialized for GitReal; run: git real init")
}
