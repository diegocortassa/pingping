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
- The `ping` command needs no sysctl because the binary carries the privilege
  itself: `getcap /usr/bin/ping` shows `cap_net_raw=ep` on Ubuntu and Debian,
  older distributions use the setuid bit. The sysctl has nothing to do with
  the command, it exists for the programs that have no privilege at all.
- The fix is `sudo sysctl -w net.ipv4.ping_group_range="0 2147483647"`, the
  same value Debian and Ubuntu ship in `/usr/lib/sysctl.d/iputils-ping.conf`.
  The alternative is `sudo setcap cap_net_raw=ep <binary>`, the same
  capability the `ping` binary has.
- Reproduced in the container with the `ubuntu` user (uid 1000) and the range
  `0 0`: the unprivileged socket fails with `EACCES` and the privileged one
  with `EPERM`, exactly as reported. The container also drops `CAP_NET_RAW`
  from the bounding set, so even the system `ping` fails there.
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
- `superping -diagnose` opens both ICMP sockets and prints what the system
  allows, the group range and the capabilities of the process, with the
  command that fixes it. It answers the question the error alone does not:
  why does the `ping` command work without any change.

## 2026-10-21 - Default timeout

### Observations

- The default timeout of three seconds was too long for a monitor that shows
  the state of a connection: a target that stops answering keeps the last
  state for three seconds before it is reported as lost.
- The timeout is a per packet deadline implemented by the tracker of the probe
  session, the watchdog checks the in flight packets every `timeout/4` with a
  floor of 50ms and a ceiling of one second. A one second timeout therefore
  checks every 250ms.

### Decisions

- The default timeout is now one second, the default interval of five seconds
  stays. The timeout must stay smaller than the interval, otherwise a reply
  that arrives in time would still be reported as lost, a test pins the
  relation between the two defaults.
- The default applies to the new targets only, the targets that already exist
  keep the timeout of their configuration file.

## 2026-10-21 - Help menu and about dialog

### Observations

- The window had a single menu, the application menu, there was no way to see
  the version or the author of the program from the interface.
- The version was a variable of the `main` package, so the interface could not
  show it. The build sets it with `-X main.version`, the flag has to name the
  variable of the package that holds it.

### Decisions

- The metadata of the application lives in `internal/appinfo`: the name, the
  author, a short description and the version. The build sets the version with
  `-X github.com/superping/superping/internal/appinfo.Version`, the `main`
  package and the interface read the same value, so the about dialog and the
  `-version` flag can never disagree.
- The about dialog is built by `aboutContent`, a separate function, so the
  content can be tested without opening a modal window.
- The window now has a `Help` menu with the `About` item, the dialog shows the
  application icon, the name, the version and the author.

## 2026-10-21 - Toolbar

### Observations

- The settings were reachable from two places, the toolbar button and the
  application menu. The menu is the canonical place, the button was a
  duplicate.

### Decisions

- The toolbar holds only the target actions: add, edit and remove. The
  settings stay in the `SuperPinger` menu.
- The application keeps a reference to the toolbar, a test pins the buttons of
  the toolbar and checks that the settings are still in the menu.

## 2026-10-21 - Windows build

### Observations

- The Windows executable was built with a command typed by hand, the knowledge
  was not in the repository, so every release had to rediscover it.
- Fyne needs cgo, so the cross compiler is mandatory. Go turns cgo off by
  itself when the target platform differs from the host, the build has to set
  `CGO_ENABLED=1` explicitly. The Fyne build tool does the same, see
  https://docs.fyne.io/started/cross-compiling
- A console subsystem binary opens a black console window next to the
  interface, the Fyne build tool links the Windows binaries against the GUI
  subsystem with `-H=windowsgui`.

### Decisions

- The Makefile has a `build-windows` target, it cross compiles the executable
  with `CGO_ENABLED=1 GOOS=windows GOARCH=amd64` and the mingw compiler. The
  compiler can be overridden with `WIN_CC`.
