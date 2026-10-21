//go:build !linux

package ping

// groupRange returns an empty string, the group range is a Linux concept.
func groupRange() string {
	return ""
}

// capabilities returns an empty string, the capabilities are read from the
// process status on Linux only.
func capabilities() string {
	return ""
}
