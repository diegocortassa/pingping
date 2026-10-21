// Package logstore writes the collected samples to disk.
//
// One CSV file is used per calendar day, which gives the required rotation
// every 24 hours. Files older than the configured retention are removed.
package logstore

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pingping/pingping/internal/model"
)

// FileNamePattern is the pattern of the daily log files.
const FileNamePattern = "pings-2006-01-02.csv"

// Header is the header line of every log file.
var Header = []string{"timestamp", "target_id", "address", "rtt_ms", "success", "error"}

// Writer appends samples to the daily log file.
type Writer struct {
	mu        sync.Mutex
	dir       string
	retention time.Duration
	now       func() time.Time
	file      *os.File
	writer    *csv.Writer
	day       string
}

// New opens a writer for the log files in dir. A retention of zero keeps every
// file.
func New(dir string, retention time.Duration) (*Writer, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create log dir %s: %w", dir, err)
	}
	w := &Writer{
		dir:       dir,
		retention: retention,
		now:       time.Now,
	}
	if err := w.pruneLocked(); err != nil {
		return nil, err
	}
	return w, nil
}

// Write appends a sample to the log file of the current day.
func (w *Writer) Write(s model.Sample) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.rotate(s.Time); err != nil {
		return err
	}

	record := []string{
		s.Time.UTC().Format(time.RFC3339Nano),
		s.TargetID,
		s.Address,
		strconv.FormatFloat(float64(s.RTT)/float64(time.Millisecond), 'f', 3, 64),
		strconv.FormatBool(s.Success),
		s.Error,
	}
	if err := w.writer.Write(record); err != nil {
		return fmt.Errorf("write log record: %w", err)
	}
	w.writer.Flush()
	return w.writer.Error()
}

// Close flushes and closes the current log file.
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.closeFile()
}

// SetRetention changes the number of days of logs that are kept and removes
// the files that are now too old.
func (w *Writer) SetRetention(retention time.Duration) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.retention = retention
	_ = w.pruneLocked()
}

// Path returns the path of the file that is currently written.
func (w *Writer) Path() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return ""
	}
	return w.file.Name()
}

// rotate opens a new file when the day changed.
func (w *Writer) rotate(now time.Time) error {
	day := now.Format("2006-01-02")
	if w.file != nil && w.day == day {
		return nil
	}
	if err := w.closeFile(); err != nil {
		return err
	}
	if err := w.pruneLocked(); err != nil {
		return err
	}

	path := filepath.Join(w.dir, now.Format(FileNamePattern))
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open log file %s: %w", path, err)
	}
	w.file = file
	w.writer = csv.NewWriter(file)
	w.day = day

	if info, err := file.Stat(); err == nil && info.Size() == 0 {
		if err := w.writer.Write(Header); err != nil {
			return fmt.Errorf("write log header: %w", err)
		}
		w.writer.Flush()
	}
	return nil
}

// closeFile closes the current log file.
func (w *Writer) closeFile() error {
	if w.file == nil {
		return nil
	}
	if w.writer != nil {
		w.writer.Flush()
	}
	err := w.file.Close()
	w.file = nil
	w.writer = nil
	w.day = ""
	return err
}

// pruneLocked removes the log files that are older than the retention period.
// The caller holds the lock.
func (w *Writer) pruneLocked() error {
	if w.retention <= 0 {
		return nil
	}
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return fmt.Errorf("read log dir %s: %w", w.dir, err)
	}
	cutoff := w.now().Add(-w.retention)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "pings-") || !strings.HasSuffix(name, ".csv") {
			continue
		}
		day, err := time.Parse("2006-01-02", strings.TrimSuffix(strings.TrimPrefix(name, "pings-"), ".csv"))
		if err != nil {
			continue
		}
		if day.Add(24 * time.Hour).After(cutoff) {
			continue
		}
		if err := os.Remove(filepath.Join(w.dir, name)); err != nil {
			return fmt.Errorf("remove old log %s: %w", name, err)
		}
	}
	return nil
}
