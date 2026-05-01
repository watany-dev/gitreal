package git

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	backupRefPrefix    = "refs/gitreal/backups/"
	maxSafeBranchBytes = 64
)

// backupRefPattern enforces the exact shape that BackupHead produces:
//
//	refs/gitreal/backups/<safeBranch>/<YYYYMMDDTHHMMSSZ>-<nanoseconds>
//
// The safeBranch segment is restricted to allowlisted characters so the
// downstream `git update-ref` and `git reset --hard` cannot be tricked into
// resolving git revision syntax (`@{N}`, `^`, `~`, `:`) or shell metacharacters.
var backupRefPattern = regexp.MustCompile(
	`^refs/gitreal/backups/[A-Za-z0-9._-]+/[0-9]{8}T[0-9]{6}Z-[0-9]{9}$`,
)

var safeBranchAllowed = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

type runner interface {
	run(dir string, args ...string) (string, error)
}

type commandRunner struct{}

func (commandRunner) run(dir string, args ...string) (string, error) {
	fullArgs := append([]string{"-C", dir}, args...)
	cmd := exec.Command("git", fullArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s failed: %s", strings.Join(args, " "), strings.TrimSpace(string(out)))
	}

	return string(out), nil
}

type Repository struct {
	root   string
	runner runner
}

func Discover(path string) (*Repository, error) {
	return discover(path, commandRunner{})
}

func discover(path string, r runner) (*Repository, error) {
	out, err := r.run(path, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("not inside a Git repository")
	}

	return &Repository{
		root:   strings.TrimSpace(out),
		runner: r,
	}, nil
}

func (r *Repository) Root() string {
	return r.root
}

func (r *Repository) SetConfigBool(key string, value bool) error {
	boolValue := "false"
	if value {
		boolValue = "true"
	}

	_, err := r.run("config", "--local", key, boolValue)
	return err
}

func (r *Repository) SetConfigInt(key string, value int) error {
	_, err := r.run("config", "--local", key, strconv.Itoa(value))
	return err
}

func (r *Repository) ConfigBool(key string, fallback bool) bool {
	out, err := r.run("config", "--bool", "--get", key)
	if err != nil {
		return fallback
	}

	switch strings.ToLower(strings.TrimSpace(out)) {
	case "true", "yes", "on", "1":
		return true
	case "false", "no", "off", "0":
		return false
	default:
		return fallback
	}
}

func (r *Repository) ConfigInt(key string, fallback int) int {
	out, err := r.run("config", "--int", "--get", key)
	if err != nil {
		return fallback
	}

	value, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return fallback
	}

	return value
}

func (r *Repository) CurrentBranch() (string, error) {
	out, err := r.run("symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return "", fmt.Errorf("detached HEAD is not supported")
	}

	return strings.TrimSpace(out), nil
}

func (r *Repository) Upstream() (string, error) {
	out, err := r.run("rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if err != nil {
		return "", fmt.Errorf("no upstream configured; run: git push -u origin HEAD")
	}

	return strings.TrimSpace(out), nil
}

func (r *Repository) FetchQuiet() error {
	_, err := r.run("fetch", "--quiet", "--prune")
	return err
}

func (r *Repository) AheadCount() (int, error) {
	out, err := r.run("rev-list", "--count", "@{u}..HEAD")
	if err != nil {
		return 0, err
	}

	value := strings.TrimSpace(out)
	if value == "" {
		return 0, nil
	}

	count, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (r *Repository) BackupHead(branch string, now time.Time) (string, error) {
	safeBranch := sanitizeBranchSegment(branch)

	timestamp := fmt.Sprintf("%s-%09d", now.UTC().Format("20060102T150405Z"), now.UTC().Nanosecond())
	backupRef := fmt.Sprintf("%s%s/%s", backupRefPrefix, safeBranch, timestamp)

	if !backupRefPattern.MatchString(backupRef) {
		// Defense in depth: this should be impossible because
		// sanitizeBranchSegment guarantees the allowed character class, but
		// we never want to hand a malformed ref to `git update-ref`.
		return "", fmt.Errorf("refusing to write malformed backup ref: %q", backupRef)
	}

	_, err := r.run("update-ref", backupRef, "HEAD")
	if err != nil {
		return "", err
	}

	return backupRef, nil
}

// sanitizeBranchSegment maps an arbitrary branch name (including hostile or
// non-ASCII names) to a single ref path segment matching [A-Za-z0-9._-]+.
// If the input would collapse to empty or to a leading dot/dash (which git
// rejects), it falls back to a "branch-<sha256[:12]>" hash so we always
// produce a stable, valid identifier.
func sanitizeBranchSegment(branch string) string {
	collapsed := safeBranchAllowed.ReplaceAllString(branch, "-")
	collapsed = strings.Trim(collapsed, "-.")

	for strings.Contains(collapsed, "--") {
		collapsed = strings.ReplaceAll(collapsed, "--", "-")
	}

	if collapsed == "" || len(collapsed) > maxSafeBranchBytes {
		sum := sha256.Sum256([]byte(branch))
		return "branch-" + hex.EncodeToString(sum[:6])
	}
	return collapsed
}

func (r *Repository) StashDirtyWorktree(message string) (bool, error) {
	out, err := r.run("status", "--porcelain=v1", "-z")
	if err != nil {
		return false, err
	}

	if out == "" {
		return false, nil
	}

	_, err = r.run("stash", "push", "--include-untracked", "--message", message)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (r *Repository) StashPop() error {
	_, err := r.run("stash", "pop")
	return err
}

func (r *Repository) ResetHard(ref string) error {
	_, err := r.run("reset", "--hard", ref)
	return err
}

func (r *Repository) RescueRefs() ([]string, error) {
	out, err := r.run("for-each-ref", backupRefPrefix, "--format=%(refname)")
	if err != nil {
		return nil, err
	}

	text := strings.TrimSpace(out)
	if text == "" {
		return nil, nil
	}

	return strings.Split(text, "\n"), nil
}

// IsBackupRef reports whether ref is a well-formed GitReal backup ref produced
// by BackupHead. It performs a full pattern match (not just a prefix check) so
// that user-supplied refs containing git revision syntax such as `@{-1}`,
// `^`, `~`, or `:` cannot reach `git reset --hard` via `rescue restore`.
func IsBackupRef(ref string) bool {
	return backupRefPattern.MatchString(ref)
}

func (r *Repository) run(args ...string) (string, error) {
	return r.runner.run(r.root, args...)
}
