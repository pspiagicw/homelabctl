# homelabctl

A small daemon with a web dashboard for running a home lab on battery/inverter backup: power-outage safe, schedule-driven, and controllable by hand.

`homelabctl` treats the home lab like a small fleet. It knows what should be running at any moment, boots and shuts down nodes in the right order, and knows the difference between "the schedule says off" and "the power is out", reacting very differently to each.

---

## Why this exists

Most home lab automation assumes the power is always on. Mine isn't guaranteed to be: the nodes run off an inverter/battery backup, and when mains power drops I'd rather have everything shut down cleanly than have a battery die mid-write on a NAS. At the same time, I don't want to run every node 24/7 just to be safe, and I don't want to SSH into three machines by hand every time I switch from "backup mode" to "movie night".

`homelabctl` handles three things:

1. **Power-outage safety.** An always-on sentinel host on mains power acts as a power probe. When it stops answering for long enough, every managed node is shut down cleanly before the battery runs out.
2. **Profiles on a schedule.** Named profiles (`backup`, `movie`, `off`, …) say which nodes should run. Applying one boots its nodes and shuts down the rest. A daily schedule applies profiles at fixed times.
3. **Manual control.** A web dashboard to apply a profile, boot or shut down a single node, or shut everything down. A profile applied by hand switches itself off again after an hour.

It's meant to be boring, predictable infrastructure software: explicit state machines, clear rules about who wins, and no magic.

---

## How it works

```
┌─────────────────────────────── homelabctld (runs on a Raspberry Pi) ──────────────────────────────┐
│                                                                                                   │
│   Sentinel          Scheduler         Profiles               Node registry      Catalog           │
│   pings the power   applies profiles  boot listed nodes,     node state, WoL,   service HTTP      │
│   probe every 30s   at HH:MM          shut down the rest     SSH shutdown,      checks every 1m   │
│        │                 │            (+ 1h auto-off for     15-min refresh                       │
│        │                 │             manual applies)            ▲                               │
│        └─────────────────┴──────────────► Orchestrator ───────────┘                               │
│                                                │                                                  │
│                         HTTP server on :3000:  /ui (web dashboard)  +  JSON API                   │
└───────────────────────────────────────────────────────────────────────────────────────────────────┘
```

**The sentinel always wins.** While it isn't `Normal`, nothing boots: single-node boots and profile applies (manual or scheduled) are refused with a 409. Shutting down is always allowed.

### The sentinel

A host on mains power (here `falcon-lite`), never a managed node itself, acts as the power probe. The daemon pings it every `check_interval` seconds:

- No answer → `OutageDetected`. Still no answer after `outage_grace_period` → `OutageConfirmed`, and every managed node is shut down.
- Answering again → `Restoring`. Still answering after `restore_grace_period` → back to `Normal`, and boots are allowed again. Nodes aren't booted automatically yet (see [Status](#status)).

### Profiles

A profile is a list of nodes, in boot order:

```yaml
profiles:
  backup:
    nodes: [falcon1, falcon-heavy]
  movie:
    nodes: [falcon-heavy, falcon2]
  all:
    nodes: [falcon1, falcon-heavy, falcon2]
  off:
    nodes: []
```

Applying a profile:

