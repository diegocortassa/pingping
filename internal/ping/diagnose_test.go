package ping

import (
	"runtime"
	"strings"
	"testing"
)

func TestDiagnoseReportsTheSockets(t *testing.T) {
	report := Diagnose()

	if report.OS != runtime.GOOS {
		t.Fatalf("got OS %q, want %q", report.OS, runtime.GOOS)
	}
	if report.User == "" {
		t.Fatal("the user is missing")
	}

	text := report.String()
	if !strings.Contains(text, "unprivileged ICMP socket") {
		t.Fatalf("the unprivileged socket is missing in %q", text)
	}
	if !strings.Contains(text, "privileged ICMP socket") {
		t.Fatalf("the privileged socket is missing in %q", text)
	}
}

func TestReportExplainsAFailure(t *testing.T) {
	report := Report{
		OS:           "linux",
		User:         "user (uid=1000, gid=1000)",
		GroupRange:   "1 0",
		Capabilities: "none",
		Unprivileged: errFake{},
		Privileged:   errFake{},
	}

	text := report.String()
	for _, want := range []string{linuxGroupRangeCommand, "setcap", "FAILED"} {
		if !strings.Contains(text, want) {
			t.Fatalf("%q is missing in %q", want, text)
		}
	}
}

func TestReportWithoutFailures(t *testing.T) {
	report := Report{OS: "linux", User: "root", GroupRange: "0 0", Capabilities: "CAP_NET_RAW"}
	text := report.String()

	if !strings.Contains(text, "OK") {
		t.Fatalf("the working sockets are missing in %q", text)
	}
	if strings.Contains(text, "How to fix it") {
		t.Fatalf("no fix is needed but the report asks for one: %q", text)
	}
}

type errFake struct{}

func (errFake) Error() string { return "socket: permission denied" }
