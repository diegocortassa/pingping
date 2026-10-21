// Package ping wraps the ICMP echo implementation used by PingPing.
//
// The library underneath (github.com/prometheus-community/pro-bing) reports
// every packet it sends and every reply it receives, but it has no notion of
// a per packet deadline. This package adds that: a probe that is not answered
// within the configured timeout is reported as lost.
package ping

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/prometheus-community/pro-bing"

	"github.com/pingping/pingping/internal/config"
)

// Result is the outcome of a single probe.
type Result struct {
	// Time is the moment the probe was sent.
	Time time.Time
	// RTT is the round trip time, only valid when Success is true.
	RTT time.Duration
	// Success tells whether the host answered in time.
	Success bool
	// Offline tells whether the probe could not be sent at all, for example
	// because the ICMP socket could not be opened.
	Offline bool
	// Err describes why the probe failed.
	Err string
}

// Pinger probes a single address at a fixed interval.
type Pinger struct {
	address  string
	interval time.Duration
	timeout  time.Duration
	mode     config.ICMode
	factory  proberFactory
}

// New returns a pinger for the given address.
func New(address string, interval, timeout time.Duration, mode config.ICMode) *Pinger {
	return &Pinger{
		address:  address,
		interval: interval,
		timeout:  timeout,
		mode:     mode,
		factory:  defaultProberFactory,
	}
}

// prober runs a single ICMP session.
type prober interface {
	RunWithContext(ctx context.Context) error
}

// proberConfig holds the parameters of one ICMP session.
type proberConfig struct {
	address     string
	interval    time.Duration
	privileged  bool
	tracker     *tracker
	onRecv      func(time.Duration)
	onSendError func(error)
}

// proberFactory builds the prober of one ICMP session.
type proberFactory func(proberConfig) prober

// defaultProberFactory builds a pro-bing pinger.
func defaultProberFactory(cfg proberConfig) prober {
	pinger := probing.New(cfg.address)
	pinger.Interval = cfg.interval
	pinger.Count = -1
	pinger.RecordRtts = false
	pinger.RecordTTLs = false
	pinger.SetPrivileged(cfg.privileged)

	pinger.OnSend = func(pkt *probing.Packet) {
		cfg.tracker.sent(pkt.Seq, time.Now())
	}
	pinger.OnRecv = func(pkt *probing.Packet) {
		cfg.tracker.received(pkt.Seq)
		cfg.onRecv(pkt.Rtt)
	}
	pinger.OnSendError = func(_ *probing.Packet, err error) {
		cfg.onSendError(err)
	}
	return pinger
}

// Run probes the address until the context is cancelled. The returned channel
// carries one result per probe and is closed when the loop ends.
func (p *Pinger) Run(ctx context.Context) <-chan Result {
	out := make(chan Result, 16)
	go p.loop(ctx, out)
	return out
}

// loop runs the probe, retrying with a privileged socket when the operating
// system refuses the unprivileged one.
func (p *Pinger) loop(ctx context.Context, out chan<- Result) {
	defer close(out)

	attempts := p.attempts()
	var firstErr error
	for i, privileged := range attempts {
		err := p.probe(ctx, privileged, out)
		if ctx.Err() != nil {
			return
		}
		if err == nil {
			return
		}
		if firstErr == nil {
			firstErr = err
		}
		if i == len(attempts)-1 || !isPermissionError(err) {
			p.emit(ctx, out, Result{
				Time:    time.Now(),
				Offline: true,
				Err:     describeError(firstErr, err, len(attempts)),
			})
			return
		}
	}
}

// describeError builds the message of a session that could not be started.
//
// The unprivileged socket is tried first, so its error is the root cause and
// the one the user can act on. The error of the privileged fallback is only
// added as a remark.
func describeError(first, last error, attempts int) string {
	if attempts == 1 || first == nil || !isPermissionError(first) {
		return last.Error()
	}

	message := first.Error()
	if hint := privilegeHint(); hint != "" {
		message += ": " + hint
	}
	message += " (the privileged fallback failed too: " + last.Error() + ")"
	return message
}

// attempts returns the socket flavours to try, in order.
func (p *Pinger) attempts() []bool {
	switch p.mode {
	case config.ICModePrivileged:
		return []bool{true}
	case config.ICModeUnprivileged:
		return []bool{false}
	default:
		return []bool{false, true}
	}
}

// probe runs a single ICMP session until the context is cancelled.
func (p *Pinger) probe(ctx context.Context, privileged bool, out chan<- Result) error {
	tracker := newTracker(p.timeout)
	session := p.factory(proberConfig{
		address:    p.address,
		interval:   p.interval,
		privileged: privileged,
		tracker:    tracker,
		onRecv: func(rtt time.Duration) {
			p.emit(ctx, out, Result{Time: time.Now(), RTT: rtt, Success: true})
		},
		onSendError: func(err error) {
			p.emit(ctx, out, Result{Time: time.Now(), Err: err.Error()})
		},
	})

	stop := make(chan struct{})
	watchdog := p.watchdog(ctx, stop, tracker, out)
	err := session.RunWithContext(ctx)
	close(stop)
	<-watchdog

	if ctx.Err() != nil {
		return nil
	}
	return err
}

// watchdog reports the probes that were not answered in time.
func (p *Pinger) watchdog(ctx context.Context, stop <-chan struct{}, tracker *tracker, out chan<- Result) chan struct{} {
	done := make(chan struct{})

	go func() {
		defer close(done)

		tick := p.timeout / 4
		if tick < 50*time.Millisecond {
			tick = 50 * time.Millisecond
		}
		if tick > time.Second {
			tick = time.Second
		}
		ticker := time.NewTicker(tick)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-stop:
				return
			case now := <-ticker.C:
				for range tracker.expired(now) {
					p.emit(ctx, out, Result{Time: now, Err: "timeout"})
				}
			}
		}
	}()

	return done
}

// emit delivers a result without blocking forever when nobody listens.
func (p *Pinger) emit(ctx context.Context, out chan<- Result, r Result) {
	select {
	case out <- r:
	case <-ctx.Done():
	}
}

// tracker keeps the sequence numbers of the probes that are still in flight.
type tracker struct {
	mu      sync.Mutex
	timeout time.Duration
	pending map[int]time.Time
}

func newTracker(timeout time.Duration) *tracker {
	return &tracker{timeout: timeout, pending: make(map[int]time.Time)}
}

func (t *tracker) sent(seq int, at time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.pending[seq] = at
}

func (t *tracker) received(seq int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.pending, seq)
}

// expired removes and returns the probes that timed out.
func (t *tracker) expired(now time.Time) []int {
	t.mu.Lock()
	defer t.mu.Unlock()

	var lost []int
	for seq, sent := range t.pending {
		if now.Sub(sent) >= t.timeout {
			lost = append(lost, seq)
		}
	}
	for _, seq := range lost {
		delete(t.pending, seq)
	}
	return lost
}

// isPermissionError reports whether the error is caused by the missing
// privileges needed to open a raw ICMP socket.
func isPermissionError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, os.ErrPermission) {
		return true
	}
	msg := strings.ToLower(err.Error())
	for _, needle := range []string{
		"permission denied",
		"operation not permitted",
		"access is denied",
		"not supported",
		"protocol",
		"privilege",
	} {
		if strings.Contains(msg, needle) {
			return true
		}
	}
	return false
}
