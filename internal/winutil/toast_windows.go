//go:build windows

package winutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Toast represents a Windows toast notification.
type Toast struct {
	AppID   string
	Title   string
	Message string
	Icon    string // Path to icon file
}

// Show displays a Windows toast notification using PowerShell.
// This approach works on Windows 10+ without requiring COM registration.
func (t *Toast) Show() error {
	appID := t.AppID
	if appID == "" {
		appID = "ProxyPilot"
	}

	// Escape single quotes in strings
	title := strings.ReplaceAll(t.Title, "'", "''")
	message := strings.ReplaceAll(t.Message, "'", "''")

	// Build the PowerShell script
	script := `
[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null
[Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom.XmlDocument, ContentType = WindowsRuntime] | Out-Null

$template = @"
<toast>
    <visual>
        <binding template="ToastGeneric">
            <text>` + title + `</text>
            <text>` + message + `</text>
        </binding>
    </visual>
</toast>
"@

$xml = New-Object Windows.Data.Xml.Dom.XmlDocument
$xml.LoadXml($template)
$toast = [Windows.UI.Notifications.ToastNotification]::new($xml)
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('` + appID + `').Show($toast)
`

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

// ShowToast is a convenience function for quick notifications.
func ShowToast(title, message string) error {
	t := &Toast{
		Title:   title,
		Message: message,
	}
	return t.Show()
}

// ShowToastWithIcon shows a toast with a custom icon.
func ShowToastWithIcon(title, message, iconPath string) error {
	t := &Toast{
		Title:   title,
		Message: message,
		Icon:    iconPath,
	}
	return t.Show()
}

// GetAppIconPath returns the path to the app icon if it exists.
func GetAppIconPath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	dir := filepath.Dir(exe)

	// Try common icon locations
	candidates := []string{
		filepath.Join(dir, "icon.ico"),
		filepath.Join(dir, "proxypilot.ico"),
		filepath.Join(dir, "assets", "icon.ico"),
	}

	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}
