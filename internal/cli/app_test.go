package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		args           []string
		wantExitCode   int
		wantStdoutPart string
		wantStderrPart string
	}{
		{
			name:           "empty args prints help",
			args:           nil,
			wantExitCode:   0,
			wantStdoutPart: "Usage:",
		},
		{
			name:           "help command",
			args:           []string{"help"},
			wantExitCode:   0,
			wantStdoutPart: "git real once",
		},
		{
			name:           "init command",
			args:           []string{"init"},
			wantExitCode:   0,
			wantStdoutPart: "repository bootstrap complete",
		},
		{
			name:           "status command",
			args:           []string{"status"},
			wantExitCode:   0,
			wantStdoutPart: "enabled=false armed=false",
		},
		{
			name:           "once command uses default grace",
			args:           []string{"once"},
			wantExitCode:   0,
			wantStdoutPart: "120 seconds",
		},
		{
			name:           "start command clamps grace seconds",
			args:           []string{"start", "--grace-seconds=7200"},
			wantExitCode:   0,
			wantStdoutPart: "3600 seconds",
		},
		{
			name:           "arm command",
			args:           []string{"arm"},
			wantExitCode:   0,
			wantStdoutPart: "destructive mode enabled",
		},
		{
			name:           "disarm command",
			args:           []string{"disarm"},
			wantExitCode:   0,
			wantStdoutPart: "dry-run mode enabled",
		},
		{
			name:           "rescue list",
			args:           []string{"rescue", "list"},
			wantExitCode:   0,
			wantStdoutPart: "no backups yet",
		},
		{
			name:           "rescue restore",
			args:           []string{"rescue", "restore", "refs/gitreal/backups/main/123"},
			wantExitCode:   0,
			wantStdoutPart: "would restore refs/gitreal/backups/main/123",
		},
		{
			name:           "unknown command",
			args:           []string{"wat"},
			wantExitCode:   2,
			wantStderrPart: "unknown command: wat",
		},
		{
			name:           "invalid grace value",
			args:           []string{"once", "--grace-seconds=nope"},
			wantExitCode:   2,
			wantStderrPart: "invalid value",
		},
		{
			name:           "unexpected trailing arguments",
			args:           []string{"once", "extra"},
			wantExitCode:   2,
			wantStderrPart: "unexpected arguments: extra",
		},
		{
			name:           "rescue missing subcommand",
			args:           []string{"rescue"},
			wantExitCode:   2,
			wantStderrPart: "expected subcommand list or restore <ref>",
		},
		{
			name:           "rescue list rejects extra args",
			args:           []string{"rescue", "list", "extra"},
			wantExitCode:   2,
			wantStderrPart: "unexpected arguments",
		},
		{
			name:           "rescue restore missing ref",
			args:           []string{"rescue", "restore"},
			wantExitCode:   2,
			wantStderrPart: "expected exactly one backup ref",
		},
		{
			name:           "rescue unknown subcommand",
			args:           []string{"rescue", "wat"},
			wantExitCode:   2,
			wantStderrPart: "unknown subcommand: wat",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			stdout := new(bytes.Buffer)
			stderr := new(bytes.Buffer)

			if got := Run(tc.args, stdout, stderr); got != tc.wantExitCode {
				t.Fatalf("Run(%v) exitCode = %d, want %d", tc.args, got, tc.wantExitCode)
			}

			if tc.wantStdoutPart != "" && !strings.Contains(stdout.String(), tc.wantStdoutPart) {
				t.Fatalf("stdout = %q, want substring %q", stdout.String(), tc.wantStdoutPart)
			}

			if tc.wantStderrPart != "" && !strings.Contains(stderr.String(), tc.wantStderrPart) {
				t.Fatalf("stderr = %q, want substring %q", stderr.String(), tc.wantStderrPart)
			}
		})
	}
}
