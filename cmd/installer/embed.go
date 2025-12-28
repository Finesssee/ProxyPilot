//go:build windows

package main

import (
	"embed"
)

// embeddedUI contains the built installer UI from installer-ui/dist.
// The ui directory should contain index.html and any other assets.
//
//go:embed all:ui
var embeddedUI embed.FS

// embeddedBundle contains the files to install.
// These are embedded during the build process.
//
//go:embed bundle/ProxyPilot.exe bundle/config.example.yaml bundle/icon.ico bundle/icon.png
var embeddedBundle embed.FS
