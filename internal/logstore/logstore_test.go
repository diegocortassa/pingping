package logstore

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pingping/pingping/internal/model"
)

func readFile(t *testing.T, path string) [][]string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	records, err := csv.NewReader(strings.NewReader(string(raw))).ReadAll()
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return records
}

func TestWriteCreatesFileWithHeader(t *testing.T) {
	dir := t.TempDir()
	writer, err := New(dir, 0)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	defer writer.Close()

	at := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	if err := writer.Write(model.Sample{
		TargetID: "t1", Address: "1.1.1.1", Time: at, RTT: 12 * time.Millisecond, Success: true,
	}); err != nil {
		t.Fatalf("write: %v", err)
	}

	path := filepath.Join(dir, "pings-1.1.1.1-2026-10-21.csv")
	records := readFile(t, path)
	if len(records) != 2 {
		t.Fatalf("got %d records, want 2", len(records))
	}
	if records[0][0] != "timestamp" || records[0][3] != "rtt_ms" {
		t.Fatalf("unexpected header %v", records[0])
	}
	if records[1][1] != "t1" || records[1][2] != "1.1.1.1" {
		t.Fatalf("unexpected record %v", records[1])
	}
	if records[1][3] != "12.000" {
		t.Fatalf("unexpected rtt %q", records[1][3])
	}
	if records[1][4] != "true" {
		t.Fatalf("unexpected success %q", records[1][4])
	}
}

func TestWriteKeepsOneFilePerTarget(t *testing.T) {
	dir := t.TempDir()
	writer, err := New(dir, 0)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	defer writer.Close()

	at := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	if err := writer.Write(model.Sample{
		TargetID: "t1", Address: "1.1.1.1", Time: at, RTT: 12 * time.Millisecond, Success: true,
	}); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := writer.Write(model.Sample{
		TargetID: "t2", Address: "8.8.8.8", Time: at, RTT: 20 * time.Millisecond, Success: true,
	}); err != nil {
		t.Fatalf("write: %v", err)
	}

	first := readFile(t, filepath.Join(dir, "pings-1.1.1.1-2026-10-21.csv"))
	if len(first) != 2 || first[1][1] != "t1" {
		t.Fatalf("unexpected records of the first target %v", first)
	}

	second := readFile(t, filepath.Join(dir, "pings-8.8.8.8-2026-10-21.csv"))
	if len(second) != 2 || second[1][1] != "t2" {
		t.Fatalf("unexpected records of the second target %v", second)
	}
}

func TestWriteRotatesOnNewDay(t *testing.T) {
	dir := t.TempDir()
	writer, err := New(dir, 0)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	defer writer.Close()

	day1 := time.Date(2026, 10, 21, 23, 59, 0, 0, time.UTC)
	day2 := time.Date(2026, 10, 22, 0, 1, 0, 0, time.UTC)

	if err := writer.Write(model.Sample{TargetID: "t1", Address: "1.1.1.1", Time: day1, Success: true}); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := writer.Write(model.Sample{TargetID: "t1", Address: "1.1.1.1", Time: day2, Success: true}); err != nil {
		t.Fatalf("write: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "pings-1.1.1.1-2026-10-21.csv")); err != nil {
		t.Fatalf("expected the first day file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "pings-1.1.1.1-2026-10-22.csv")); err != nil {
		t.Fatalf("expected the second day file: %v", err)
	}
}

func TestRetentionRemovesOldFiles(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "pings-1.1.1.1-2020-01-01.csv")
	recent := filepath.Join(dir, "pings-1.1.1.1-2026-10-20.csv")
	for _, path := range []string{old, recent} {
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	writer, err := New(dir, 30*24*time.Hour)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	defer writer.Close()

	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatal("expected the old file to be removed")
	}
	if _, err := os.Stat(recent); err != nil {
		t.Fatalf("expected the recent file to stay: %v", err)
	}
}

func TestRetentionKeepsEverythingWhenZero(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "pings-1.1.1.1-2020-01-01.csv")
	if err := os.WriteFile(old, []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	writer, err := New(dir, 0)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	defer writer.Close()

	if _, err := os.Stat(old); err != nil {
		t.Fatalf("expected the old file to stay: %v", err)
	}
}

func TestFileSlug(t *testing.T) {
	tests := []struct {
		address string
		want    string
	}{
		{"1.1.1.1", "1.1.1.1"},
		{"my-host.local", "my-host.local"},
		{"2001:db8::1", "2001_db8__1"},
		{"host name", "host_name"},
		{"", "target"},
		{"...", "target"},
	}
	for _, testCase := range tests {
		if got := fileSlug(testCase.address); got != testCase.want {
			t.Fatalf("fileSlug(%q) = %q, want %q", testCase.address, got, testCase.want)
		}
	}
}

func TestFileSlugIsTruncated(t *testing.T) {
	long := strings.Repeat("a", 100)
	if got := fileSlug(long); len(got) != maxSlugLength {
		t.Fatalf("got %d characters, want %d", len(got), maxSlugLength)
	}
}

func TestDayOfFile(t *testing.T) {
	tests := []struct {
		name string
		ok   bool
	}{
		{"pings-1.1.1.1-2026-10-21.csv", true},
		{"pings-2001_db8__1-2026-10-21.csv", true},
		{"pings-2026-10-21.csv", true},
		{"other-2026-10-21.csv", false},
		{"pings-1.1.1.1-2026-10-21.txt", false},
		{"pings-1.1.1.1-notadate.csv", false},
	}
	for _, testCase := range tests {
		day, ok := dayOfFile(testCase.name)
		if ok != testCase.ok {
			t.Fatalf("dayOfFile(%q) ok = %v, want %v", testCase.name, ok, testCase.ok)
		}
		if !ok {
			continue
		}
		if day.Format(dayPattern) != "2026-10-21" {
			t.Fatalf("dayOfFile(%q) = %v", testCase.name, day)
		}
	}
}
