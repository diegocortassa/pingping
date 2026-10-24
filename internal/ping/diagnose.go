package ping

import (
	"fmt"
	"os/user"
	"runtime"
	"strings"

	"golang.org/x/net/icmp"
)

const linuxGroupRangeCommand = `sudo sysctl -w net.ipv4.ping_group_range="0 2147483647"`

// Report describes what the system allows for the ICMP probes.
type Report struct {
	// OS is the operating system the report was made on.
	OS string
	// User is the user the application runs as.
	User string
	// GroupRange is the content of net.ipv4.ping_group_range, empty outside
	// Linux.
	GroupRange string
	// Capabilities are the capabilities of the process that matter for ICMP.
	Capabilities string
	// Unprivileged is the error of the unprivileged ICMP socket, nil when the
	// socket can be opened.
	Unprivileged error
	// Privileged is the error of the raw ICMP socket, nil when the socket can
	// be opened.
	Privileged error
}

// Diagnose opens both ICMP sockets and reports what the system allows.
func Diagnose() Report {
	return Report{
		OS:           runtime.GOOS,
		User:         currentUser(),
		GroupRange:   groupRange(),
		Capabilities: capabilities(),
		Unprivileged: testSocket("udp4"),
		Privileged:   testSocket("ip4:icmp"),
	}
}

// String renders the report for the terminal.
func (r Report) String() string {
	var b strings.Builder

	fmt.Fprintf(&b, "OS: %s\n", r.OS)
	fmt.Fprintf(&b, "user: %s\n", r.User)
	if r.GroupRange != "" {
		fmt.Fprintf(&b, "net.ipv4.ping_group_range: %s\n", r.GroupRange)
	}
	if r.Capabilities != "" {
		fmt.Fprintf(&b, "capabilities: %s\n", r.Capabilities)
	}
	b.WriteString("\n")

	writeSocket := func(name string, err error) {
		if err == nil {
			fmt.Fprintf(&b, "%s: OK\n", name)
			return
		}
		fmt.Fprintf(&b, "%s: FAILED\n    %v\n", name, err)
	}
	writeSocket("unprivileged ICMP socket (SOCK_DGRAM)", r.Unprivileged)
	writeSocket("privileged ICMP socket (SOCK_RAW)", r.Privileged)

	if r.Unprivileged == nil && r.Privileged == nil {
		b.WriteString("\nBoth sockets can be opened, the probes will work.\n")
		return b.String()
	}

	b.WriteString("\nHow to fix it:\n")
	if r.Unprivileged != nil {
		b.WriteString("  the unprivileged socket needs the group of the user inside\n")
		b.WriteString("  net.ipv4.ping_group_range:\n")
		b.WriteString("      " + linuxGroupRangeCommand + "\n")
	}
	if r.Privileged != nil {
		b.WriteString("  the privileged socket needs root or the CAP_NET_RAW capability:\n")
		b.WriteString("      sudo setcap cap_net_raw=ep <path of the binary>\n")
	}
	return b.String()
}

// testSocket opens an ICMP socket and closes it again.
func testSocket(network string) error {
	conn, err := icmp.ListenPacket(network, "0.0.0.0")
	if err != nil {
		return err
	}
	return conn.Close()
}

// currentUser returns the user of the process.
func currentUser() string {
	account, err := user.Current()
	if err != nil {
		return "unknown"
	}
	return fmt.Sprintf("%s (uid=%s, gid=%s)", account.Username, account.Uid, account.Gid)
}
