package trayicon

import (
	_ "embed"
)

//go:embed icon.ico
var proxyPilotICO []byte

// ProxyPilotICO returns the embedded ProxyPilot icon for Windows tray.
func ProxyPilotICO() []byte {
	return proxyPilotICO
}
