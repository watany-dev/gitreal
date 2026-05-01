package git

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const backupRefPrefix = "refs/gitreal/backups/"

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
	safeBranch := strings.ReplaceAll(branch, string(filepath.Separator), "-")
	safeBranch = strings.ReplaceAll(safeBranch, "/", "-")

	timestamp := fmt.Sprintf("%s-%09d", now.UTC().Format("20060102T150405Z"), now.UTC().Nanosecond())
	backupRef := fmt.Sprintf("%s%s/%s", backupRefPrefix, safeBranch, timestamp)
	_, err := r.run("update-ref", backupRef, "HEAD")
	if err != nil {
		return "", err
	}

	return backupRef, nil
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

func IsBackupRef(ref string) bool {
	return strings.HasPrefix(ref, backupRefPrefix)
}

func (r *Repository) run(args ...string) (string, error) {
	return r.runner.run(r.root, args...)
}
