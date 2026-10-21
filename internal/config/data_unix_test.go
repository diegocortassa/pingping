//go:build !windows && !darwin

package config

import (
	"path/filepath"
	"testing"
)

func TestDefaultDataDirUsesXDGDataHome(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/tmp/xdg-data")
	if got, want := DefaultDataDir(), filepath.Join("/tmp/xdg-data", "pingping"); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDefaultDataDirFallsBackToTheHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "")
	want := filepath.Join(home, ".local", "share", "pingping")
	if got := DefaultDataDir(); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDefaultDataDirIgnoresRelativeXDGDataHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "relative/data")
	want := filepath.Join(home, ".local", "share", "pingping")
	if got := DefaultDataDir(); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