1. boots its nodes, in order (a node that's already on is reported as `skipped`)
2. shuts down every other managed node, in reverse boot order of the previously applied profile
3. records a result per node (`booting`, `shutting`, `skipped` or `failed`)

So `off` shuts everything down. Applying a profile **by hand** also arms an **auto-off**: an hour later, the nodes that apply booted are shut down again. Any later apply cancels it.

### The schedule

```yaml
schedule:
  entries:
    - time: "08:45"
      profile: backup
    - time: "10:30"
      profile: off
    - time: "12:00"
      profile: movie
    - time: "23:00"
      profile: off
```

Times are 24h `HH:MM` in the host's local time. The scheduler only acts at those times: it never replays missed entries and applies nothing at startup, so restarting the daemon never boots machines by itself.

### Unmanaged nodes

A node with `unmanaged: true` (e.g. a desktop) can be booted and shut down from the dashboard, but nothing automatic touches it: it's skipped by the outage shutdown, can boot during an outage, and can't be in a profile.

### Services

The dashboard also lists your services (Homer-style tiles grouped by `group`). Each one is checked over HTTP every minute. A tile shows **up**, **down**, **asleep** (not answering because its `node` is off, so expected) or **unchecked**.

---

## Web dashboard

Served by the daemon at `http://<host>:3000/ui/`:

- **Sentinel card:** mains state, last seen, whether boots are allowed.
- **Profiles card:** one button per profile (with a confirmation that lists what it boots and shuts down), the last applied profile with per-node results, the pending auto-off, and the schedule's next and last run.
- **Node cards:** state, address, last seen, Boot / Shut down buttons; plus **refresh** (re-ping every node now) and **shut down all** (every managed node).
- **Services:** grouped tiles with a health indicator.
- **Event history:** results of your actions, kept in the browser.

It refreshes every 5 seconds, works on phones, and follows the light/dark setting. The visual style is documented in [`web/DESIGN.md`](./web/DESIGN.md).

---

## Architecture notes

- **One binary, one process.** The daemon serves the web UI and a JSON API on `:3000` (all interfaces, no auth: it's meant for a trusted LAN or Tailscale). The UI calls the daemon in-process; the JSON API is for scripts and `curl`.
- **Explicit state machines.** Node power state (`unknown`, `off`, `booting`, `on`, `shutting`) and sentinel state are modelled as named states, not scattered booleans.
- **Fresh state before acting.** Boot and shutdown re-ping the node first, and every node is re-pinged every 15 minutes, so state set outside the daemon doesn't stay stale.
- **Static waits for readiness.** After Wake-on-LAN or an SSH shutdown, a node is checked again after a fixed `wait_after_boot` / `wait_after_shutdown`.
- **Validated config.** Every section is checked at startup and all problems are reported at once; a bad config fails before anything runs.

---

## Tech stack

| Concern | Choice |
|---|---|
| Language | Go |
| Daemon flags | [kong](https://github.com/alecthomas/kong) |
| Config | YAML ([yaml.v3](https://gopkg.in/yaml.v3)) |
| HTTP routing | [chi](https://github.com/go-chi/chi) |
| Web UI | Go templates + [htmx](https://htmx.org), embedded in the binary |
| SSH shutdown | `golang.org/x/crypto/ssh` |
| Wake-on-LAN | [gowol](https://github.com/linde12/gowol) (UDP magic packets) |
| Node health | ICMP ping ([pro-bing](https://github.com/prometheus-community/pro-bing)) |
| Service health | HTTP GET |
| Logging | `log/slog` with [tint](https://github.com/lmittmann/tint) |
| Process management | systemd user unit |

---

## Project layout

```
cmd/homelabctld/   Daemon entrypoint
cmd/homelabctl/    CLI entrypoint (placeholder; the CLI isn't built)
daemon/            Orchestrator: wires everything together, enforces the sentinel rule
config/            Config types, loading, validation
node/              Node model and state machine, registry, refresh loop
sentinel/          Power-outage watchdog state machine
profile/           Profiles: apply (boot + shut down the rest), auto-off
schedule/          Daily scheduler
catalog/           Services and their HTTP health checks
api/               HTTP server and JSON API
web/               Web dashboard (templates, CSS, JS, htmx)
backend/           Interfaces the components use to call the orchestrator
apierr/            Error kinds mapped to HTTP status codes
agents/            Function types injected for testing (ping, WoL, SSH, timers)
utils/             Ping, Wake-on-LAN, SSH, HTTP check, response helpers
logging/           slog setup
version/           Build version info
deploy/            Example config, systemd unit, Pi deployment guide
```

---

## Running it

```bash
make build-daemon
./bin/homelabctld --config deploy/config/config.example.yaml   # then open http://localhost:3000/ui/
```

Copy [`deploy/config/config.example.yaml`](./deploy/config/config.example.yaml) and fill in your own hosts. Deploying to a Raspberry Pi (as a user systemd service) is described in [`deploy/README.md`](./deploy/README.md); [`USAGE.md`](./USAGE.md) covers day-to-day use.

---

## Status

Running in production on a Raspberry Pi 5 (`falcon-control`) since v0.3.0.

**Built:** sentinel watchdog, node boot/shutdown/refresh, unmanaged nodes, profiles with switching, manual-apply auto-off, daily scheduler, service catalog with health checks, web dashboard, JSON API, config validation, Pi deployment.

**Not built yet:**
- Booting nodes again after power comes back (the restore step only logs).
- A daemon start during an outage skips the outage grace period: check the sentinel before restarting.
- A server-side event log (the history is per-browser today).
- The CLI (`homelabctl`).
- Per-node stats (CPU/RAM/disk) through a small agent on each node.

The full checklist is in [`TRACK.md`](./TRACK.md).

---

## License

TBD.
