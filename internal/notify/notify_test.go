package notify

import (
	"errors"
	"testing"
)

func TestCommand(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		goos    string
		wantCmd string
		wantOK  bool
	}{
		{name: "darwin", goos: "darwin", wantCmd: "osascript", wantOK: true},
		{name: "linux", goos: "linux", wantCmd: "notify-send", wantOK: true},
		{name: "windows", goos: "windows", wantCmd: "powershell", wantOK: true},
		{name: "unsupported", goos: "plan9", wantOK: false},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotCmd, gotArgs, gotOK := command(tc.goos, "GitReal", "test")
			if gotOK != tc.wantOK {
				t.Fatalf("command(%q) ok = %t, want %t", tc.goos, gotOK, tc.wantOK)
			}

			if gotCmd != tc.wantCmd {
				t.Fatalf("command(%q) cmd = %q, want %q", tc.goos, gotCmd, tc.wantCmd)
			}

			if tc.wantOK && len(gotArgs) == 0 {
				t.Fatalf("command(%q) args = %v, want non-empty", tc.goos, gotArgs)
			}
		})
	}
}

func TestSend(t *testing.T) {
	t.Parallel()

	called := false
	err := send("linux", "GitReal", "test", func(name string, args ...string) error {
		called = true
		if name != "notify-send" {
			t.Fatalf("runner name = %q, want notify-send", name)
		}
		if len(args) != 2 {
			t.Fatalf("runner args = %v, want 2 args", args)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("send() error = %v", err)
	}
	if !called {
		t.Fatalf("runner was not called")
	}
}

func TestSendUnsupportedPlatform(t *testing.T) {
	t.Parallel()

	err := send("plan9", "GitReal", "test", func(name string, args ...string) error {
		t.Fatalf("runner should not be called")
		return nil
	})
	if err == nil {
		t.Fatalf("send() error = nil, want non-nil")
	}
}

func TestSendRunnerError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("boom")
	err := send("linux", "GitReal", "test", func(name string, args ...string) error {
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("send() error = %v, want %v", err, wantErr)
	}
}

func TestSendUsesPackageVariables(t *testing.T) {
	t.Parallel()

	previousGOOS := currentGOOS
	previousRunner := runCommand
	t.Cleanup(func() {
		currentGOOS = previousGOOS
		runCommand = previousRunner
	})

	currentGOOS = "linux"
	called := false
	runCommand = func(name string, args ...string) error {
		called = true
		return nil
	}

	if err := Send("GitReal", "test"); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if !called {
		t.Fatalf("Send() did not invoke runCommand")
	}
}
