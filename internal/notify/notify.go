package notify

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

const maxFieldBytes = 1024

var currentGOOS = runtime.GOOS

var runCommand = func(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	return cmd.Run()
}

func Send(title, message string) error {
	return send(currentGOOS, title, message, runCommand)
}

func send(goos, title, message string, run func(name string, args ...string) error) error {
	name, args, ok := command(goos, sanitizeField(title), sanitizeField(message))
	if !ok {
		return fmt.Errorf("notifications are not supported on %s", goos)
	}

	return run(name, args...)
}

func command(goos, title, message string) (string, []string, bool) {
	switch goos {
	case "darwin":
		script := fmt.Sprintf("display notification %s with title %s", strconv.Quote(message), strconv.Quote(title))
		return "osascript", []string{"-e", script}, true
	case "linux":
		// `--` prevents notify-send from interpreting a leading "-" in title or message as a flag.
		return "notify-send", []string{"--", title, message}, true
	case "windows":
		template := fmt.Sprintf(
			`<toast><visual><binding template="ToastGeneric"><text>%s</text><text>%s</text></binding></visual></toast>`,
			xmlEscape(title),
			xmlEscape(message),
		)
		// -EncodedCommand takes a UTF-16LE base64 blob, so any quote/newline/$/backtick
		// inside `template` cannot escape the surrounding PowerShell context.
		script := fmt.Sprintf(`[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] > $null; [Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom.XmlDocument, ContentType = WindowsRuntime] > $null; $template = @'
%s
'@; $xml = New-Object Windows.Data.Xml.Dom.XmlDocument; $xml.LoadXml($template); $toast = [Windows.UI.Notifications.ToastNotification]::new($xml); $notifier = [Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier("git-real"); $notifier.Show($toast)`, template)
		return "powershell", []string{"-NoProfile", "-EncodedCommand", encodePowerShellCommand(script)}, true
	default:
		return "", nil, false
	}
}

// sanitizeField strips control characters and caps the length so a malicious
// branch name or backupRef cannot blow up the external command or smuggle
// terminal escape sequences into a notification.
func sanitizeField(value string) string {
	var b strings.Builder
	b.Grow(len(value))
	for _, r := range value {
		switch {
		case r == '\n', r == '\t', r == ' ':
			b.WriteRune(' ')
		case r < 0x20, r == 0x7f:
			b.WriteRune('?')
		default:
			b.WriteRune(r)
		}
	}

	out := b.String()
	if len(out) > maxFieldBytes {
		// Truncate on a rune boundary so we never produce invalid UTF-8.
		truncated := out[:maxFieldBytes]
		for len(truncated) > 0 {
			r, size := utf8.DecodeLastRuneInString(truncated)
			if r == utf8.RuneError && size <= 1 {
				truncated = truncated[:len(truncated)-1]
				continue
			}
			break
		}
		out = truncated
	}
	return out
}

func encodePowerShellCommand(script string) string {
	runes := utf16.Encode([]rune(script))
	buf := make([]byte, 2*len(runes))
	for i, r := range runes {
		binary.LittleEndian.PutUint16(buf[i*2:], r)
	}
	return base64.StdEncoding.EncodeToString(buf)
}

func xmlEscape(value string) string {
	var escaped bytes.Buffer
	if err := xml.EscapeText(&escaped, []byte(value)); err != nil {
		return value
	}

	return escaped.String()
}
