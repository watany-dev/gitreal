package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestMain(t *testing.T) {
	t.Parallel()

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)

	exitCode := Main([]string{"help"}, stdout, stderr)
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
