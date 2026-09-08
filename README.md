 # homelabctl

A single-binary CLI + daemon for orchestrating a home lab that runs on battery/inverter backup — power-outage safe, schedule-driven, and manually overridable.

`homelabctl` treats your home lab like a small fleet: it knows what should be running at any given moment, boots and shuts down nodes in the right order, and — critically — knows the difference between "the schedule says off" and "the power is out," reacting very differently to each.

---

## Why this exists

Most home lab automation assumes the power is always on. Mine isn't guaranteed to be — nodes run off an inverter/battery backup, and when mains power drops, I'd rather have everything shut down cleanly than have a battery die mid-write on a NAS. At the same time, I don't want to run every node 24/7 just to be safe, and I don't want to SSH into three machines by hand every time I want to switch from "backup mode" to "movie night."

`homelabctl` solves three problems at once:

1. **Power-outage safety** — a dedicated always-on sentinel node watches mains power as a proxy and shuts the fleet down gracefully before the battery runs out, then wakes everything back up once power is restored.
2. **Scheduled power profiles** — named profiles (`backup`, `movie`, `off`, ...) define which nodes should be running and in what order, on a daily schedule.
3. **Manual overrides** — jump to a profile on demand ("turn on the media stack right now") without fighting the schedule, with automatic expiry.

It's designed to be boring, predictable infrastructure software: explicit state machines, clear precedence rules, and no magic.

---

## How it works

```
┌──────────────────────────────────────────────────────────┐
│                      homelabctld                         │
│                    (daemon, always-on)                   │
│                                                          │
│   ┌───────────┐   ┌────────────┐   ┌───────────────────┐ │
│   │ Sentinel  │   │ Scheduler  │   │ Override Manager  │ │
│   │  (power)  │   │  (profiles)│   │   (manual)        │ │
│   └─────┬─────┘   └──────┬─────┘   └─────────┬─────────┘ │
│         └────────────────┼───────────────────┘           │
│                    Orchestrator                          │
│              (single source of truth)                    │
│                          │                               │
│                 REST / WebSocket API                     │
└─────────────────────────┬────────────────────────────────┘
                           │
              ┌────────────┴────────────┐
              │                         │
         homelabctl CLI           (future) Web UI
        (thin API client)          (htmx-based)
```

**Precedence, highest to lowest: Sentinel > Override > Schedule.**

A power outage always wins — it suspends whatever the schedule or an active override says and takes the fleet down. A manual override beats the schedule but yields to a real outage. The schedule runs everything else, day to day.

### The sentinel

A lightweight, mains-powered Raspberry Pi (`falcon-lite`) does nothing but watch for the lights going out. It's not a managed node itself — it's a power-proxy. `homelabctld` pings it continuously:

- Ping fails → after a grace period (default 15 min, to ignore blips) → shut down all managed nodes, in reverse boot order.
- Sentinel comes back → after another grace period → wake nodes back up, resuming whatever profile *should* be active for the current time.

### The scheduler

Profiles are just an ordered list of nodes:

```yaml
profiles:
  backup:
    nodes: [falcon1]
  movie:
    nodes: [falcon-heavy, falcon2]
  off:
    nodes: []
```

List order **is** boot order. Shutdown is strictly the reverse. The daily schedule maps times to profiles:

```yaml
schedule:
  entries:
    - time: "09:00"
      profile: backup
    - time: "12:30"
      profile: movie
    - time: "23:00"
      profile: off
```

### Manual overrides

```
homelabctl override movie --for 2h
homelabctl override backup --indefinite
homelabctl override clear
```

Overrides default to a 1-hour expiry unless `--for` or `--indefinite` is given. When they expire (or are cleared), control hands back to the scheduler, which reconciles to whatever profile should currently be active — it never blindly replays missed transitions.

---

## Architecture notes

