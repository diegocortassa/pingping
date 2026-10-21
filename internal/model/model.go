// Package model holds the data types shared by the monitoring, history and
// logging packages.
package model

import "time"

// Sample is the result of a single probe of a monitored address.
type Sample struct {
	// TargetID identifies the monitored address the sample belongs to.
	TargetID string `json:"targetId"`
	// Address is the host name or IP address that was probed.
	Address string `json:"address"`
	// Time is the moment the probe was sent.
	Time time.Time `json:"timestamp"`
	// RTT is the round trip time, only meaningful when Success is true.
	RTT time.Duration `json:"-"`
	// Success tells whether the host answered in time.
	Success bool `json:"success"`
	// Offline tells whether the probe could not be sent at all, for example
	// because the ICMP socket could not be opened.
	Offline bool `json:"offline,omitempty"`
	// Error describes why the probe failed.
	Error string `json:"error,omitempty"`
}
