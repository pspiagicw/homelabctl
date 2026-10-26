# homelabctl — Usage Guide

How to use a running `homelabctld`: the web dashboard, the JSON API, and the config. For the overview and design, see [`README.md`](./README.md); for deploying to a Raspberry Pi, see [`deploy/README.md`](./deploy/README.md).

There is no CLI yet. Everything goes through the dashboard or the API.

---

## Table of Contents

- [Running the daemon](#running-the-daemon)
- [The dashboard](#the-dashboard)
- [Nodes](#nodes)
- [Profiles](#profiles)
- [The schedule](#the-schedule)
- [Services](#services)
- [During a power outage](#during-a-power-outage)
- [JSON API](#json-api)
- [Configuration](#configuration)

---

## Running the daemon

```bash
make build-daemon
./bin/homelabctld --config ./config.yaml
./bin/homelabctld --version
```

| Flag | Description |
|---|---|
| `--config <path>` | Config file. Default: `~/.config/homelabctl/config.yaml` |
| `--log-level` | `debug`, `info` (default), `warn`, `error` |
| `--log-format` | `text` (default) or `json` |
| `--version` | Print version, commit and build date, then exit |

The daemon listens on port **3000** on all interfaces, with no authentication. Keep it on a trusted network.

At startup the config is validated; every problem is printed and the daemon exits if anything is wrong. A missing SSH key is only a warning (booting doesn't need it, shutting down does).

> **Check the sentinel before (re)starting.** If the sentinel doesn't answer when the daemon starts, it goes straight to `OutageConfirmed` and shuts down every running managed node about 30 seconds later.

---

## The dashboard

Open `http://<host>:3000/ui/`. It refreshes itself every 5 seconds.

- **Header:** daemon version (hover for commit and build date) and when the page last synced.
- **Sentinel card:** `mains: up` / `down` / …, last seen, and whether boots are allowed.
- **Summary line:** node counts, **refresh** (re-ping every node now), **shut down all**.
- **Profiles card:** see [Profiles](#profiles).
- **Node cards:** see [Nodes](#nodes).
- **Services:** see [Services](#services).
- **Event history** (sidebar on wide screens, top on phones): the result of every action you take, with an `OK`/`ERR` tag and HTTP code. It's stored in your browser, so each device has its own history.

Times show as `2:33pm (12m ago)`; hover for the exact time.

---

## Nodes

Each node card shows its state (`on`, `off`, `booting`, `shutting`, `unknown`), address, when it last answered a ping, and **Boot** / **Shut down** buttons.

- **Boot** sends Wake-on-LAN. The node shows `booting`, then after `wait_after_boot` seconds it's pinged and shows `on` (or `off` if it didn't come up).
- **Shut down** asks for confirmation, then runs `shutdown now` over SSH. The node shows `shutting`, then `off` after `wait_after_shutdown` seconds.
- Both re-ping the node first, so a stale state on the card doesn't block them. Boot is greyed out for a node that's `on`, Shut down for one that's `off`, and both while it's booting or shutting down.
- **Unmanaged** nodes (marked with an `unmanaged` chip) work the same, but no automation touches them.
- **refresh** re-pings every node immediately; the daemon also does this every 15 minutes by itself.
- **shut down all** shuts down every managed node (unmanaged ones are skipped) and reports each node's result.

---

## Profiles

The profiles card has one button per profile. Hover or click to see exactly what it does, e.g. *"Boot falcon-heavy, falcon2 and shut down falcon1?"*.

Applying a profile boots its nodes in order, then shuts down every other managed node. Each node's result is shown on the card and in the event history:

| Result | Meaning |
|---|---|
| `booting` | Wake-on-LAN sent |
| `shutting` | shutdown sent |
| `skipped` | nothing to do (already on/off, or mid-transition); hover for the reason |
| `failed` | the boot or shutdown failed; hover for the error |

**Auto-off.** When you apply a profile by hand, the nodes it actually booted are shut down again **one hour later** (in reverse order). The card shows *"auto-off 3:45pm (in 59m) · shuts down …"*. Applying any other profile, by hand or on schedule, cancels the pending auto-off. Scheduled applies never start one.

Profile buttons are disabled while the sentinel isn't `Normal`.

---

## The schedule

The profiles card shows the next scheduled profile (*"next movie at 12:00 (in 2h 5m)"*) and the last scheduled run. A scheduled apply refused because of an outage shows as *refused*; it is not retried, and the next entry runs as usual.

The schedule only acts at its entry times. Missed entries aren't replayed and nothing is applied when the daemon starts.

---

## Services

Services from the config appear as tiles, grouped by `group`. Each tile links to the service and shows a health indicator, updated every minute:

| Indicator | Meaning |
|---|---|
| ● **up** | answered an HTTP request (any status below 500, including logins and redirects) |
| ● **down** | no answer, and its node is on or not set: a real problem |
| ○ **asleep** | no answer, but its `node` is off, so expected |
| ◌ **unchecked** | no check has finished yet |

Hover for when it was last checked. Set `node:` on a service so a planned shutdown shows as asleep rather than down.

---

## During a power outage

When the sentinel stops answering:

1. `mains: flickering?` (`OutageDetected`): boots and profile applies are refused immediately.
2. After `outage_grace_period`: `mains: down` (`OutageConfirmed`), and every managed node is shut down.
3. When the sentinel answers again: `mains: coming back` (`Restoring`). After `restore_grace_period` it's back to `Normal` and boots are allowed again.

Nodes are not booted automatically after power returns yet: apply a profile, or wait for the next schedule entry.

---

## JSON API

Every route returns JSON. Errors return a 4xx/5xx status and `{"status": false, "message": "…"}`.

| Method | Route | Does |
|---|---|---|
| `GET` | `/health` | `Hi!` if the daemon is up |
| `GET` | `/status` | sentinel state and all nodes |
| `GET` | `/nodes` | all nodes |
| `GET` | `/nodes/{name}` | one node, with MAC and boot timeout |
| `POST` | `/nodes/{name}/boot` | boot a node |
| `POST` | `/nodes/{name}/shutdown` | shut down a node |
| `POST` | `/nodes/refresh` | re-ping every node |
| `POST` | `/nodes/shutdown-all` | shut down every managed node; per-node results |
| `GET` | `/profiles` | profiles (with `nodes` and `shuts_down`), last applied, auto-off |
| `GET` | `/profiles/{name}` | one profile |
| `POST` | `/profiles/{name}/apply` | apply a profile; per-node results |
| `GET` | `/schedule` | entries, next run, last run |
| `GET` | `/services` | services and their latest check |

| Status | Meaning |
|---|---|
| 404 | unknown node or profile |
| 409 | refused: node already in that state, mid-transition, or sentinel not `Normal` |
| 502 | the boot or shutdown itself failed (WoL, SSH) |

```bash
curl -s localhost:3000/status | jq '.sentinel.state'
curl -s localhost:3000/schedule | jq '.next'
curl -s -X POST localhost:3000/profiles/movie/apply | jq '.nodes'
```

```json
{
  "profile": "movie",
  "trigger": "manual",
  "applied_at": "2026-10-23T12:00:00+05:30",
  "nodes": [
    { "node": "falcon-heavy", "result": "booting" },
    { "node": "falcon2", "result": "skipped", "message": "node online, no need of boot; node: falcon2; conflict" },
    { "node": "falcon1", "result": "shutting" }
  ]
}
```

---

## Configuration

A complete example with comments is in [`deploy/config/config.example.yaml`](./deploy/config/config.example.yaml). All durations are in seconds.

| Section | Keys |
|---|---|
| `ssh` | `user`, `identity_file` (absolute path), `port`, `connection_timeout`, `strict_host_key_checking`, `known_hosts_file` |
| `sentinel` | `host`, `address`, `check_interval`, `outage_grace_period`, `restore_grace_period` |
| `nodes.<name>` | `address`, `mac`, `wait_after_boot`, `wait_after_shutdown`, `unmanaged` |
| `services[]` | `name`, `description`, `url` (http/https), `icon`, `group`, `node` |
| `profiles.<name>` | `nodes` (boot order) |
| `schedule.entries[]` | `time` (`HH:MM`, local time), `profile` |

Validation rules, checked at startup:

- `ssh`: a user, a port between 1 and 65535, an identity file or password, and `known_hosts_file` when strict checking is on.
- `sentinel`: a host and valid address, positive interval and grace periods, and not also listed as a node (by name or address).
- Nodes: a valid IP or hostname, a valid MAC, no duplicate addresses or MACs, and wait times above zero.
- Services: unique names, http(s) URLs, and a known `node` if set.
- Profiles: only known, managed nodes, each listed once.
- Schedule: valid, unique times that name existing profiles.

Host key checking isn't enforced yet: the daemon accepts any SSH host key, whatever `strict_host_key_checking` says.
