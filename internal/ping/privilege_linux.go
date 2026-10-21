//go:build linux

package ping

// privilegeHint explains how to allow unprivileged ICMP sockets on Linux.
//
// The kernel restricts the ICMP_PROTO datagram sockets to the groups listed in
// net.ipv4.ping_group_range, which is empty by default.
//
// See https://www.kernel.org/doc/html/latest/networking/ip-sysctl.html
func privilegeHint() string {
	return `unprivileged ICMP is blocked, allow it with: sudo sysctl -w net.ipv4.ping_group_range="0 2147483647"`
}
