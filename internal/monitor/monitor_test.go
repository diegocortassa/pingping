package monitor

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/pingping/pingping/internal/config"
	"github.com/pingping/pingping/internal/model"
	"github.com/pingping/pingping/internal/ping"
)

// fakePinger reports a fixed number of samples and then stops.
type fakePinger struct {
	results []ping.Result
}

func (f *fakePinger) Run(ctx context.Context) <-chan ping.Result {
	out := make(chan ping.Result, len(f.results))
	for _, result := range f.results {
		out <- result
	}
	close(out)
	return out
}

func TestMonitorPublishesSamples(t *testing.T) {
	fake := &fakePinger{results: []ping.Result{
		{Time: time.Now(), RTT: 10 * time.Millisecond, Success: true},
		{Time: time.Now(), Err: "timeout"},
	}}

	mon := New(WithPingerFactory(func(config.Target, config.ICMode) Pinger { return fake }))
	samples, cancel := mon.Subscribe(8)
	defer cancel()

	mon.Upsert(config.Target{ID: "t1", Address: "1.1.1.1", Interval: config.Duration(time.Second), Timeout: config.Duration(time.Second), Enabled: true})

	ctx, cancelCtx := context.WithCancel(context.Background())
	mon.Start(ctx)
	defer mon.Stop()

	received := make([]model.Sample, 0, 2)
	for i := 0; i < 2; i++ {
		select {
		case s := <-samples:
			received = append(received, s)
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for a sample")
		}
	}
	cancelCtx()

	if received[0].TargetID != "t1" || received[0].Address != "1.1.1.1" {
		t.Fatalf("unexpected sample %+v", received[0])
	}
	if !received[0].Success || received[0].RTT != 10*time.Millisecond {
		t.Fatalf("unexpected sample %+v", received[0])
	}
	if received[1].Success || received[1].Error != "timeout" {
		t.Fatalf("unexpected sample %+v", received[1])
	}
}

func TestMonitorSinkReceivesSamples(t *testing.T) {
	fake := &fakePinger{results: []ping.Result{{Time: time.Now(), RTT: time.Millisecond, Success: true}}}

	var mu sync.Mutex
	var count int
	mon := New(WithPingerFactory(func(config.Target, config.ICMode) Pinger { return fake }))
	mon.SetSink(func(model.Sample) {
		mu.Lock()
		count++
		mu.Unlock()
	})

	mon.Upsert(config.Target{ID: "t1", Enabled: true, Interval: config.Duration(time.Second), Timeout: config.Duration(time.Second)})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mon.Start(ctx)
	defer mon.Stop()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		got := count
		mu.Unlock()
		if got > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("the sink was not called")
}

func TestMonitorUpsertAndRemove(t *testing.T) {
	mon := New(WithPingerFactory(func(config.Target, config.ICMode) Pinger { return &fakePinger{} }))

	target := config.Target{ID: "t1", Address: "1.1.1.1", Enabled: true, Interval: config.Duration(time.Second), Timeout: config.Duration(time.Second)}
	mon.Upsert(target)

	if got := len(mon.Targets()); got != 1 {
		t.Fatalf("got %d targets, want 1", got)
	}

	mon.Remove("t1")
	if got := len(mon.Targets()); got != 0 {
		t.Fatalf("got %d targets, want 0", got)
	}
}

func TestMonitorDisabledTargetIsNotProbed(t *testing.T) {
	var mu sync.Mutex
	probed := false
	mon := New(WithPingerFactory(func(config.Target, config.ICMode) Pinger {
		mu.Lock()
		probed = true
		mu.Unlock()
		return &fakePinger{}
	}))

	mon.Upsert(config.Target{ID: "t1", Enabled: false, Interval: config.Duration(time.Second), Timeout: config.Duration(time.Second)})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mon.Start(ctx)
	defer mon.Stop()

	time.Sleep(100 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if probed {
		t.Fatal("a disabled target must not be probed")
	}
}
