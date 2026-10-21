package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDurationJSON(t *testing.T) {
	var d Duration
	if err := json.Unmarshal([]byte(`"1m30s"`), &d); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if d.Duration() != 90*time.Second {
		t.Fatalf("got %v, want 90s", d.Duration())
	}

	raw, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(raw) != `"1m30s"` {
		t.Fatalf("got %s, want \"1m30s\"", raw)
	}
}

func TestDurationJSONNumber(t *testing.T) {
	var d Duration
	if err := json.Unmarshal([]byte(`5000`), &d); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if d.Duration() != 5000*time.Nanosecond {
		t.Fatalf("got %v, want 5us", d.Duration())
	}
}

func TestDurationJSONInvalid(t *testing.T) {
	var d Duration
	if err := json.Unmarshal([]byte(`"nope"`), &d); err == nil {
		t.Fatal("expected an error")
	}
}

func TestDurationFormat(t *testing.T) {
	tests := []struct {
		value time.Duration
		want  string
	}{
		{0, "0s"},
		{time.Second, "1s"},
		{5 * time.Second, "5s"},
		{time.Minute, "1m"},
		{time.Hour, "1h"},
		{90 * time.Second, "1m30s"},
		{2*time.Hour + 30*time.Minute, "2h30m"},
		{1500 * time.Millisecond, "1s500ms"},
		{250 * time.Millisecond, "250ms"},
	}
	for _, testCase := range tests {
		got := Duration(testCase.value).String()
		if got != testCase.want {
			t.Fatalf("got %q, want %q", got, testCase.want)
		}
		parsed, err := time.ParseDuration(got)
		if err != nil {
			t.Fatalf("%q is not a valid duration: %v", got, err)
		}
		if parsed != testCase.value {
			t.Fatalf("%q parsed as %v, want %v", got, parsed, testCase.value)
		}
	}
}

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.GraphWindow.Duration() != DefaultGraphWindow {
		t.Fatalf("got %v, want %v", cfg.GraphWindow, DefaultGraphWindow)
	}
	if cfg.ICMode != ICModeAuto {
		t.Fatalf("got %q, want %q", cfg.ICMode, ICModeAuto)
	}
	if len(cfg.Targets) != 0 {
		t.Fatalf("got %d targets, want 0", len(cfg.Targets))
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "config.json")
	cfg := Default()
	cfg.Targets = []Target{{
		ID:       "abc",
		Address:  "1.1.1.1",
		Interval: Duration(2 * time.Second),
		Timeout:  Duration(time.Second),
		Enabled:  true,
	}}
	cfg.GraphWindow = Duration(30 * time.Minute)

	if err := Save(path, cfg); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded.Targets) != 1 {
		t.Fatalf("got %d targets, want 1", len(loaded.Targets))
	}
	got := loaded.Targets[0]
	if got.Address != "1.1.1.1" || got.Interval.Duration() != 2*time.Second || got.Timeout.Duration() != time.Second {
		t.Fatalf("unexpected target %+v", got)
	}
	if loaded.GraphWindow.Duration() != 30*time.Minute {
		t.Fatalf("got %v, want 30m", loaded.GraphWindow)
	}
}

func TestLoadAppliesDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	raw := `{"targets":[{"address":" 8.8.8.8 "}],"graphWindow":0,"icmpMode":""}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	target := cfg.Targets[0]
	if target.Address != "8.8.8.8" {
		t.Fatalf("got %q, want 8.8.8.8", target.Address)
	}
	if target.Interval.Duration() != DefaultInterval {
		t.Fatalf("got %v, want %v", target.Interval, DefaultInterval)
	}
	if target.Timeout.Duration() != DefaultTimeout {
		t.Fatalf("got %v, want %v", target.Timeout, DefaultTimeout)
	}
	if target.ID == "" {
		t.Fatal("expected a generated id")
	}
	if cfg.GraphWindow.Duration() != DefaultGraphWindow {
		t.Fatalf("got %v, want %v", cfg.GraphWindow, DefaultGraphWindow)
	}
	if cfg.ICMode != ICModeAuto {
		t.Fatalf("got %q, want %q", cfg.ICMode, ICModeAuto)
	}
}

func TestDefaults(t *testing.T) {
	if DefaultTimeout != time.Second {
		t.Fatalf("got %v, want %v", DefaultTimeout, time.Second)
	}
	if DefaultInterval != 5*time.Second {
		t.Fatalf("got %v, want %v", DefaultInterval, 5*time.Second)
	}
	if DefaultTimeout >= DefaultInterval {
		t.Fatal("the default timeout must be smaller than the default interval")
	}
}

func TestValidateRejectsBadValues(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{"empty address", Config{Targets: []Target{{Address: "  "}}}},
		{"zero interval", Config{Targets: []Target{{Address: "a", Interval: 1}}}},
		{"zero timeout", Config{Targets: []Target{{Address: "a", Timeout: 1}}}},
		{"duplicate id", Config{Targets: []Target{
			{ID: "x", Address: "a", Interval: 1, Timeout: 1},
			{ID: "x", Address: "b", Interval: 1, Timeout: 1},
		}}},
		{"bad icmp mode", Config{ICMode: "weird"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := test.cfg
			if err := cfg.Validate(); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestResolvePath(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	if got := ResolvePath(""); got != DefaultPath() {
		t.Fatalf("got %q, want %q", got, DefaultPath())
	}

	if got := ResolvePath("/tmp/explicit.json"); got != "/tmp/explicit.json" {
		t.Fatalf("got %q, want /tmp/explicit.json", got)
	}

	if err := os.WriteFile(filepath.Join(dir, FileName), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	want, err := filepath.Abs(FileName)
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	if got := ResolvePath(""); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	if got := ResolvePath("/tmp/explicit.json"); got != "/tmp/explicit.json" {
		t.Fatalf("got %q, want /tmp/explicit.json", got)
	}
}

func TestResolvePathIgnoresDirectory(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	if err := os.Mkdir(filepath.Join(dir, FileName), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if got := ResolvePath(""); got != DefaultPath() {
		t.Fatalf("got %q, want %q", got, DefaultPath())
	}
}

// chdir moves the process into dir for the duration of the test.
func chdir(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(old); err != nil {
			t.Fatalf("chdir back: %v", err)
		}
	})
}

func TestNewIDIsUnique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := NewID()
		if id == "" {
			t.Fatal("empty id")
		}
		if seen[id] {
			t.Fatalf("duplicate id %s", id)
		}
		seen[id] = true
	}
}
