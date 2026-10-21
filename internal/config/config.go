// Package config holds the persistent configuration of SuperPinger.
//
// The configuration is stored as JSON. Field names use camelCase, durations
// are stored as human readable strings such as "5s" or "1h".
package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Default values used when a field is missing from the configuration file.
const (
	DefaultInterval    = 5 * time.Second
	DefaultTimeout     = 3 * time.Second
	DefaultGraphWindow = time.Hour
	DefaultLogDays     = 30
)

// ICMode selects how ICMP sockets are opened.
type ICMode string

const (
	// ICModeAuto tries an unprivileged socket first and falls back to a
	// privileged one when the operating system refuses it.
	ICModeAuto ICMode = "auto"
	// ICModeUnprivileged only uses unprivileged sockets.
	ICModeUnprivileged ICMode = "unprivileged"
	// ICModePrivileged always uses a privileged raw socket.
	ICModePrivileged ICMode = "privileged"
)

// Duration is a time.Duration that is serialised as a string such as "1m30s".
type Duration time.Duration

// Duration returns the wrapped standard library duration.
func (d Duration) Duration() time.Duration {
	return time.Duration(d)
}

func (d Duration) String() string {
	return formatDuration(time.Duration(d))
}

// MarshalJSON implements json.Marshaler.
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

// UnmarshalJSON implements json.Unmarshaler. Both "5s" and a plain number of
// nanoseconds are accepted.
func (d *Duration) UnmarshalJSON(data []byte) error {
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	switch v := raw.(type) {
	case string:
		if v == "" {
			*d = 0
			return nil
		}
		parsed, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("invalid duration %q: %w", v, err)
		}
		*d = Duration(parsed)
		return nil
	case float64:
		*d = Duration(time.Duration(v))
		return nil
	default:
		return fmt.Errorf("invalid duration value %v", raw)
	}
}

// Target is a single monitored address.
type Target struct {
	// ID is a stable identifier, generated when the target is created.
	ID string `json:"id"`
	// Address is the host name or IP address to ping.
	Address string `json:"address"`
	// Interval is the delay between two consecutive probes.
	Interval Duration `json:"interval"`
	// Timeout is the time after which a probe is reported as lost.
	Timeout Duration `json:"timeout"`
	// Enabled tells whether the target is currently monitored.
	Enabled bool `json:"enabled"`
}

// Config is the whole persisted configuration of the application.
type Config struct {
	// Targets is the list of monitored addresses.
	Targets []Target `json:"targets"`
	// GraphWindow is the time span displayed by the charts.
	GraphWindow Duration `json:"graphWindow"`
	// LogRetentionDays is the number of days of logs that are kept.
	// Zero keeps every log file.
	LogRetentionDays int `json:"logRetentionDays"`
	// ICMode selects how the ICMP sockets are opened.
	ICMode ICMode `json:"icmpMode"`
	// DataDir is the directory that holds the log files.
	DataDir string `json:"dataDir"`
}

// Default returns a configuration with all defaults applied.
func Default() Config {
	return Config{
		Targets:          []Target{},
		GraphWindow:      Duration(DefaultGraphWindow),
		LogRetentionDays: DefaultLogDays,
		ICMode:           ICModeAuto,
		DataDir:          DefaultDataDir(),
	}
}

// DefaultPath returns the default location of the configuration file.
func DefaultPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "superping.json"
	}
	return filepath.Join(dir, "superping", "config.json")
}

// DefaultDataDir returns the default directory for the persistent data.
func DefaultDataDir() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "superping-data"
	}
	return filepath.Join(dir, "superping", "data")
}

// Load reads the configuration from path. A missing file yields the default
// configuration.
func Load(path string) (Config, error) {
	cfg := Default()
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("read config %s: %w", path, err)
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config %s: %w", path, err)
	}
	cfg.applyDefaults()
	return cfg, cfg.Validate()
}

// Save writes the configuration to path. The file is written atomically so a
// crash never leaves a truncated configuration behind.
func Save(path string, cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create config dir %s: %w", dir, err)
		}
	}
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	raw = append(raw, '\n')

	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp config: %w", err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp config: %w", err)
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return fmt.Errorf("chmod temp config: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("replace config: %w", err)
	}
	return nil
}

// applyDefaults fills in the fields that are missing or invalid.
func (c *Config) applyDefaults() {
	if c.GraphWindow <= 0 {
		c.GraphWindow = Duration(DefaultGraphWindow)
	}
	if c.LogRetentionDays < 0 {
		c.LogRetentionDays = 0
	}
	if c.ICMode == "" {
		c.ICMode = ICModeAuto
	}
	if c.DataDir == "" {
		c.DataDir = DefaultDataDir()
	}
	if c.Targets == nil {
		c.Targets = []Target{}
	}
	for i := range c.Targets {
		t := &c.Targets[i]
		if t.ID == "" {
			t.ID = NewID()
		}
		if t.Interval <= 0 {
			t.Interval = Duration(DefaultInterval)
		}
		if t.Timeout <= 0 {
			t.Timeout = Duration(DefaultTimeout)
		}
		t.Address = strings.TrimSpace(t.Address)
	}
}

// Validate checks the configuration and normalises the target list.
func (c *Config) Validate() error {
	switch c.ICMode {
	case ICModeAuto, ICModeUnprivileged, ICModePrivileged:
	default:
		return fmt.Errorf("unknown icmp mode %q", c.ICMode)
	}
	if c.GraphWindow <= 0 {
		return errors.New("graph window must be positive")
	}
	if c.LogRetentionDays < 0 {
		return errors.New("log retention must not be negative")
	}
	seen := make(map[string]bool, len(c.Targets))
	for i := range c.Targets {
		t := &c.Targets[i]
		t.Address = strings.TrimSpace(t.Address)
		if t.Address == "" {
			return fmt.Errorf("target %d: address must not be empty", i)
		}
		if t.Interval <= 0 {
			return fmt.Errorf("target %q: interval must be positive", t.Address)
		}
		if t.Timeout <= 0 {
			return fmt.Errorf("target %q: timeout must be positive", t.Address)
		}
		if t.ID == "" {
			t.ID = NewID()
		}
		if seen[t.ID] {
			return fmt.Errorf("target %q: duplicate id %q", t.Address, t.ID)
		}
		seen[t.ID] = true
	}
	return nil
}

// LogDir returns the directory that holds the log files.
func (c Config) LogDir() string {
	return filepath.Join(c.DataDir, "logs")
}

// NewID returns a random identifier for a target.
func NewID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("t%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

// formatDuration renders a duration in a compact form such as "1h" or "1m30s".
// The output is always accepted by time.ParseDuration.
func formatDuration(d time.Duration) string {
	if d == 0 {
		return "0s"
	}

	sign := ""
	if d < 0 {
		sign = "-"
		d = -d
	}

	hours := d / time.Hour
	d -= hours * time.Hour
	minutes := d / time.Minute
	d -= minutes * time.Minute
	seconds := d / time.Second
	d -= seconds * time.Second
	millis := d / time.Millisecond

	parts := make([]string, 0, 4)
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if minutes > 0 {
		parts = append(parts, fmt.Sprintf("%dm", minutes))
	}
	if seconds > 0 {
		parts = append(parts, fmt.Sprintf("%ds", seconds))
	}
	if millis > 0 {
		parts = append(parts, fmt.Sprintf("%dms", millis))
	}
	if len(parts) == 0 {
		return sign + "0s"
	}
	return sign + strings.Join(parts, "")
}
