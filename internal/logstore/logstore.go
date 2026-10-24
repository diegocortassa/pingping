// Package logstore writes the collected samples to disk.
//
// Every target has its own CSV file, the name holds the address and the day,
// so the files are rotated every 24 hours. Files older than the configured
// retention are removed.
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

	"github.com/diegocortassa/pingping/internal/model"
)

const (
	// filePrefix is the prefix of the log files.
	filePrefix = "pings-"
	// fileSuffix is the extension of the log files.
	fileSuffix = ".csv"
	// dayPattern is the layout of the day inside the name of a log file.
	dayPattern = "2006-01-02"
	// dayLength is the length of the day inside the name of a log file.
	dayLength = len(dayPattern)
	// maxSlugLength is the maximum length of the address part of the name.
	maxSlugLength = 64
)

// Header is the header line of every log file.
var Header = []string{"timestamp", "target_id", "address", "rtt_ms", "success", "error"}

// Writer appends the samples of every target to its own log file.
type Writer struct {
	mu        sync.Mutex
	dir       string
	retention time.Duration
	now       func() time.Time
	files     map[string]*logFile
}

// logFile is an open log file of one target.
type logFile struct {
	file   *os.File
	writer *csv.Writer
	day    string
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
		files:     make(map[string]*logFile),
	}
	if err := w.pruneLocked(); err != nil {
		return nil, err
	}
	return w, nil
}

// Write appends a sample to the log file of its target.
func (w *Writer) Write(s model.Sample) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	slug := fileSlug(s.Address)
	if err := w.rotate(slug, s.Time); err != nil {
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
	if err := w.files[slug].writer.Write(record); err != nil {
		return fmt.Errorf("write log record: %w", err)
	}
	w.files[slug].writer.Flush()
	return w.files[slug].writer.Error()
}

// Close flushes and closes the open log files.
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	var firstErr error
	for slug, file := range w.files {
		if err := file.close(); err != nil && firstErr == nil {
			firstErr = err
		}
		delete(w.files, slug)
	}
	return firstErr
}

// SetRetention changes the number of days of logs that are kept and removes
// the files that are now too old.
func (w *Writer) SetRetention(retention time.Duration) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.retention = retention
	_ = w.pruneLocked()
}

// Dir returns the directory that holds the log files.
func (w *Writer) Dir() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.dir
}

// rotate opens a new file when the day of a target changed.
func (w *Writer) rotate(slug string, now time.Time) error {
	day := now.Format(dayPattern)
	if file, ok := w.files[slug]; ok {
		if file.day == day {
			return nil
		}
		if err := file.close(); err != nil {
			return err
		}
		delete(w.files, slug)
	}
	if err := w.pruneLocked(); err != nil {
		return err
	}

	path := filepath.Join(w.dir, fileName(slug, day))
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open log file %s: %w", path, err)
	}
	entry := &logFile{file: file, writer: csv.NewWriter(file), day: day}

	if info, err := file.Stat(); err == nil && info.Size() == 0 {
		if err := entry.writer.Write(Header); err != nil {
			return fmt.Errorf("write log header: %w", err)
		}
		entry.writer.Flush()
	}
	w.files[slug] = entry
	return nil
}

// close flushes and closes the file.
func (f *logFile) close() error {
	if f.writer != nil {
		f.writer.Flush()
	}
	err := f.file.Close()
	f.file = nil
	f.writer = nil
	f.day = ""
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
		day, ok := dayOfFile(entry.Name())
		if !ok {
			continue
		}
		if day.Add(24 * time.Hour).After(cutoff) {
			continue
		}
		if err := os.Remove(filepath.Join(w.dir, entry.Name())); err != nil {
			return fmt.Errorf("remove old log %s: %w", entry.Name(), err)
		}
	}
	return nil
}

// fileName returns the name of the log file of a target for a day.
func fileName(slug, day string) string {
	return filePrefix + slug + "-" + day + fileSuffix
}

// dayOfFile returns the day of a log file, the second result is false when the
// name does not belong to a log file.
func dayOfFile(name string) (time.Time, bool) {
	if !strings.HasPrefix(name, filePrefix) || !strings.HasSuffix(name, fileSuffix) {
		return time.Time{}, false
	}
	stamp := strings.TrimSuffix(strings.TrimPrefix(name, filePrefix), fileSuffix)
	if len(stamp) < dayLength {
		return time.Time{}, false
	}
	day, err := time.Parse(dayPattern, stamp[len(stamp)-dayLength:])
	if err != nil {
		return time.Time{}, false
	}
	return day, true
}

// fileSlug converts an address into the part of the name of a log file that
// identifies the target. The characters that are not allowed in a file name
// on every supported platform are replaced.
func fileSlug(address string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(address) {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '.' || r == '-' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}

	slug := strings.Trim(b.String(), ".")
	if slug == "" {
		return "target"
	}
	if len(slug) > maxSlugLength {
		slug = slug[:maxSlugLength]
	}
	return slug
}
