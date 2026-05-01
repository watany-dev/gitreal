package notify

import (
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"testing"
	"unicode/utf16"
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
		if len(args) != 3 || args[0] != "--" {
			t.Fatalf("runner args = %v, want [-- title message]", args)
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

func TestCommandEscapesWindowsToastText(t *testing.T) {
	t.Parallel()

	_, args, ok := command("windows", `O'Hare <title>`, "line 1 & line 2")
	if !ok {
		t.Fatalf("command(windows) ok = false, want true")
	}
	if len(args) != 3 || args[0] != "-NoProfile" || args[1] != "-EncodedCommand" {
		t.Fatalf("command(windows) args = %v, want [-NoProfile -EncodedCommand <b64>]", args)
	}

	script := decodePowerShellCommand(t, args[2])
	for _, want := range []string{
		`O&#39;Hare &lt;title&gt;`,
		`line 1 &amp; line 2`,
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("script = %q, want substring %q", script, want)
		}
	}
}

// TestNotificationFieldsNeverEscapeShellOrPowerShell exercises a battery of
// hostile payloads (branch-name shaped strings, command-injection attempts,
// control characters, multi-byte runes) and asserts that on every supported
// OS the would-be exec arguments are argv-shaped (no shell), and that any
// PowerShell encoded blob round-trips back to a script that does not contain
// the raw payload escaping its quote/here-string context.
func TestNotificationFieldsNeverEscapeShellOrPowerShell(t *testing.T) {
	t.Parallel()

	payloads := []string{
		`'); Write-Host pwned; '`,
		`$(calc.exe)`,
		`'@` + "\n" + `Write-Host pwned` + "\n" + `@'`,
		"line1\nline2; rm -rf /",
		"\x00null-byte",
		"\x07\x08\x1b[31mred",
		"--leading-flag",
		"with\ttab",
		strings.Repeat("A", 4096),
		"japanese-日本語-rtl-\u202e",
		"refs/gitreal/backups/main@{-1}/x",
	}

	for i, payload := range payloads {
		payload := payload
		t.Run(fmt.Sprintf("payload-%d", i), func(t *testing.T) {
			t.Parallel()

			for _, goos := range []string{"darwin", "linux", "windows"} {
				name, args, ok := command(goos, sanitizeField(payload), sanitizeField(payload))
				if !ok {
					t.Fatalf("command(%s) returned ok=false", goos)
				}
				if name == "" {
					t.Fatalf("command(%s) returned empty name", goos)
				}

				switch goos {
				case "linux":
					if len(args) != 3 || args[0] != "--" {
						t.Fatalf("linux args = %v, want [-- title message]", args)
					}
					if strings.ContainsAny(args[1]+args[2], "\n\r\x00") {
						t.Fatalf("linux argv contains forbidden control chars: %q / %q", args[1], args[2])
					}
				case "darwin":
					if len(args) != 2 || args[0] != "-e" {
						t.Fatalf("darwin args = %v, want [-e script]", args)
					}
					// strconv.Quote-d strings always start and end with `"` and
					// contain no unescaped newlines.
					if strings.Count(args[1], "\n") > 0 {
						t.Fatalf("darwin script contains literal newline: %q", args[1])
					}
				case "windows":
					if len(args) != 3 || args[1] != "-EncodedCommand" {
						t.Fatalf("windows args = %v, want [-NoProfile -EncodedCommand b64]", args)
					}
					script := decodePowerShellCommand(t, args[2])
					// The here-string terminator `'@` must only appear once
					// (at the closing line). If user content smuggled `'@`
					// onto a line start, the script would close early and the
					// remaining bytes would run as code.
					if got := strings.Count(script, "\n'@"); got != 1 {
						t.Fatalf("windows script has %d here-string terminators, want 1: %q", got, script)
					}
					// Single-quoted here-strings do not expand $(...) or $var,
					// so a raw payload appearing inside @'...'@ is harmless,
					// but quote characters that could close the here-string
					// must be neutralized.
					if strings.Contains(script, "'); Write-Host") {
						t.Fatalf("windows script leaked unescaped quote: %q", script)
					}
				}
			}
		})
	}
}

func TestSanitizeFieldNormalizesControlChars(t *testing.T) {
	t.Parallel()

	got := sanitizeField("hello\x00world\x07\x1b[0m\nbar\ttab")
	want := "hello?world??[0m bar tab"
	if got != want {
		t.Fatalf("sanitizeField() = %q, want %q", got, want)
	}

	long := strings.Repeat("a", maxFieldBytes+50)
	if got := sanitizeField(long); len(got) != maxFieldBytes {
		t.Fatalf("sanitizeField(long) len = %d, want %d", len(got), maxFieldBytes)
	}

	// Truncating must not split a multi-byte rune.
	multibyte := strings.Repeat("あ", 1000) // 3 bytes per rune
	out := sanitizeField(multibyte)
	if len(out) > maxFieldBytes {
		t.Fatalf("sanitizeField(multibyte) len = %d, exceeds cap %d", len(out), maxFieldBytes)
	}
	for i, r := range out {
		if r == '�' {
			t.Fatalf("sanitizeField(multibyte) produced replacement rune at byte %d", i)
		}
	}
}

func decodePowerShellCommand(t *testing.T, encoded string) string {
	t.Helper()

	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("base64 decode error = %v", err)
	}
	if len(raw)%2 != 0 {
		t.Fatalf("decoded length %d is not utf16-aligned", len(raw))
	}

	codepoints := make([]uint16, len(raw)/2)
	for i := range codepoints {
		codepoints[i] = binary.LittleEndian.Uint16(raw[i*2:])
	}
	return string(utf16.Decode(codepoints))
}
