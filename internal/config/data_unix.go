//go:build !windows && !darwin

package config

import (
	"os"
	"path/filepath"
)

// defaultDataDir follows the XDG base directory specification: the user data
// lives in $XDG_DATA_HOME, which defaults to $HOME/.local/share. A relative
// path is invalid and ignored.
func defaultDataDir() string {
	if dir := os.Getenv("XDG_DATA_HOME"); filepath.IsAbs(dir) {
		return filepath.Join(dir, "pingping")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "pingping-data"
	}
	return filepath.Join(home, ".local", "share", "pingping")
}
