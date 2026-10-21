package ping

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/superping/superping/internal/config"
)

// fakeProber simulates an ICMP session.
type fakeProber struct {
	cfg    proberConfig
	err    error
	rtt    time.Duration
	answer bool
	send   bool
}

func (f fakeProber) RunWithContext(ctx context.Context) error {
	if f.err != nil {
		return f.err
	}
	if f.send {
		f.cfg.tracker.sent(1, time.Now())
	}
	if f.answer {
		f.cfg.tracker.received(1)
		f.cfg.onRecv(f.rtt)
	}
	<-ctx.Done()
	return nil
}

func newTestPinger(mode config.ICMode, factory proberFactory) *Pinger {
	p := New("1.1.1.1", time.Second, time.Second, mode)
	p.factory = factory
	return p
}

func TestFallsBackToThePrivilegedSocket(t *testing.T) {
	var mu sync.Mutex
	var attempts []bool

	factory := func(cfg proberConfig) prober {
		mu.Lock()
		attempts = append(attempts, cfg.privileged)
		mu.Unlock()
		if !cfg.privileged {
			return fakeProber{cfg: cfg, err: errors.New("socket: operation not permitted")}
		}
		return fakeProber{cfg: cfg, rtt: 12 * time.Millisecond, answer: true, send: true}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	results := newTestPinger(config.ICModeAuto, factory).Run(ctx)

	select {
	case result := <-results:
		if !result.Success || result.RTT != 12*time.Millisecond {
			t.Fatalf("unexpected result %+v", result)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for a result")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(attempts) != 2 || attempts[0] || !attempts[1] {
		t.Fatalf("got attempts %v, want [false true]", attempts)
	}
}

func TestAutoModeReportsTheRootCause(t *testing.T) {
	factory := func(cfg proberConfig) prober {
		if !cfg.privileged {
			return fakeProber{cfg: cfg, err: errors.New("socket: permission denied")}
		}
		return fakeProber{cfg: cfg, err: errors.New("listen ip4:icmp : socket: operation not permitted")}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	results := newTestPinger(config.ICModeAuto, factory).Run(ctx)

	select {
	case result := <-results:
		if result.Success {
			t.Fatal("expected a failed result")
		}
		if !result.Offline {
			t.Fatal("expected an offline result")
		}
		if !strings.Contains(result.Err, "socket: permission denied") {
			t.Fatalf("the root cause is missing in %q", result.Err)
		}
		if strings.Contains(result.Err, "operation not permitted") == false {
			t.Fatalf("the fallback error is missing in %q", result.Err)
		}
		if !strings.Contains(result.Err, "ping_group_range") && !strings.Contains(result.Err, "CAP_NET_RAW") {
			t.Fatalf("the hint is missing in %q", result.Err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for a result")
	}
}

func TestUnprivilegedModeReportsTheHint(t *testing.T) {
	factory := func(cfg proberConfig) prober {
		return fakeProber{cfg: cfg, err: errors.New("socket: permission denied")}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	results := newTestPinger(config.ICModeUnprivileged, factory).Run(ctx)

	select {
	case result := <-results:
		if !result.Offline {
			t.Fatal("expected an offline result")
		}
		if !strings.Contains(result.Err, "socket: permission denied") {
			t.Fatalf("the cause is missing in %q", result.Err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for a result")
	}
}

func TestDoesNotFallBackOnOtherErrors(t *testing.T) {
	var mu sync.Mutex
	var attempts int

	factory := func(cfg proberConfig) prober {
		mu.Lock()
		attempts++
		mu.Unlock()
		return fakeProber{cfg: cfg, err: errors.New("no such host")}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	results := newTestPinger(config.ICModeAuto, factory).Run(ctx)

	select {
	case result := <-results:
		if result.Success || result.Err == "" {
			t.Fatalf("unexpected result %+v", result)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for a result")
	}

	mu.Lock()
	defer mu.Unlock()
	if attempts != 1 {
		t.Fatalf("got %d attempts, want 1", attempts)
	}
}

func TestPrivilegedModeUsesOneAttempt(t *testing.T) {
	var mu sync.Mutex
	var attempts []bool

	factory := func(cfg proberConfig) prober {
		mu.Lock()
		attempts = append(attempts, cfg.privileged)
		mu.Unlock()
		return fakeProber{cfg: cfg, rtt: time.Millisecond, answer: true, send: true}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	results := newTestPinger(config.ICModePrivileged, factory).Run(ctx)
	<-results

	mu.Lock()
	defer mu.Unlock()
	if len(attempts) != 1 || !attempts[0] {
		t.Fatalf("got attempts %v, want [true]", attempts)
	}
}

func TestUnansweredProbeIsReportedAsTimeout(t *testing.T) {
	factory := func(cfg proberConfig) prober {
		return fakeProber{cfg: cfg, send: true}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p := New("1.1.1.1", time.Second, 300*time.Millisecond, config.ICModeAuto)
	p.factory = factory

	results := p.Run(ctx)

	select {
	case result := <-results:
		if result.Success || result.Err != "timeout" {
			t.Fatalf("unexpected result %+v", result)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for the timeout result")
	}
}

func TestTrackerExpiresAfterTimeout(t *testing.T) {
	tracker := newTracker(time.Second)
	now := time.Now()

	tracker.sent(1, now)
	tracker.sent(2, now.Add(-2*time.Second))

	lost := tracker.expired(now)
	if len(lost) != 1 || lost[0] != 2 {
		t.Fatalf("got %v, want [2]", lost)
	}
	if again := tracker.expired(now); len(again) != 0 {
		t.Fatalf("got %v, want no expired sequence", again)
	}
}

func TestTrackerReceivedRemovesPending(t *testing.T) {
	tracker := newTracker(time.Second)
	now := time.Now()

	tracker.sent(7, now)
	tracker.received(7)

	if lost := tracker.expired(now.Add(2 * time.Second)); len(lost) != 0 {
		t.Fatalf("got %v, want no expired sequence", lost)
	}
}

func TestIsPermissionError(t *testing.T) {
	permission := []error{
		os.ErrPermission,
		errors.New("socket: operation not permitted"),
		errors.New("dial udp4: connect: permission denied"),
		errors.New("socket: The requested protocol has not been configured into the system"),
		errors.New("socket: access is denied"),
	}
	for _, err := range permission {
		if !isPermissionError(err) {
			t.Fatalf("%v should be a permission error", err)
		}
	}

	other := []error{
		nil,
		errors.New("no such host"),
		errors.New("i/o timeout"),
	}
	for _, err := range other {
		if isPermissionError(err) {
			t.Fatalf("%v should not be a permission error", err)
		}
	}
}

func TestAttemptsFollowTheMode(t *testing.T) {
	if got := len((&Pinger{mode: "auto"}).attempts()); got != 2 {
		t.Fatalf("got %d attempts, want 2", got)
	}
	if got := len((&Pinger{mode: "unprivileged"}).attempts()); got != 1 {
		t.Fatalf("got %d attempts, want 1", got)
	}
	privileged := (&Pinger{mode: "privileged"}).attempts()
	if len(privileged) != 1 || !privileged[0] {
		t.Fatalf("got %v, want [true]", privileged)
	}
}
