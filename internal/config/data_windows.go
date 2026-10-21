//go:build windows

package config

import (
	"os"
	"path/filepath"
)

// defaultDataDir keeps the data in the local application data, the logs are
// machine specific and must not roam with the user profile.
func defaultDataDir() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		dir, err = os.UserConfigDir()
		if err != nil {
			return "pingping-data"
		}
	}
	return filepath.Join(dir, "pingping")
}
