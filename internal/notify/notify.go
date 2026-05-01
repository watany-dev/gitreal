package notify

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

var currentGOOS = runtime.GOOS

var runCommand = func(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	return cmd.Run()
}

func Send(title, message string) error {
	return send(currentGOOS, title, message, runCommand)
}

func send(goos, title, message string, run func(name string, args ...string) error) error {
	name, args, ok := command(goos, title, message)
	if !ok {
		return fmt.Errorf("notifications are not supported on %s", goos)
	}

	return run(name, args...)
}

func command(goos, title, message string) (string, []string, bool) {
	switch goos {
	case "darwin":
		script := fmt.Sprintf(`display notification %s with title %s sound name "Sosumi"`, strconv.Quote(message), strconv.Quote(title))
		return "osascript", []string{"-e", script}, true
	case "linux":
		return "notify-send", []string{
			"-u", "critical",
			"-t", "0",
			"-a", "git-real",
			"-i", "dialog-warning",
			title,
			message,
		}, true
	case "windows":
		template := fmt.Sprintf(
			`<toast><visual><binding template="ToastGeneric"><text>%s</text><text>%s</text></binding></visual></toast>`,
			xmlEscape(title),
			xmlEscape(message),
		)
		script := fmt.Sprintf(`[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] > $null; [Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom.XmlDocument, ContentType = WindowsRuntime] > $null; $template = '%s'; $xml = New-Object Windows.Data.Xml.Dom.XmlDocument; $xml.LoadXml($template); $toast = [Windows.UI.Notifications.ToastNotification]::new($xml); $notifier = [Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier("git-real"); $notifier.Show($toast); [console]::beep(880,300)`, powerShellSingleQuote(template))
		return "powershell", []string{"-NoProfile", "-Command", script}, true
	default:
		return "", nil, false
	}
}

func powerShellSingleQuote(value string) string {
	return strings.ReplaceAll(value, `'`, `''`)
}

func xmlEscape(value string) string {
	var escaped bytes.Buffer
	if err := xml.EscapeText(&escaped, []byte(value)); err != nil {
		return value
	}

	return escaped.String()
}
