# TODO

## Done

- [x] Open log: size the Fyne file dialog to the main window (0.5.2).

- [x] Project skeleton, `go.mod`, dependencies (Fyne v2.8.2, pro-bing v0.9.1).
- [x] `PLAN.md` with the architecture and the feature breakdown.
- [x] `internal/config`: config model, JSON serialisation, defaults, paths.
- [x] `internal/ping`: ICMP wrapper with privilege fallback and timeout tracking.
- [x] `internal/monitor`: target lifecycle, sample fan out, injectable pinger factory.
- [x] `internal/history`: rolling buffer and statistics.
- [x] `internal/logstore`: daily rotated CSV writer with retention.
- [x] `internal/ui`: dashboard, target table, dialogs, settings, system tray.
- [x] `internal/ui/chart`: custom real time line chart widget.
- [x] Icon conversion (SVG -> PNG) with ImageMagick and `go:embed` embedding.
- [x] `Makefile` with build, test, audit, run, icons, deb, rpm, clean targets.
- [x] `README.md` with user and developer instructions.
- [x] Unit tests for config, history, logstore, monitor, ping, ui.
- [x] `DIARY.md` log of the work.
- [x] Dependency updates: `golang.org/x/image` v0.43.0, Go 1.27.3 for the
      `crypto/x509` advisory, `govulncheck` reports no findings.
- [x] Actionable error for the blocked unprivileged ICMP socket on Linux, the
      root cause is reported and the `ping_group_range` command is shown in
      the interface and in the log.
- [x] `pingping -diagnose` command that opens both ICMP sockets and reports
      the group range, the capabilities of the process and the fix.
- [x] Windows executable with the application icon and the version information
      of the file properties, embedded with go-winres.
- [x] Rename to PingPing: the module path, the binary, the configuration
      directory, the Fyne application identifier and the documentation.
- [x] One CSV log file per target, rotated every 24 hours.
- [x] `Graph` menu to change the graph window without opening the settings.
- [x] Store the logs in `$XDG_DATA_HOME` instead of the configuration directory.
- [x] Open an old CSV log in a graph window, with a range picker and a `-log` flag.

## Doing

- [ ] Verify the `net.ipv4.ping_group_range` hint on a desktop Linux with a
      normal user (the container has a read only `/proc/sys` and drops
      `CAP_NET_RAW`, even the system `ping` fails there).

- [ ] Final end to end verification on a real desktop session. The application
      was verified headless with Xvfb: the window is created, the probes run,
      the samples reach the log files and the CSV files are rotated daily.

## Todo

- [ ] Package release archives (zip/tar.gz) for Linux, Windows, macOS.
- [ ] Optional: export the collected samples to CSV/JSON for external analysis.
- [ ] Optional: per target graph window override.
- [ ] Optional: notification when a target goes down for longer than N probes.
- [ ] Optional: reload the configuration file when it changes on disk.
