# SuperPinger

A portable network monitoring tool. SuperPinger pings the addresses you configure,
shows the round trip time in real time, keeps a daily rotated log of every probe
and stays in the system tray while it monitors.

![SuperPinger](assets/icons/icon.png)

## Features

- Monitor any number of addresses, host names included.
- Per target ping interval and timeout.
- Real time round trip time graph, the visible window is configurable and
  defaults to one hour.
- Statistics per target: last, average, minimum, maximum, jitter and packet
  loss.
- Persistent daily rotated CSV logs with a configurable retention.
- Minimises to the system tray, the monitoring keeps running when the window is
  closed.
- Single portable binary, nothing to install.
- Linux, Windows and macOS.

## Screenshots

The main window shows the list of the monitored targets on the left and the
graph of the selected target on the right.

| Target | Interval | Status | Last | Average | Loss |
| ------ | -------- | ------ | ---- | ------- | ---- |
| 1.1.1.1 | 5s | up | 12.4ms | 11.9ms | 0.0% |
| 10.255.255.1 | 5s | down | - | - | 100.0% |

## Installation

SuperPinger is portable: download the archive of your platform, unpack it and
run the binary. There is no installer and no system wide configuration.

The program stores its configuration and its logs in the user configuration
directory of the operating system:

| Platform | Configuration | Logs |
| -------- | ------------- | ---- |
| Linux | `~/.config/superping/config.json` | `~/.config/superping/data/logs` |
| macOS | `~/Library/Application Support/superping/config.json` | `~/Library/Application Support/superping/data/logs` |
| Windows | `%AppData%\superping\config.json` | `%AppData%\superping\data\logs` |

## Usage

Start the application and add the addresses you want to watch with the `Add`
button or with `SuperPinger -> Add target`.

The add and edit dialog takes four fields:

| Field | Description |
| ----- | ----------- |
| Address | The host name or IP address to ping. |
| Interval | The delay between two probes, for example `5s` or `1m`. |
| Timeout | The time after which a probe is reported as lost, for example `3s`. |
| Enabled | Uncheck it to keep the target but stop probing it. |

Selecting a row of the table shows the graph of that target. The summary line
below the graph holds the statistics of the visible window.

`Settings` opens the application settings:

| Setting | Description |
| ------- | ----------- |
| Graph window | The time span shown by the graphs, from 15 minutes to 24 hours. |
| Keep logs for (days) | The number of days of logs that are kept, `0` keeps everything. |
| ICMP mode | How the ICMP sockets are opened, see below. |
| Log folder | The folder with the log files, it can be opened with a click. |

Closing the window hides it, the program keeps monitoring in the system tray.
Use the tray menu to show the window again or to quit.

### ICMP mode

| Mode | Behaviour |
| ---- | --------- |
| `auto` | Tries an unprivileged ICMP socket first and falls back to a privileged raw socket when the system refuses it. |
| `unprivileged` | Only uses the unprivileged ICMP datagram socket, needs `net.ipv4.ping_group_range` to include the user. |
| `privileged` | Always uses a raw socket, needs root or `CAP_NET_RAW`. |

On Windows the ICMP sockets need an elevated process, the `auto` mode picks the
working flavour by itself.

### Linux and unprivileged ICMP

Linux restricts the unprivileged ICMP sockets to the groups listed in
`net.ipv4.ping_group_range`, which is empty by default. As a normal user the
probes then fail with:

```
socket: permission denied: unprivileged ICMP is blocked, allow it with:
sudo sysctl -w net.ipv4.ping_group_range="0 2147483647"
```

The fix is one command, run it once as root:

```bash
sudo sysctl -w net.ipv4.ping_group_range="0 2147483647"
```

To make it permanent, create `/etc/sysctl.d/99-superping.conf` with:

```
net.ipv4.ping_group_range = 0 2147483647
```

and run `sudo sysctl --system`.

The same error is shown in the interface: the status column of the target
reads `error` and the line below the graph holds the reason. Without the
sysctl the only other option is to run SuperPinger as root or to grant it the
`CAP_NET_RAW` capability.

### Command line

```
Usage of superping:
  -config string
        path of the configuration file
  -data-dir string
        directory of the persistent data
  -version
        print the version and exit
```

## Logs

The probes are written to one CSV file per day, named `pings-YYYY-MM-DD.csv`,
inside the log folder:

```csv
timestamp,target_id,address,rtt_ms,success,error
2026-10-21T09:23:51.285661599Z,a1,127.0.0.1,0.045,true,
2026-10-21T09:25:35.328929207Z,b2,10.255.255.1,0.000,false,timeout
```

The `rtt_ms` column holds the round trip time in milliseconds, a failed probe
has `success` set to `false` and the reason in the `error` column.

## Configuration file

The configuration is written when the application starts and after every
change, so it can be edited by hand as well:

```json
{
  "targets": [
    {
      "id": "a1",
      "address": "1.1.1.1",
      "interval": "5s",
      "timeout": "3s",
      "enabled": true
    }
  ],
  "graphWindow": "1h",
  "logRetentionDays": 30,
  "icmpMode": "auto",
  "dataDir": "/home/user/.config/superping/data"
}
```

## Development

### Requirements

- Go 1.27.3 or newer.
- A C toolchain, Fyne uses cgo for the OpenGL driver.
- On Linux the OpenGL and X11 development headers, on Debian or Ubuntu:

```bash
sudo apt-get install libgl1-mesa-dev xorg-dev libglx-dev libwayland-dev \
    wayland-protocols libxkbcommon-dev librsvg2-bin
```

- ImageMagick, only to regenerate the icons from the SVG sources.

### Build

```bash
make build
```

The binary is written to `dist/superping`. The version is taken from the git
tags, it can be overridden:

```bash
make build VERSION=0.2.0
```

### Test

```bash
make test
```

### Quality control

```bash
make install-tools   # once, installs staticcheck and govulncheck
make audit
```

`make audit` runs `go mod tidy -diff`, `go mod verify`, `gofmt`, `go vet`,
`staticcheck` and `govulncheck`.

### Icons

The window and the tray icons are generated from the SVG sources in `assets`
with ImageMagick and embedded into the binary:

```bash
make icons
```

### Packages

| Package | Responsibility |
| ------- | -------------- |
| `internal/config` | Configuration model, JSON serialisation, defaults, paths. |
| `internal/ping` | ICMP probing, privilege fallback, per packet timeout tracking. |
| `internal/monitor` | Target lifecycle and fan out of the samples. |
| `internal/history` | Rolling sample buffer and the statistics. |
| `internal/logstore` | Daily rotated CSV writer with retention. |
| `internal/ui` | Dashboard, target table, dialogs, settings, system tray, graph. |
| `assets/icons` | Embedded PNG icons. |

### Packaging

```bash
make deb   # Debian package in dist
make rpm   # RPM package in dist
```

## License

BSD 3-Clause.
