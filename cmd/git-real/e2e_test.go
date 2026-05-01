package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMainEndToEnd(t *testing.T) {
	workDir := t.TempDir()
	originDir := filepath.Join(workDir, "origin.git")
	repoDir := filepath.Join(workDir, "repo")

	runGit(t, workDir, "init", "--bare", originDir)
	runGit(t, workDir, "clone", originDir, repoDir)
	runGit(t, repoDir, "checkout", "-b", "main")
	runGit(t, repoDir, "config", "user.name", "GitReal Test")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "config", "commit.gpgsign", "false")

	writeFile(t, filepath.Join(repoDir, "file.txt"), "base\n")
	runGit(t, repoDir, "add", "file.txt")
	runGit(t, repoDir, "commit", "-m", "base")
	runGit(t, repoDir, "push", "-u", "origin", "HEAD")

	stdout, stderr, exitCode := runMain(t, repoDir, "init")
	if exitCode != 0 {
		t.Fatalf("init exitCode = %d, stderr = %q", exitCode, stderr)
	}
	if !strings.Contains(stdout, "GitReal initialized for:") {
		t.Fatalf("stdout = %q, want init output", stdout)
	}

	stdout, stderr, exitCode = runMain(t, repoDir, "status")
	if exitCode != 0 {
		t.Fatalf("status exitCode = %d, stderr = %q", exitCode, stderr)
	}
	for _, want := range []string{"enabled: true", "armed: false", "upstream: origin/main", "ahead: 0"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("status stdout = %q, want substring %q", stdout, want)
		}
	}

	writeFile(t, filepath.Join(repoDir, "file.txt"), "base\nlocal\n")
	runGit(t, repoDir, "commit", "-am", "local")

	_, stderr, exitCode = runMain(t, repoDir, "once", "--grace-seconds=1")
	if exitCode != 0 {
		t.Fatalf("once dry-run exitCode = %d, stderr = %q", exitCode, stderr)
	}
	if ahead := strings.TrimSpace(runGitOutput(t, repoDir, "rev-list", "--count", "@{u}..HEAD")); ahead != "1" {
		t.Fatalf("ahead after dry-run = %q, want 1", ahead)
	}

	_, stderr, exitCode = runMain(t, repoDir, "arm")
	if exitCode != 0 {
		t.Fatalf("arm exitCode = %d, stderr = %q", exitCode, stderr)
	}

	stdout, stderr, exitCode = runMain(t, repoDir, "once", "--grace-seconds=1")
	if exitCode != 0 {
		t.Fatalf("once armed exitCode = %d, stderr = %q", exitCode, stderr)
	}
	if !strings.Contains(stdout, "backup ref: refs/gitreal/backups/main/") {
		t.Fatalf("stdout = %q, want backup ref", stdout)
	}
	if ahead := strings.TrimSpace(runGitOutput(t, repoDir, "rev-list", "--count", "@{u}..HEAD")); ahead != "0" {
		t.Fatalf("ahead after penalty = %q, want 0", ahead)
	}

	stdout, stderr, exitCode = runMain(t, repoDir, "rescue", "list")
	if exitCode != 0 {
		t.Fatalf("rescue list exitCode = %d, stderr = %q", exitCode, stderr)
	}
	backupRef := firstNonEmptyLine(stdout)
	if !strings.HasPrefix(backupRef, "refs/gitreal/backups/main/") {
		t.Fatalf("backupRef = %q, want GitReal backup ref", backupRef)
	}

	stdout, stderr, exitCode = runMain(t, repoDir, "rescue", "restore", backupRef)
	if exitCode != 0 {
		t.Fatalf("rescue restore exitCode = %d, stderr = %q", exitCode, stderr)
	}
	if !strings.Contains(stdout, "previous HEAD backed up to: refs/gitreal/backups/main/") {
		t.Fatalf("stdout = %q, want current HEAD backup output", stdout)
	}
	if ahead := strings.TrimSpace(runGitOutput(t, repoDir, "rev-list", "--count", "@{u}..HEAD")); ahead != "1" {
		t.Fatalf("ahead after rescue restore = %q, want 1", ahead)
	}
}

func runMain(t *testing.T, dir string, args ...string) (string, string, int) {
	t.Helper()

	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir(%q) error = %v", dir, err)
	}
	defer func() {
		if err := os.Chdir(previousDir); err != nil {
			t.Fatalf("restore Chdir(%q) error = %v", previousDir, err)
		}
	}()

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	exitCode := Main(context.Background(), args, stdout, stderr)
	return stdout.String(), stderr.String(), exitCode
}

func TestMainOnceCancelledByContext(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	originDir := filepath.Join(workDir, "origin.git")
	repoDir := filepath.Join(workDir, "repo")

	runGit(t, workDir, "init", "--bare", originDir)
	runGit(t, workDir, "clone", originDir, repoDir)
	runGit(t, repoDir, "checkout", "-b", "main")
	runGit(t, repoDir, "config", "user.name", "GitReal Test")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "config", "commit.gpgsign", "false")

	writeFile(t, filepath.Join(repoDir, "file.txt"), "base\n")
	runGit(t, repoDir, "add", "file.txt")
	runGit(t, repoDir, "commit", "-m", "base")
	runGit(t, repoDir, "push", "-u", "origin", "HEAD")

	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	defer func() { _ = os.Chdir(previousDir) }()

	if exit := Main(context.Background(), []string{"init"}, new(bytes.Buffer), new(bytes.Buffer)); exit != 0 {
		t.Fatalf("init exit = %d, want 0", exit)
	}
	if exit := Main(context.Background(), []string{"arm"}, new(bytes.Buffer), new(bytes.Buffer)); exit != 0 {
		t.Fatalf("arm exit = %d, want 0", exit)
	}

	writeFile(t, filepath.Join(repoDir, "file.txt"), "base\nlocal\n")
	runGit(t, repoDir, "commit", "-am", "local")

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	exitCode := Main(ctx, []string{"once", "--grace-seconds=30"}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("once after cancel exit = %d, stderr = %q", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "interrupted") {
		t.Fatalf("stdout = %q, want interrupt notice", stdout.String())
	}
	if ahead := strings.TrimSpace(runGitOutput(t, repoDir, "rev-list", "--count", "@{u}..HEAD")); ahead != "1" {
		t.Fatalf("ahead after cancel = %q, want 1 (no penalty)", ahead)
	}
	out := runGitOutput(t, repoDir, "for-each-ref", "refs/gitreal/backups/")
	if strings.TrimSpace(out) != "" {
		t.Fatalf("backup refs created on cancel: %q", out)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=GitReal Test",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=GitReal Test",
		"GIT_COMMITTER_EMAIL=test@example.com",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v: %s", args, err, output)
	}
}

func runGitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v: %s", args, err, output)
	}
	return string(output)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func firstNonEmptyLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}
