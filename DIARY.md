# DIARY

## 2026-10-21 - Project start

### Observations

- The repository only contained the two SVG icons (`assets/icon.svg`,
  `assets/tray.svg`), the `AGENTS.md` rules and the `scratch/` folder.
- Go 1.27.2 and ImageMagick are available. Fyne needs X11/OpenGL development
  headers, they were installed with `apt-get install libgl1-mesa-dev xorg-dev`.
- `fyne.io/fyne/v2` v2.8.2 is the current release. It has no built in chart
  widget, so the graph is implemented as a custom widget.
- `github.com/prometheus-community/pro-bing` v0.9.1 is the maintained ICMP
  library. Important details found while reading the source:
  - a `Pinger` instance is single use, `Stop()` closes an internal channel and
    the instance cannot be restarted, therefore one pinger is created per
    target and runs continuously,
  - `Timeout` is a ticker started when the pinger starts, not a per packet
    deadline, so per packet timeouts have to be implemented by the caller,
  - `OnSend` reports the sequence number of every outgoing packet and
    `OnRecv` the sequence number plus the RTT, which is enough to detect
    timeouts by tracking in flight sequence numbers,
  - the default protocol is `udp4` (unprivileged ICMP datagram socket) and
    `SetPrivileged(true)` switches to a raw socket. On Windows the raw socket
    is mandatory.
- The container runs as root with `net.ipv4.ping_group_range = "0 0"`, so the
  unprivileged path is not usable here and the privileged fallback is the one
  that gets exercised.

### Decisions

- Config and data live in the OS user config directory
  (`~/.config/superping` on Linux) instead of next to the binary: the binary
  may live on a read only medium. The path can be overridden with
  `--config` and `--data-dir`.
- Durations are stored in JSON as human readable strings (`"5s"`) through a
  small `config.Duration` type instead of raw nanoseconds.
- The chart renders the plot area into a `canvas.Raster` (fast with thousands
  of points) and draws the axis labels with regular Fyne text widgets so they
  follow the theme and the display scaling.

## 2026-10-21 - Verification

### Observations

- `govulncheck` found five reachable advisories in `golang.org/x/image`
  v0.24.0, a transitive dependency of Fyne, and one in the Go standard
  library (`crypto/x509`, fixed in go1.27.3). The x/image dependency was
  upgraded to v0.43.0 and the `go` directive of `go.mod` was raised to
  1.27.3, the scan is clean now.
- Fyne 2.8 changed several APIs compared to the older releases:
  `fyne.NewMainMenu` replaces the old `*fyne.Menu` argument,
  `widget.NewTable` returns the length as `(rows, cols)`, `TableCellID` uses
  `Col`, `fyne.TextStyle` is a struct and `theme.Color` needs a variant.
- The application was verified headless with Xvfb. The window is created with
  the expected size, the probes run, the samples reach the daily CSV files and
  the tray icon is installed. The system tray needs a running DBus session,
  without it Fyne only logs an error and the window still works.
- `history.statsOf` divided by `ok-1` to compute the jitter, which crashed with
  a division by zero when a target had exactly one successful probe. The
  jitter is now only computed when there are at least two successful probes,
  a regression test covers the single sample case.

### Decisions

- The user interface is built by `App.setup`, `Run` only creates the Fyne
  application and shows the window. This makes the whole interface testable
  with the headless Fyne test driver.
- `golang.org/x/image` is pinned to a patched version even though it is an
  indirect dependency, so the audit does not depend on the Fyne release
  cadence.
- The ICMP session is built by a `proberFactory`, the real one wraps
  pro-bing. The tests inject a fake session, which is the only way to test
  the privilege fallback without a special container.
- The watchdog of a probe session had its own stop channel. Before that the
  fallback to the privileged socket never happened: the watchdog only ended
  with the context, so a session that failed to start blocked the loop
  forever. The bug was found while testing the fallback with a fake session.
- Durations are written in a compact form (`1h`, `1m30s`) instead of the Go
  default (`1h0m0s`), the output is still valid for `time.ParseDuration`.

## 2026-10-21 - Linux unprivileged ICMP

### Observations

- A normal user on Linux got `socket: permission denied` with the
  unprivileged mode and `listen ip4:icmp : socket: operation not permitted`
  with the privileged one. The kernel restricts the ICMP_PROTO datagram
  sockets to the groups listed in `net.ipv4.ping_group_range`, which is empty
  by default, and the raw socket needs `CAP_NET_RAW`.
  See https://www.kernel.org/doc/html/latest/networking/ip-sysctl.html
- The fix is `sudo sysctl -w net.ipv4.ping_group_range="0 2147483647"`, the
  same value Debian and Ubuntu ship in `/usr/lib/sysctl.d/iputils-ping.conf`.
- Reproduced in the container with the `ubuntu` user (uid 1000) and the range
  `0 0`: the unprivileged socket fails with `EACCES` and the privileged one
  with `EPERM`, exactly as reported.
- The `auto` mode reported the error of the last attempt, the privileged one,
  which hides the root cause. The unprivileged error is the one the user can
  act on.

### Decisions

- The error of a session that could not be started now carries a hint with the
  exact command, the hint is built per platform in
  `internal/ping/privilege_linux.go` and `internal/ping/privilege_other.go`.
- The `auto` mode reports the unprivileged error as the root cause and adds
  the error of the privileged fallback as a remark.
- The samples carry an `offline` flag, it tells the interface that the probe
  was never sent, so the status column reads `error` instead of `down` and the
  reason is shown below the graph.
