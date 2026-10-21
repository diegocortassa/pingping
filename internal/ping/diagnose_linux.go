//go:build linux

package ping

import (
	"os"
	"strconv"
	"strings"
)

// capNetRaw is the number of the CAP_NET_RAW capability.
const capNetRaw = 13

// groupRange returns the range of the groups that may open ICMP sockets.
func groupRange() string {
	raw, err := os.ReadFile("/proc/sys/net/ipv4/ping_group_range")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

// capabilities returns the capabilities of the process that matter for ICMP.
func capabilities() string {
	raw, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if !strings.HasPrefix(line, "CapEff:") {
			continue
		}
		value, err := strconv.ParseUint(strings.TrimSpace(strings.TrimPrefix(line, "CapEff:")), 16, 64)
		if err != nil {
			return ""
		}
		if value&(1<<capNetRaw) != 0 {
			return "CAP_NET_RAW"
		}
		return "none"
	}
	return ""
}
