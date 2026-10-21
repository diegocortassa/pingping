//go:build !linux

package ping

// privilegeHint explains how to allow ICMP sockets outside Linux, where the
// raw socket needs elevated rights.
func privilegeHint() string {
	return "the raw ICMP socket needs root or the CAP_NET_RAW capability"
}
