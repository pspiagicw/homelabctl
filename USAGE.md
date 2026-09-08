# homelabctl — Usage Guide

Example-driven walkthrough of the `homelabctl` CLI. For the project overview, architecture, and design rationale, see [`README.md`](./README.md).

---

## Table of Contents

- [Installation](#installation)
- [Quickstart](#quickstart)
- [Checking status](#checking-status)
- [Managing nodes](#managing-nodes)
- [Working with profiles](#working-with-profiles)
- [Manual overrides](#manual-overrides)
- [Viewing the schedule](#viewing-the-schedule)
- [Event history](#event-history)
- [JSON output for scripting](#json-output-for-scripting)
- [Global flags](#global-flags)

---

## Installation

<!-- GIF: terminal install walkthrough — download binary, move to /usr/local/bin, systemctl enable -->
![install-demo](./docs/gifs/install.gif)

```bash
# Download the latest release for your architecture
curl -LO https://github.com/pspiagicw/homelabctl/releases/latest/download/homelabctl_linux_arm64.tar.gz
tar -xzf homelabctl_linux_arm64.tar.gz

# Install the CLI and daemon
sudo mv homelabctl homelabctld /usr/local/bin/

# Copy the example config and edit it for your fleet
sudo mkdir -p /etc/homelabctl
sudo cp deploy/config/config.example.yaml /etc/homelabctl/config.yaml
sudo $EDITOR /etc/homelabctl/config.yaml

# Enable and start the daemon
sudo cp deploy/systemd/homelabctld.service /etc/systemd/system/
sudo systemctl enable --now homelabctld
```

---

## Quickstart

Once the daemon is running, everything goes through the `homelabctl` CLI:

```bash
$ homelabctl status
```

<!-- GIF: `homelabctl status` output, showing current profile, node states, sentinel state -->
![status-demo](./docs/gifs/status.gif)

```
Profile:   movie (active since 12:30, next: off @ 23:00)
Sentinel:  normal (falcon-lite reachable)
Override:  none

NODE          STATE   ADDRESS         LAST SEEN
falcon-heavy  on      192.168.1.20    2s ago
falcon1       off     192.168.1.21    -
falcon2       on      192.168.1.22    4s ago
```

---

## Checking status

```bash
# Full daemon + fleet snapshot
homelabctl status

# Include recent event history inline
homelabctl status --history
```

---

## Managing nodes

```bash
# List all configured nodes and their current state
homelabctl nodes list
```

<!-- GIF: `homelabctl nodes list` scrolling through node table -->
![nodes-list-demo](./docs/gifs/nodes-list.gif)

```
NAME          ADDRESS         STATE   SERVICES
falcon-heavy  192.168.1.20    on      jellyfin, immich, truenas
falcon1       192.168.1.21    off     proxmox-backup, borg
falcon2       192.168.1.22    on      transmission, navidrome, slskd
```

```bash
# Inspect a single node in detail
homelabctl nodes show falcon-heavy

# Manually boot or shut down a single node, outside of any profile
homelabctl boot falcon1
homelabctl shutdown falcon1
```

> Manual single-node boot/shutdown doesn't change the active profile or override — it's a direct, out-of-band action. The next scheduler tick or override change will reconcile the fleet back to whatever *should* be running.

---

## Working with profiles

```bash
# List all configured profiles
homelabctl profile list
```

```
NAME     NODES
backup   falcon1
movie    falcon-heavy, falcon2
off      (none)
```

```bash
# Show the boot order for a specific profile
homelabctl profile show movie
```

```
Profile: movie
Boot order:     falcon-heavy → falcon2
Shutdown order: falcon2 → falcon-heavy
```

---

## Manual overrides

<!-- GIF: setting an override, watching `status` reflect it, then clearing it -->
![override-demo](./docs/gifs/override.gif)

```bash
# Jump to the movie profile for 2 hours, then hand back to the schedule
homelabctl override movie --for 2h

# Jump to backup mode indefinitely (won't auto-expire)
homelabctl override backup --indefinite

# Default override duration (1h, per config) if no flag is given
homelabctl override movie

# Clear an active override early — reconciles immediately to the
# profile the schedule says should currently be active
homelabctl override clear
```

```
$ homelabctl override movie --for 2h
✓ Override set: movie (expires 14:45, in 2h)

$ homelabctl status
Profile:   movie (override, expires 14:45)
Sentinel:  normal
Override:  movie → expires in 1h 58m
```

---

## Viewing the schedule

```bash
homelabctl schedule show
```

```
Timezone: Asia/Kolkata

TIME    PROFILE
09:00   backup
12:30   movie
23:00   off

Next transition: 23:00 → off (in 6h 12m)
```

---

## Event history

```bash
# Last 20 events (boots, shutdowns, override changes, sentinel transitions)
homelabctl events

# Limit output
homelabctl events --limit 5
```

```
TIME      TYPE               DETAIL
12:30:01  profile_applied    backup → movie
12:30:02  node_boot          falcon-heavy
12:33:02  node_boot          falcon2
09:00:00  profile_applied    off → backup
```

---

## JSON output for scripting

Every command supports `--json` for machine-readable output — useful for dashboards, cron jobs, or feeding into other tools:

```bash
homelabctl status --json | jq '.sentinel.state'
homelabctl nodes list --json | jq '.[] | select(.state=="on") | .name'
```

```json
{
  "profile": "movie",
  "sentinel": { "state": "normal", "last_seen": "2026-09-08T12:34:56+05:30" },
  "override": null,
  "nodes": [
    { "name": "falcon-heavy", "state": "on", "address": "192.168.1.20" },
    { "name": "falcon1", "state": "off", "address": "192.168.1.21" },
    { "name": "falcon2", "state": "on", "address": "192.168.1.22" }
  ]
}
```

---

## Global flags

| Flag | Description |
|---|---|
| `--config <path>` | Path to config file (default: `/etc/homelabctl/config.yaml`) |
| `--api-addr <addr>` | Override the daemon API address (default: `127.0.0.1:PORT` from config) |
| `--json` | Emit machine-readable JSON instead of a formatted table |


