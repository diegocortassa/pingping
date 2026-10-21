// Package monitor drives the probes of the configured targets and publishes
// the results to its subscribers.
package monitor

import (
	"context"
	"sync"
	"time"

	"github.com/pingping/pingping/internal/config"
	"github.com/pingping/pingping/internal/model"
	"github.com/pingping/pingping/internal/ping"
)

// restartDelay is the pause between two attempts when the probe loop died,
// for example because the host name could not be resolved.
const restartDelay = 15 * time.Second

// Pinger probes a single address and reports the results.
type Pinger interface {
	Run(ctx context.Context) <-chan ping.Result
}

// PingerFactory creates the pinger of a target.
type PingerFactory func(target config.Target, mode config.ICMode) Pinger

// Option configures a monitor.
type Option func(*Monitor)

// WithPingerFactory replaces the pinger implementation. It is mostly useful
// for the tests.
func WithPingerFactory(factory PingerFactory) Option {
	return func(m *Monitor) {
		m.factory = factory
	}
}

// Monitor owns one probe loop per target.
type Monitor struct {
	mu      sync.Mutex
	targets map[string]config.Target
	runners map[string]*runner
	subs    map[int]chan model.Sample
	nextSub int
	sink    func(model.Sample)
	mode    config.ICMode
	factory PingerFactory
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// runner is the probe loop of a single target.
type runner struct {
	cancel context.CancelFunc
	done   chan struct{}
}

// New returns a monitor without any target.
func New(opts ...Option) *Monitor {
	m := &Monitor{
		targets: make(map[string]config.Target),
		runners: make(map[string]*runner),
		subs:    make(map[int]chan model.Sample),
		factory: defaultFactory,
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// defaultFactory creates the real ICMP pinger.
func defaultFactory(target config.Target, mode config.ICMode) Pinger {
	return ping.New(target.Address, target.Interval.Duration(), target.Timeout.Duration(), mode)
}

// Start begins the monitoring. The monitor stops when the context is
// cancelled.
func (m *Monitor) Start(ctx context.Context) {
	m.mu.Lock()
	m.ctx, m.cancel = context.WithCancel(ctx)
	targets := m.targetsLocked()
	m.mu.Unlock()

	for _, t := range targets {
		m.launch(t)
	}
}

// Stop ends all probe loops.
func (m *Monitor) Stop() {
	m.mu.Lock()
	cancel := m.cancel
	m.cancel = nil
	m.ctx = nil
	m.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	m.wg.Wait()
}

// SetICMode changes the socket flavour used for the probe loops.
func (m *Monitor) SetICMode(mode config.ICMode) {
	m.mu.Lock()
	m.mode = mode
	m.mu.Unlock()
}

// SetSink registers a function that receives every sample. It is called from
// a dedicated goroutine, so it may block without slowing the probes down.
func (m *Monitor) SetSink(sink func(model.Sample)) {
	m.mu.Lock()
	m.sink = sink
	m.mu.Unlock()
}

// Subscribe returns a channel that receives every sample and a function that
// cancels the subscription.
func (m *Monitor) Subscribe(buffer int) (<-chan model.Sample, func()) {
	if buffer <= 0 {
		buffer = 64
	}
	ch := make(chan model.Sample, buffer)

	m.mu.Lock()
	id := m.nextSub
	m.nextSub++
	m.subs[id] = ch
	m.mu.Unlock()

	return ch, func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		if sub, ok := m.subs[id]; ok {
			delete(m.subs, id)
			close(sub)
		}
	}
}

// Upsert adds a target or replaces its configuration. The probe loop is
// restarted so that a new interval or timeout takes effect immediately.
func (m *Monitor) Upsert(t config.Target) {
	m.mu.Lock()
	m.targets[t.ID] = t
	m.mu.Unlock()

	m.stop(t.ID)
	if t.Enabled {
		m.launch(t)
	}
}

// Remove stops the probe loop of a target and forgets it.
func (m *Monitor) Remove(targetID string) {
	m.mu.Lock()
	delete(m.targets, targetID)
	m.mu.Unlock()

	m.stop(targetID)
}

// Targets returns the configuration of every monitored target.
func (m *Monitor) Targets() []config.Target {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.targetsLocked()
}

// targetsLocked returns the target configurations, the caller holds the lock.
func (m *Monitor) targetsLocked() []config.Target {
	out := make([]config.Target, 0, len(m.targets))
	for _, t := range m.targets {
		out = append(out, t)
	}
	return out
}

// launch starts the probe loop of a target.
func (m *Monitor) launch(t config.Target) {
	if !t.Enabled {
		return
	}

	m.mu.Lock()
	ctx := m.ctx
	mode := m.mode
	if ctx == nil {
		m.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(ctx)
	r := &runner{cancel: cancel, done: make(chan struct{})}
	m.runners[t.ID] = r
	m.wg.Add(1)
	m.mu.Unlock()

	go func() {
		defer m.wg.Done()
		defer close(r.done)
		m.loop(ctx, t, mode)
	}()
}

// stop cancels the probe loop of a target.
func (m *Monitor) stop(targetID string) {
	m.mu.Lock()
	r, ok := m.runners[targetID]
	delete(m.runners, targetID)
	m.mu.Unlock()

	if ok {
		r.cancel()
		<-r.done
	}
}

// loop probes a target until the context is cancelled.
func (m *Monitor) loop(ctx context.Context, t config.Target, mode config.ICMode) {
	for ctx.Err() == nil {
		for result := range m.factory(t, mode).Run(ctx) {
			m.publish(model.Sample{
				TargetID: t.ID,
				Address:  t.Address,
				Time:     result.Time,
				RTT:      result.RTT,
				Success:  result.Success,
				Offline:  result.Offline,
				Error:    result.Err,
			})
		}
		if ctx.Err() != nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(restartDelay):
		}
	}
}

// publish hands a sample to the sink and to every subscriber.
func (m *Monitor) publish(s model.Sample) {
	m.mu.Lock()
	sink := m.sink
	subs := make([]chan model.Sample, 0, len(m.subs))
	for _, ch := range m.subs {
		subs = append(subs, ch)
	}
	m.mu.Unlock()

	if sink != nil {
		sink(s)
	}
	for _, ch := range subs {
		select {
		case ch <- s:
		default:
			// A slow subscriber must never block the probes.
		}
	}
}
