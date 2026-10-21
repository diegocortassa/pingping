// Package icons exposes the application artwork.
//
// The PNG files are generated from the SVG sources in assets/ with the
// "make icons" target of the Makefile and are embedded into the binary so the
// application stays portable.
package icons

import (
	_ "embed"
	"fyne.io/fyne/v2"
)

//go:embed icon.png
var appPNG []byte

//go:embed tray.png
var trayPNG []byte

// App returns the application and window icon.
func App() fyne.Resource {
	return fyne.NewStaticResource("icon.png", appPNG)
}

// Tray returns the system tray icon.
func Tray() fyne.Resource {
	return fyne.NewStaticResource("tray.png", trayPNG)
}
