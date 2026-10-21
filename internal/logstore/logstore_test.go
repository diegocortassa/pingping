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

	path := filepath.Join(dir, "pings-2026-10-21.csv")
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

func TestWriteRotatesOnNewDay(t *testing.T) {
	dir := t.TempDir()
	writer, err := New(dir, 0)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	defer writer.Close()

	day1 := time.Date(2026, 10, 21, 23, 59, 0, 0, time.UTC)
	day2 := time.Date(2026, 10, 22, 0, 1, 0, 0, time.UTC)

	if err := writer.Write(model.Sample{TargetID: "t1", Time: day1, Success: true}); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := writer.Write(model.Sample{TargetID: "t1", Time: day2, Success: true}); err != nil {
		t.Fatalf("write: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "pings-2026-10-21.csv")); err != nil {
		t.Fatalf("expected the first day file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "pings-2026-10-22.csv")); err != nil {
		t.Fatalf("expected the second day file: %v", err)
	}
}

func TestRetentionRemovesOldFiles(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "pings-2020-01-01.csv")
	recent := filepath.Join(dir, "pings-2026-10-20.csv")
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
	old := filepath.Join(dir, "pings-2020-01-01.csv")
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
