# SuperPinger - Development Plan

## 1. Goal

Build a portable network monitoring desktop application that:

- pings one or more user configured addresses on a per-target interval/timeout,
- plots round trip time over time in real time (graph window configurable, default 1 hour),
- keeps a persistent, daily rotated log of every ping result,
- keeps running in the system tray when the window is closed,
- runs on Linux, Windows and macOS from a single portable binary.

## 2. Technology decisions

| Concern | Choice | Rationale |
|---|---|---|
| Language | Go 1.27 | Required, portable single binary |
| GUI | Fyne v2 (`fyne.io/fyne/v2`) | Required by project rules, native look on all 3 OSes |
| System tray | `desktop.App` (`SetSystemTrayIcon/Menu/Window`) | Built into Fyne, no extra dependency |
| ICMP | `github.com/prometheus-community/pro-bing` | Maintained cross platform ICMP library, unprivileged mode on Linux, raw mode on Windows/macOS |
| Config | JSON file in the OS user config dir | Human readable, `camelCase` fields per project conventions |
| Logs | CSV, one file per calendar day | Simple, greppable, rotated every 24h |
| Graph | Custom Fyne widget (`canvas.Raster` + `canvas.Text`) | No third party chart dependency, full control over styling |
| Icons | SVG source in `assets/`, converted to PNG with ImageMagick, embedded with `go:embed` | No runtime asset dependency, fully portable |

### ICMP privilege model

`pro-bing` opens a `udp4` ICMP datagram socket on Linux (unprivileged, needs
`net.ipv4.ping_group_range`) and a raw socket elsewhere. The app therefore tries
unprivileged first and automatically falls back to privileged mode when the
socket cannot be opened. The user can force privileged mode from the settings
dialog.

Timeout detection is implemented by the application, not by the library: the
library reports every packet it sends (`OnSend`) and every reply (`OnRecv`), so
the app tracks in flight sequence numbers and reports a timeout when a reply
does not arrive within the per target timeout.

## 3. Architecture

```
main.go                     flags, wiring, lifecycle
internal/config             Config/Target model, JSON (de)serialisation, defaults, paths
internal/ping               ICMP pinger wrapper, privilege fallback, timeout tracking
internal/monitor            owns targets, drives pings, publishes samples to subscribers
internal/history            rolling per target sample buffer + statistics
internal/logstore           daily rotated CSV writer with retention pruning
internal/ui                 Fyne window, tray, target table, dialogs, chart widget
internal/ui/chart           custom real time line chart widget
assets/                     SVG sources
assets/icons/               generated PNG assets (embedded)
```

Data flow:

```
ping.Pinger --> monitor.Monitor --(fan out)--> subscriber channel
                    |                                 |
                    +--> history (rolling buffer)    +--> UI (chart + table)
                    +--> logstore (CSV, daily rotation)
```

The monitor never touches the UI directly, the UI never blocks on network IO.
All Fyne widget updates are funnelled through `fyne.Do`.

## 4. Feature breakdown

### 4.1 Targets
- Address (host name or IP), interval, timeout, enabled flag, stable id.
- Add / edit / remove through a dialog, persisted immediately.

### 4.2 Dashboard
- Left: table of targets (address, interval, status, last RTT, average, loss).
- Right: tab container with one chart + statistics per target.
- Toolbar: add, edit, remove. The settings are in the application menu, the log
  folder is opened from the settings dialog.

### 4.3 Chart
- X axis: time (graph window, default 1 hour).
- Y axis: round trip time in milliseconds, auto scaled.
- Gaps in the line mark packet loss, red markers mark failed probes.
- Data is downsampled to at most one bucket per pixel column.

### 4.4 Logging
- `<dataDir>/logs/pings-YYYY-MM-DD.csv` with header
  `timestamp,target_id,address,rtt_ms,success,error` (snake_case, as the
  project conventions require for stored records).
- New file every calendar day (24h rotation).
- Old files pruned after the configured retention (default 30 days, 0 keeps all).

### 4.5 Tray behaviour
- Closing the window hides it, monitoring continues.
- Tray menu: Show, Hide, Quit.
- Left click on the tray icon brings the window back.

### 4.6 Settings
- Graph window (15m, 30m, 1h, 2h, 6h, 24h).
- Log retention in days.
- Force privileged ICMP mode.
- Log folder path.

## 5. Versioning

Semantic versioning. `0.1.0` is the first release: the feature set described
above is complete for the initial version.

## 6. Build and packaging

- `make build` - compile the binary into `dist/` for the host platform.
- `make test` - run the unit tests.
- `make audit` - `go mod tidy -diff`, `go mod verify`, `gofmt`, `go vet`,
  `staticcheck`, `govulncheck`.
- `make icons` - regenerate the PNG assets from the SVG sources.
- `make deb` / `make rpm` - packaging.

## 7. Test strategy

- `internal/config`: JSON round trip, defaults, validation, duration parsing.
- `internal/history`: pruning, statistics computation.
- `internal/logstore`: rotation at the day boundary, file content, retention.
- `internal/monitor`: target lifecycle with a fake pinger.
- `internal/ping`: sequence tracking and timeout detection with a fake transport.
- `internal/ui`: headless smoke test with the Fyne test driver.
