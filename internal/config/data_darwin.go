//go:build darwin

package config

import (
	"os"
	"path/filepath"
)

// defaultDataDir keeps the configuration and the data in Application Support,
// the conventional location on macOS.
func defaultDataDir() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "pingping-data"
	}
	return filepath.Join(dir, "pingping")
}
