package history

import (
	"testing"
	"time"

	"github.com/pingping/pingping/internal/model"
)

func sample(at time.Time, rtt time.Duration, ok bool) model.Sample {
	return model.Sample{TargetID: "t1", Address: "1.1.1.1", Time: at, RTT: rtt, Success: ok}
}

func TestAddAndSamples(t *testing.T) {
	now := time.Now()
	h := New(time.Hour)

	h.Add(sample(now, 10*time.Millisecond, true))
	h.Add(sample(now.Add(time.Second), 20*time.Millisecond, true))

	samples := h.Samples("t1")
	if len(samples) != 2 {
		t.Fatalf("got %d samples, want 2", len(samples))
	}
	if samples[0].RTT != 10*time.Millisecond {
		t.Fatalf("got %v, want 10ms", samples[0].RTT)
	}
}

func TestAddPrunesOldSamples(t *testing.T) {
	now := time.Now()
	h := New(time.Minute)

	h.Add(sample(now.Add(-2*time.Minute), 10*time.Millisecond, true))
	h.Add(sample(now, 20*time.Millisecond, true))

	if got := len(h.Samples("t1")); got != 1 {
		t.Fatalf("got %d samples, want 1", got)
	}
}

func TestSetWindowPrunes(t *testing.T) {
	now := time.Now()
	h := New(time.Hour)
	h.Add(sample(now.Add(-30*time.Minute), 10*time.Millisecond, true))
	h.Add(sample(now, 20*time.Millisecond, true))

	h.SetWindow(time.Minute)
	if got := len(h.Samples("t1")); got != 1 {
		t.Fatalf("got %d samples, want 1", got)
	}
	if got := len(h.SamplesInsideWindow("t1", now)); got != 1 {
		t.Fatalf("got %d samples inside window, want 1", got)
	}
	h.mu.RLock()
	stored := len(h.series["t1"])
	h.mu.RUnlock()
	if stored != 2 {
		t.Fatalf("got %d samples in memory, want 2", stored)
	}
}

func TestStats(t *testing.T) {
	now := time.Now()
	h := New(time.Hour)

	h.Add(sample(now.Add(-3*time.Second), 10*time.Millisecond, true))
	h.Add(sample(now.Add(-2*time.Second), 30*time.Millisecond, true))
	h.Add(sample(now.Add(-time.Second), 0, false))

	stats := h.Stats("t1")
	if stats.Count != 3 {
		t.Fatalf("got count %d, want 3", stats.Count)
	}
	if stats.Lost != 1 {
		t.Fatalf("got lost %d, want 1", stats.Lost)
	}
	if stats.Loss < 33 || stats.Loss > 34 {
		t.Fatalf("got loss %v, want about 33.3", stats.Loss)
	}
	if stats.Min != 10*time.Millisecond {
		t.Fatalf("got min %v, want 10ms", stats.Min)
	}
	if stats.Max != 30*time.Millisecond {
		t.Fatalf("got max %v, want 30ms", stats.Max)
	}
	if stats.Avg != 20*time.Millisecond {
		t.Fatalf("got avg %v, want 20ms", stats.Avg)
	}
	if stats.LastOK {
		t.Fatal("expected the last probe to be failed")
	}
	if stats.Jitter != 20*time.Millisecond {
		t.Fatalf("got jitter %v, want 20ms", stats.Jitter)
	}
}

func TestStatsEmpty(t *testing.T) {
	h := New(time.Hour)
	stats := h.Stats("missing")
	if stats.Count != 0 || stats.Loss != 0 {
		t.Fatalf("unexpected stats %+v", stats)
	}
}

func TestStatsWithASingleSample(t *testing.T) {
	h := New(time.Hour)
	h.Add(sample(time.Now(), 10*time.Millisecond, true))

	stats := h.Stats("t1")
	if stats.Count != 1 || stats.Lost != 0 {
		t.Fatalf("unexpected stats %+v", stats)
	}
	if stats.Avg != 10*time.Millisecond {
		t.Fatalf("got avg %v, want 10ms", stats.Avg)
	}
	if stats.Jitter != 0 {
		t.Fatalf("got jitter %v, want 0", stats.Jitter)
	}
}

func TestStatsWithASingleFailure(t *testing.T) {
	h := New(time.Hour)
	h.Add(sample(time.Now(), 0, false))

	stats := h.Stats("t1")
	if stats.Count != 1 || stats.Lost != 1 {
		t.Fatalf("unexpected stats %+v", stats)
	}
	if stats.Loss != 100 {
		t.Fatalf("got loss %v, want 100", stats.Loss)
	}
}

func TestStatsKeepsTheLastError(t *testing.T) {
	h := New(time.Hour)
	h.Add(sample(time.Now(), 0, false))
	h.Add(model.Sample{TargetID: "t1", Time: time.Now(), Offline: true, Error: "socket: permission denied"})

	stats := h.Stats("t1")
	if stats.LastError != "socket: permission denied" {
		t.Fatalf("got error %q", stats.LastError)
	}
	if !stats.LastOffline {
		t.Fatal("expected the last probe to be offline")
	}
}

func TestForget(t *testing.T) {
	h := New(time.Hour)
	h.Add(sample(time.Now(), time.Millisecond, true))
	h.Forget("t1")
	if got := len(h.Samples("t1")); got != 0 {
		t.Fatalf("got %d samples, want 0", got)
	}
}
