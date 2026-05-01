package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestMain(t *testing.T) {
	t.Parallel()

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)

	exitCode := Main(context.Background(), []string{"help"}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("Main() exitCode = %d, want 0", exitCode)
	}

	if got := stdout.String(); !strings.Contains(got, "git real once") {
		t.Fatalf("stdout = %q, want help output", got)
	}

	if got := stderr.String(); got != "" {
		t.Fatalf("stderr = %q, want empty", got)
	}
}

func TestMainVersion(t *testing.T) {
	t.Parallel()

	previousVersion, previousCommit, previousDate := version, commit, date
	t.Cleanup(func() {
		version, commit, date = previousVersion, previousCommit, previousDate
	})
	version, commit, date = "v1.2.3", "abc1234", "2026-05-01T00:00:00Z"

	for _, flag := range []string{"--version", "-V"} {
		stdout := new(bytes.Buffer)
		stderr := new(bytes.Buffer)
		if got := Main(context.Background(), []string{flag}, stdout, stderr); got != 0 {
			t.Fatalf("Main(%s) exit code = %d, want 0", flag, got)
		}
		out := stdout.String()
		for _, want := range []string{"git-real", "v1.2.3", "abc1234", "2026-05-01T00:00:00Z"} {
			if !strings.Contains(out, want) {
				t.Fatalf("Main(%s) stdout = %q, want substring %q", flag, out, want)
			}
		}
		if stderr.Len() != 0 {
			t.Fatalf("Main(%s) stderr = %q, want empty", flag, stderr.String())
		}
	}
}
