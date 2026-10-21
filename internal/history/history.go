// Package history keeps a rolling window of the collected samples and derives
// the statistics that are shown in the user interface.
package history

import (
	"sort"
	"sync"
	"time"

	"github.com/superping/superping/internal/model"
)

// maxSamplesPerTarget caps the memory used by a single target. The window is
// the primary limit, this is only a safety net for very short intervals.
const maxSamplesPerTarget = 20000

// Stats summarises the samples of a target inside the current window.
type Stats struct {
	// Count is the number of probes in the window.
	Count int
	// Lost is the number of probes that did not get an answer.
	Lost int
	// Loss is the percentage of lost probes.
	Loss float64
	// Last is the round trip time of the most recent probe.
	Last time.Duration
	// LastAt is the time of the most recent probe.
	LastAt time.Time
	// LastOK tells whether the most recent probe succeeded.
	LastOK bool
	// LastError is the error of the most recent probe, empty when it worked.
	LastError string
	// LastOffline tells whether the most recent probe could not be sent at
	// all, for example because the ICMP socket could not be opened.
	LastOffline bool
	// Min, Avg and Max are the round trip times of the successful probes.
	Min, Avg, Max time.Duration
	// Jitter is the mean absolute difference between consecutive round trip
	// times of the successful probes.
	Jitter time.Duration
}

// History stores the samples of every target inside a rolling window.
type History struct {
	mu     sync.RWMutex
	window time.Duration
	series map[string][]model.Sample
}

// New returns an empty history with the given window.
func New(window time.Duration) *History {
	if window <= 0 {
		window = time.Hour
	}
	return &History{
		window: window,
		series: make(map[string][]model.Sample),
	}
}

// Window returns the length of the rolling window.
func (h *History) Window() time.Duration {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.window
}

// SetWindow changes the rolling window and drops the samples that fell out of
// it.
func (h *History) SetWindow(window time.Duration) {
	if window <= 0 {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.window = window
	h.pruneLocked(time.Now())
}

// Add stores a sample and drops the ones that left the window.
func (h *History) Add(s model.Sample) {
	h.mu.Lock()
	defer h.mu.Unlock()

	samples := append(h.series[s.TargetID], s)
	sort.SliceStable(samples, func(i, j int) bool {
		return samples[i].Time.Before(samples[j].Time)
	})
	if len(samples) > maxSamplesPerTarget {
		samples = samples[len(samples)-maxSamplesPerTarget:]
	}
	h.series[s.TargetID] = samples
	h.pruneLocked(s.Time)
}

// Samples returns a copy of the samples of a target inside the window.
func (h *History) Samples(targetID string) []model.Sample {
	h.mu.RLock()
	defer h.mu.RUnlock()

	stored := h.series[targetID]
	out := make([]model.Sample, len(stored))
	copy(out, stored)
	return out
}

// Stats computes the statistics of a target inside the window.
func (h *History) Stats(targetID string) Stats {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return statsOf(h.series[targetID])
}

// Targets returns the identifiers that have samples.
func (h *History) Targets() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	ids := make([]string, 0, len(h.series))
	for id := range h.series {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Forget drops all the samples of a target.
func (h *History) Forget(targetID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.series, targetID)
}

// pruneLocked removes the samples that are older than the window.
func (h *History) pruneLocked(now time.Time) {
	cutoff := now.Add(-h.window)
	for id, samples := range h.series {
		idx := 0
		for idx < len(samples) && samples[idx].Time.Before(cutoff) {
			idx++
		}
		if idx == 0 {
			continue
		}
		if idx == len(samples) {
			delete(h.series, id)
			continue
		}
		h.series[id] = samples[idx:]
	}
}

// statsOf computes the statistics of a list of samples.
func statsOf(samples []model.Sample) Stats {
	var st Stats
	st.Count = len(samples)
	if st.Count == 0 {
		return st
	}

	last := samples[len(samples)-1]
	st.Last = last.RTT
	st.LastAt = last.Time
	st.LastOK = last.Success
	st.LastError = last.Error
	st.LastOffline = last.Offline

	var sum, jitter time.Duration
	var prev time.Duration
	ok := 0
	for _, s := range samples {
		if !s.Success {
			st.Lost++
			continue
		}
		ok++
		if st.Min == 0 || s.RTT < st.Min {
			st.Min = s.RTT
		}
		if s.RTT > st.Max {
			st.Max = s.RTT
		}
		sum += s.RTT
		if ok > 1 {
			diff := s.RTT - prev
			if diff < 0 {
				diff = -diff
			}
			jitter += diff
		}
		prev = s.RTT
	}

	st.Loss = float64(st.Lost) / float64(st.Count) * 100
	if ok > 0 {
		st.Avg = sum / time.Duration(ok)
	}
	if ok > 1 {
		st.Jitter = jitter / time.Duration(ok-1)
	}
	return st
}
