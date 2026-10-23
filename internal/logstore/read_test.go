package logstore

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

const readHeader = "timestamp,target_id,address,rtt_ms,success,error\n"

func writeLog(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pings-test.csv")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}
	return path
}

func TestReadParsesTheSamples(t *testing.T) {
	path := writeLog(t, readHeader+
		"2026-10-21T09:00:02Z,t1,1.1.1.1,12.500,true,\n"+
		"2026-10-21T09:00:01Z,t1,1.1.1.1,0.000,false,timeout\n")

	samples, err := Read(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(samples) != 2 {
		t.Fatalf("got %d samples, want 2", len(samples))
	}
	if !samples[0].Time.Before(samples[1].Time) {
		t.Fatal("the samples are not sorted by time")
	}
	if samples[0].Success || samples[0].Error != "timeout" {
		t.Fatalf("unexpected failed sample %+v", samples[0])
	}
	if got := samples[1].RTT; got != 12500*time.Microsecond {
		t.Fatalf("got rtt %v, want 12.5ms", got)
	}
	if samples[1].TargetID != "t1" || samples[1].Address != "1.1.1.1" {
		t.Fatalf("unexpected sample %+v", samples[1])
	}
}

func TestReadAcceptsAReorderedHeader(t *testing.T) {
	path := writeLog(t, "success,address,timestamp,rtt_ms,target_id,error\n"+
		"true,8.8.8.8,2026-10-21T09:00:00Z,20.000,t2,\n")

	samples, err := Read(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(samples) != 1 || samples[0].Address != "8.8.8.8" || samples[0].RTT != 20*time.Millisecond {
		t.Fatalf("unexpected samples %+v", samples)
	}
}

func TestReadSkipsBadRows(t *testing.T) {
	path := writeLog(t, readHeader+
		"not-a-time,t1,1.1.1.1,12.0,true,\n"+
		"2026-10-21T09:00:00Z,t1,1.1.1.1,bad,true,\n"+
		"2026-10-21T09:00:01Z,t1,1.1.1.1,12.0,maybe,\n"+
		"2026-10-21T09:00:02Z,t1,1.1.1.1,12.0,true,\n")

	samples, err := Read(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(samples) != 1 {
		t.Fatalf("got %d samples, want 1", len(samples))
	}
}

func TestReadEmptyFile(t *testing.T) {
	path := writeLog(t, readHeader)
	samples, err := Read(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(samples) != 0 {
		t.Fatalf("got %d samples, want 0", len(samples))
	}
}

func TestReadRejectsAMissingColumn(t *testing.T) {
	path := writeLog(t, "timestamp,address,rtt_ms\n2026-10-21T09:00:00Z,1.1.1.1,12\n")
	if _, err := Read(path); err == nil {
		t.Fatal("expected an error for the missing success column")
	}
}

func TestReadRejectsAnUnreadableFile(t *testing.T) {
	if _, err := Read(filepath.Join(t.TempDir(), "missing.csv")); err == nil {
		t.Fatal("expected an error for the missing file")
	}
}