- **API-first daemon.** `homelabctld` exposes a REST/WebSocket API from day one; the CLI is a thin client of that API, not a separate code path. This means a future web UI is just another client — no daemon rework required.
- **Explicit state machines**, not implicit behavior. Sentinel states, override lifecycle, and profile transitions are all modeled as first-class types, not scattered booleans.
- **Static waits now, health-based readiness later.** Nodes currently wait a fixed `wait_after_boot` duration before being considered "up." A `Prober` interface is stubbed from the start so this can be swapped for real polling-based readiness checks (e.g., hitting a service port) without touching any calling code.
- **Local-only API for v1.** The API binds to `127.0.0.1` and has no auth in the initial version — it's not exposed off the box until the web UI phase needs it, at which point auth gets added deliberately rather than bolted on.

---

## Tech stack

| Concern | Choice |
|---|---|
| Language | Go |
| CLI framework | [cobra](https://github.com/spf13/cobra) |
| Config format | YAML |
| SSH | `golang.org/x/crypto/ssh` |
| Wake-on-LAN | UDP magic packets |
| Health checks | ICMP ping (v1), pollable service checks (planned) |
| HTTP routing | [chi](https://github.com/go-chi/chi) |
| Realtime updates | WebSocket ([gorilla/websocket](https://github.com/gorilla/websocket)) |
| Process management | systemd |
| Planned web UI | htmx + server-rendered Go templates |

---

## Project layout

```
cmd/homelabctl/     CLI entrypoint
cmd/homelabctld/    Daemon entrypoint
internal/config/    Config parsing + validation
internal/node/      Node model, SSH, WoL, health checks
internal/sentinel/  Power-outage watchdog state machine
internal/scheduler/ Profile scheduler
internal/override/  Manual override state machine
internal/daemon/    Orchestrator + event log
internal/api/       REST/WebSocket API server
internal/apiclient/ Go client used by the CLI
internal/cli/       Cobra command definitions
deploy/systemd/     systemd unit files
deploy/config/      Example config
```

---

## Status

Actively in development. Design is finalized end-to-end (config schema, sentinel behavior, scheduling semantics, override precedence, API surface, CLI commands); implementation is proceeding phase by phase from config parsing through daemon deployment. See [`homelabctl-development-track.md`](./homelabctl-development-track.md) for the full build plan.

**Not yet built:**
- The physical `falcon-control` host this will run on
- Web UI
- Color output in CLI tables
- Polling-based node readiness (health-check probing instead of fixed waits)

---

## Example config

```yaml
sentinel:
  host: falcon-lite
  address: 192.168.1.10
  check_method: icmp
  check_interval: 30s
  outage_grace_period: 900s
  restore_grace_period: 900s

nodes:
  falcon-heavy:
    address: 192.168.1.20
    mac: "AA:BB:CC:DD:EE:01"
    wait_after_boot: 180s
    services: [jellyfin, immich, truenas]
  falcon1:
    address: 192.168.1.21
    mac: "AA:BB:CC:DD:EE:02"
    services: [proxmox-backup, borg]
  falcon2:
    address: 192.168.1.22
    mac: "AA:BB:CC:DD:EE:03"
    services: [transmission, navidrome, slskd]

profiles:
  backup:
    nodes: [falcon1]
  movie:
    nodes: [falcon-heavy, falcon2]
  off:
    nodes: []

schedule:
  timezone: "Asia/Kolkata"
  entries:
    - time: "09:00"
      profile: backup
    - time: "12:30"
      profile: movie
    - time: "23:00"
      profile: off

overrides:
  default_duration: 1h
```

---

## Why not just [existing tool]?

Most existing power/homelab management tools (Home Assistant automations, generic Wake-on-LAN schedulers, UPS shutdown scripts) handle one piece of this but not the combination: outage-aware shutdown *and* scheduled profiles *and* ad-hoc overrides, with a clean precedence model between them, and a single daemon that a future UI can sit on top of without a rewrite. `homelabctl` is purpose-built for a specific home lab, but the pattern — sentinel + scheduler + override, arbitrated by one orchestrator — is general enough to be useful anywhere nodes share a battery budget.

---

## License

TBD.