- The Windows binary is linked with `-H=windowsgui`, so it does not open a
  console window.

## 2026-10-21 - Windows log folder

### Observations

- On Windows the button "Open log folder" failed with
  `parse "file://C:\Users\dcortassa\AppData\Roaming\superping\data\logs": invalid port ":\Users\..." after host`.
- The URL was built by concatenating the scheme and the path. A Windows path
  is not a URL path: the backslashes are invalid and the drive letter is
  read as the host, so the rest of the path is parsed as the port.

### Decisions

- The file URL is built with `url.URL`, the path is made absolute, the
  separators are slashes and a Windows drive gets the leading slash of a URL
  path: `file:///C:/Users/dcortassa/AppData/Roaming/superping/data/logs`.
- The helper `fileURL` is split in two, the part that builds the URL from a
  slash path is testable on every platform, so the Windows case is covered by
  the tests of a Linux build.
- The URL is escaped by the URL builder, a log folder with a space in the path
  works too.

## 2026-10-21 - Windows resources

### Observations

- The Windows executable had no icon, Windows shows the default one of the
  applications. The icon of the application was only embedded in the binary
  for Fyne, the shell does not read it from there.
- Fyne sets the icon of the window at runtime, the icon of the file in the
  explorer is a resource of the executable, it needs the resource section of
  the PE file.

### Decisions

- The `windows-resource` target runs go-winres, it writes the icon, the
  version information and the manifest into `rsrc_windows_amd64.syso`. The Go
  linker embeds the file into the executable, the suffix keeps it out of the
  builds for the other platforms.
- The icon is the PNG of the application, go-winres resizes it to the sizes
  Windows expects (256, 64, 48, 32 and 16 pixels).
- The manifest is the GUI one, it asks for the per monitor v2 DPI awareness
  and for the administrator rights, the raw ICMP sockets of Windows need
  them.
- The version of the file properties is numeric, the tag is cleaned of its
  prefix and of the dirty suffix, so the properties show `0.1.7.0` and not
  `v0.1.7-1-gca5b93f-dirty`.

## 2026-10-21 - Configuration file in the working directory

### Observations

- The application is portable, the binary can be copied anywhere, but the
  configuration always came from the user configuration directory. A user
  with several setups, or with a configuration shared between machines, had
  to move the file by hand every time.

### Decisions

- A `config.json` in the working directory replaces the one of the user
  configuration directory. The `-config` flag keeps the priority over both,
  so nothing changes for the users that pass the path explicitly.
- The path of the file in use is shown in the settings dialog, the user does
  not have to guess which one was picked up.
- The data directory is not moved, it stays the one of the configuration
  file. A fully portable installation needs a `dataDir` in the local
  configuration file, the value is written to it the first time the file is
  saved.

## 2026-10-21 - Graph window changes

### Observations

- Changing the graph window in the settings from one hour to fifteen
  minutes and back to one hour made the chart show only the last fifteen
  minutes. The history was pruned when the window was changed, so the old
  samples were lost.

### Decisions

- The samples are kept in memory, the window is only used when computing
  the statistics and when drawing the chart. Changing the window does not
  drop samples anymore, so expanding it again shows the previous data.
- The pruning is only done by the time of reading, and the safety cap is
  still enforced when a sample is added.

## 2026-10-21 - Graph time axis direction

### Observations

- The time axis labels went from now on the left to the oldest on the right,
  but the data was plotted with old samples on the left and new samples on
  the right. The labels were inverted relative to the data.

### Decisions

- The labels are now calculated from right to left, the right edge shows
  "now" and the left edge shows "oldest - window".

## 2026-10-21 - Sliding time axis

### Observations

- After fixing the labels, they stayed in the same positions even as time
  passed. The user expects the time axis to slide to the right in sync with
  the graph, like a real time plot.

### Decisions

- The chart refreshes every second on its own ticker, updating both the axis
  labels and the raster. The labels are computed from "now", so they slide
  continuously as time passes.
