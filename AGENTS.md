# Agents Guide for homelabctl

Essential context and patterns for agents (LLMs, bots, or developers) working in this repository. User-facing docs: [`README.md`](./README.md) (overview), [`USAGE.md`](./USAGE.md) (dashboard, API, config), [`deploy/README.md`](./deploy/README.md) (Raspberry Pi).

## Project Overview

**homelabctl** is a Go daemon (`homelabctld`) with an embedded web dashboard that runs a home lab on battery/inverter backup:

- a **sentinel** watches a mains-powered host and shuts down every managed node during a power outage
- **profiles** (named node sets) are applied by hand or by a daily **schedule**: applying one boots its nodes and shuts down every other managed node
- a manual apply arms a 1-hour **auto-off** for the nodes it booted
- a **catalog** of services is health-checked over HTTP and shown on the dashboard

It runs in production on a Raspberry Pi 5 (`falcon-control`) as a user systemd service. The CLI (`homelabctl`) is not built.

**The sentinel always wins:** while it isn't `Normal`, managed nodes can't boot and profiles (manual or scheduled) are refused with 409. Shutdowns are always allowed.

## Essential Commands

```bash
make build-daemon && ./bin/homelabctld --config ./config.yaml   # run locally; UI at http://localhost:3000/ui/
./bin/homelabctld --version

make test          # go test -race -coverprofile ./... (the cli package doesn't compile, see Gotchas)
make test-short
go test -race ./api/ ./catalog/ ./config/ ./daemon/ ./node/ ./profile/ ./schedule/ ./sentinel/ ./web/

make vet / make fmt / make lint / make check

# Raspberry Pi 5 (falcon-control) deployment, see deploy/README.md
make build-pi        # static arm64 daemon binary in dist/
make deploy          # build, push to ~/.local/bin on the Pi, restart the user service
make deploy-config   # copy deploy/config.yaml (gitignored, the real config) to ~/.config/homelabctl/
```

**Before any restart or deploy, check the sentinel is `Normal`** (`curl -s <host>:3000/status | jq .sentinel.state`). A daemon that starts while the sentinel is unreachable shuts down every running managed node about 30 seconds later.

## Project Structure

```
cmd/homelabctld/   Daemon entrypoint (calls daemon.Run)
cmd/homelabctl/    CLI entrypoint: placeholder ("Hello, world!")
cli/               Kong CLI definition: does NOT compile (missing *CMD types)

daemon/
  argparse.go      kong flags (--config, --log-level, --log-format, --version), Run()
  orchestrator.go  Wires everything; implements backend.API, backend.Power and web.Backend;
                   owns the sentinel rule (Boot guard, applyProfile)
  status.go        Response and DaemonStatus JSON types
config/
  types.go         Config, SSHConfig, SentinelConfig, NodeConfig, ServiceConfig, ProfileConfig, ScheduleConfig
  load.go          New(path): resolve path (default ~/.config/homelabctl/config.yaml), read, Validate, Fatal on error
  validate.go      Validate(): every section, errors joined; missing SSH key is a warning
node/
  node.go          Node and its state machine; Boot/Shutdown re-ping first (refresh)
  registry.go      Registry: Init, Boot, Shutdown, ShutdownAll (skips unmanaged), RefreshAll, Watch (15 min)
sentinel/
  sentinel.go      Sentinel state, Init, Ping, Status
  watch.go         Watch: the outage/restore state machine; calls backend.Power
profile/
  profile.go       ProfileManager: ApplyProfile (boot listed nodes, shut down the rest), ListProfiles, GetProfile
  autooff.go       1-hour auto-off after a manual apply (time.AfterFunc + generation counter)
schedule/
  schedule.go      Scheduler: Watch applies entries at HH:MM via Applier; Status
catalog/
  catalog.go       Services from config + HTTP health checks (Init, Refresh, Watch every minute)
api/
  api.go           chi router: JSON API, mounts the web UI at /ui, "/" redirects there; listens on :3000
web/
  web.go           Dashboard handlers and view model; web.Backend interface
  templates/       index.html (page), nodes.html (the polled "panel")
  static/          style.css, ui.js, htmx.min.js, fonts: Fira Code (body), Cascadia Code
                   (headings), each with its OFL licence (embedded with go:embed)
  DESIGN.md        Visual design system
backend/           Interfaces: API (for the JSON server) and Power (for the sentinel)
apierr/            ErrNotFound / ErrConflict / ErrUnavailable / ErrUpstream → 404 / 409 / 503 / 502
agents/            Function types for injection: Pinger, Runner, WakeSender, After
utils/             Ping (pro-bing), Wake (gowol), SSHRun, HTTPCheck, After, ParseErrCode, ToJSON
logging/           slog setup (tint), WithComponent
version/           Version/Commit/BuildDate set by ldflags (see makefile)
deploy/
  config/config.example.yaml   Example config (generic hosts)
  config.yaml                  The real config: gitignored, never commit
  systemd/homelabctld.service  User unit
  README.md                    Pi deployment runbook
```

## Configuration Format

Example: [`deploy/config/config.example.yaml`](./deploy/config/config.example.yaml). All durations are integer seconds.

```yaml
ssh:
  user: root
  identity_file: /home/pspiagicw/.config/homelabctl/keys/id_ed25519   # absolute; ~ isn't expanded
  port: 22
  connection_timeout: 10
  strict_host_key_checking: true    # validated, but not enforced (InsecureIgnoreHostKey)
  known_hosts_file: /home/pspiagicw/.config/homelabctl/keys/known_hosts

sentinel:                 # power probe on mains, never a node
  host: falcon-lite
  address: 172.20.1.46
  check_interval: 30
  outage_grace_period: 900
  restore_grace_period: 900

nodes:
  falcon-heavy:
    address: 172.20.1.45
    mac: "fc:9d:05:25:5f:13"
    wait_after_boot: 180       # seconds before the post-boot ping
    wait_after_shutdown: 60    # seconds before the post-shutdown ping
  starship:
    address: 172.20.1.56
    mac: "34:5a:60:66:58:87"
    wait_after_boot: 90
    wait_after_shutdown: 60
    unmanaged: true            # manual only: no outage shutdown, no boot block, not in profiles

services:                      # dashboard tiles, in order
  - name: Jellyfin
    description: Media Server
    url: http://172.20.1.91:8096   # http(s); health-checked every minute
    icon: https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons/svg/jellyfin.svg
    group: Media
    node: falcon-heavy             # optional; lets the UI show "asleep" when the node is off

profiles:                      # list order = boot order
  backup:
    nodes: [falcon1, falcon-heavy]
  off:
    nodes: []                  # shuts down every managed node

schedule:
  entries:                     # HH:MM, host local time
    - time: "08:45"
      profile: backup
    - time: "23:00"
      profile: off
```

**Validation** (`config/validate.go`, run at startup, all errors joined):
- **ssh:** a user, a port between 1 and 65535, a timeout ≥ 0, an identity file or password, and `known_hosts_file` when strict checking is on.
- **sentinel:** a host and valid address, positive interval and grace periods, and not also a node (by name or address).
- **nodes:** a valid IP or hostname (all-numeric names must be valid IPs), a valid MAC, no duplicate addresses or MACs, and positive waits.
- **services:** names unique and required, an http(s) URL, and a known node if set.
- **profiles:** only known nodes, no unmanaged nodes, and no duplicates.
- **schedule:** valid unique `HH:MM` times that name existing profiles.

## Code Patterns & Architecture

### One process, two front doors
`api.Server` serves both the JSON API (`backend.API`, methods return `(any, error)`, encoded by `WriteResponse`) and the web UI mounted at `/ui`. The UI does **not** call the JSON API: `web.Backend` is implemented by the orchestrator in-process, with typed methods (`NodeStates()`, `ProfileStatus()`, `Services()`, `ScheduleStatus()`, …). New features usually need both a JSON method and a typed UI method.

### Errors → HTTP codes
Wrap errors with `apierr` kinds via `%w`; `utils.ParseErrCode` maps them (404/409/503/502, else 500). The UI's handlers use the same mapping, and `ui.js` records the code in the event history.

### Node state machine
States: `unknown`, `off`, `booting`, `on`, `shutting` (`unreachable` exists but is never set). State only changes from a ping (`IsReachable`), or when Boot/Shutdown start a transition.
- **Boot:** re-ping (refresh) → must be `off` → WoL → `booting` → after `wait_after_boot`, ping → `on`/`off`.
- **Shutdown:** re-ping → must be `on` → SSH `shutdown now` → `shutting` → after `wait_after_shutdown`, ping → `off`/`on`.
- Wrong state → `apierr.ErrConflict` (409). `refresh` skips `booting`/`shutting`, so the timers stay in charge.
- `Registry.Watch` re-pings every node every 15 minutes (`nodeRefreshInterval`, a var so tests can shorten it).

### Sentinel
`Normal → OutageDetected → OutageConfirmed → Restoring → Restored → Normal`, one step per `check_interval`.
- In `OutageConfirmed` it calls `Power.ShutdownAll` **on every tick** (refused for nodes already off).
- `Power.RestoreAll` currently only logs.
- `Init` sets `Normal` or, if the first ping fails, **`OutageConfirmed` directly** (no grace period).

### Profiles
`ProfileManager.ApplyProfile(ctx, name, trigger)`:
- boots the profile's nodes in order, then shuts down `ShutsDown` (managed nodes not in the profile; computed once in `NewProfileManager`). Shutdowns go in reverse boot order of the previously applied profile, then by name.
- records a `NodeResult` per node: `booting`, `shutting`, `skipped` (409 conflict) or `failed`. A node that fails doesn't stop the others.
- **cancels any pending auto-off**; with trigger `profile.Manual` it schedules a new one for the nodes it booted.
- **Locks:** `applyMu` serialises whole applies (held for seconds); `mu` guards `last`/`autoOff`/`lastAutoOff`/`timer`/`autoOffGen` and is held only briefly. Order is always `applyMu` → `mu`. `ListProfiles` returns deep copies.
- **Auto-off** (`autooff.go`) uses `time.AfterFunc`. Each schedule or cancel bumps `autoOffGen`, so a timer that already fired can't act on a newer state. `runAutoOff` does **not** take `applyMu`.
- **The sentinel check** lives in `Orchestrator.applyProfile`, shared by manual (`ApplyProfile`) and scheduled (`ApplyScheduledProfile`) applies.

### Scheduler
`schedule.Scheduler.Watch` loops: compute `nextAfter(from)` (the first entry strictly after `from`, wrapping to tomorrow), wait, call `Applier.ApplyScheduledProfile`, record `LastRun`, continue **from the entry's time** (not the clock).
- No replay of missed entries, no apply at startup, no retry of a refused apply.
- `now`/`after` are injected for tests (fake clock in `schedule_test.go`).

### Catalog
`catalog.Catalog` checks every service with `utils.HTTPCheck` (GET, 3s timeout, no redirects, self-signed certs accepted, keep-alives off) at startup and every minute.
- **Up** means any status < 500.
- Per-service results live under a mutex; `List()` returns copies (`catalog.Status`).
- Logs at info only on the first result and on up/down changes.
- The UI's `health()` turns a status plus the node's state into `up` / `down` / `asleep` / `unchecked`.

### Orchestrator goroutines
`Init()` runs registry, sentinel, server and catalog init in parallel. `Start()` runs five long-lived goroutines: the HTTP server, `Sentinel.Watch`, `Registry.Watch`, `Catalog.Watch` and `Scheduler.Watch`.

### Dependency injection
Time, ping, WoL and SSH are function fields (`agents.*` types, `CheckFunc`, scheduler `now`/`after`) so tests never touch the network or wait. Package-level intervals (`nodeRefreshInterval`, `autoOffAfter`) are vars for the same reason.

## Testing Approach

- Tests live next to the code (`*_test.go`), mostly small focused tests with a `setup…` helper, plus table-driven ones (validation, health, scheduler).
- Fakes implement the small interfaces: `fakeBackend` for `backend.API` / `web.Backend`, `fakePower`, a fake `Applier`, a fake clock.
- Run with `-race`; there are dedicated tests that read while a refresh or apply writes.
- Not yet covered: `Sentinel.Watch` and `Catalog.Watch` loops, the CLI.

## Gotchas & Non-Obvious Patterns

1. **Restarting during an outage skips the grace period.** `Sentinel.Init` goes straight to `OutageConfirmed` if the first ping fails, and the next tick shuts down every running managed node. Always check the sentinel before a deploy or restart.
2. **Nothing boots after power returns.** `Orchestrator.RestoreAll` only logs; nodes stay off until a profile is applied or the next schedule entry.
3. **Node state isn't locked.** `Node.State`/`LastSeen` are written by HTTP handlers, timers, the refresher, the scheduler and the auto-off without a mutex.
4. **SSH host keys aren't checked.** `utils.NewSSHClient` uses `ssh.InsecureIgnoreHostKey()` whatever the config says.
5. **The API listens on `:3000` on all interfaces, with no auth.** It's meant for a trusted LAN/Tailscale.
6. **`cli/` doesn't compile**, so `go build ./...` and `make test` report it as a failure. `make build`/`build-cli` still work, because `cmd/homelabctl` doesn't import `cli`.
7. **Shutdown runs `shutdown now` as the SSH user** (`root` on the managed nodes). starship is unmanaged partly because shutting it down needs sudo.
8. **Docker macvlan containers on the Pi can't be reached from the Pi itself** (Linux macvlan isolation), so services hosted that way always look down from the daemon. A host shim interface fixes it.
9. **ICMP uses unprivileged sockets** (`pro-bing` UDP mode), so `net.ipv4.ping_group_range` must include the daemon's group. Debian 13 allows all groups by default.
10. **The config's `identity_file` must be absolute;** `~` isn't expanded.

## Coding Guidelines

### Edit in small steps
When refactoring or adding code, make multiple small edits rather than large replacements:

1. Add the function signature as one edit
2. Add the function body as another edit (or split further into helper functions)
3. Continue splitting until each change is under 10-15 lines

This reduces whitespace-matching failures during find/replace and makes reviews easier.

### Avoid changing entire files at once
To replace a whole function or struct definition, read the file for its exact boundaries first, and use `lsp_replace_symbol` instead of `edit` where the LSP supports it (Go does).

### File organization patterns
- Keep config-related code in `config/`.
- Avoid cross-package dependency loops: components call the orchestrator through small interfaces (`backend.Power`, `schedule.Applier`, `web.Backend`).
- Use `log/slog` consistently, with structured fields.
- Shared mutable state gets a mutex, and readers get copies (see `catalog.List`, `ProfileManager.ListProfiles`).

### Testing while developing
After each small edit: `make fmt vet`, then the package's tests with `-race`, then check LSP diagnostics before committing.

### Commits
Messages follow the existing style: `type: summary`, then a `-` bullet list prefixed by package (`- profile: …`). No co-author trailers.

## Future Work (Not Yet Implemented)

- Boot the scheduled profile again after power is restored (`RestoreAll`).
- Start the sentinel in `OutageDetected` rather than `OutageConfirmed`, so a restart gets the grace period.
- A mutex on `Node`; `runAutoOff` taking `applyMu`.
- Enforce SSH host keys (`known_hosts_file`).
- Graceful shutdown: signal handling, and `Server.Run` honouring `ctx`.
- Daemon-side event log (JSONL), replacing the browser-only history.
- Per-node stats agent (CPU/RAM/disk/temp) for the Proxmox nodes.
- The CLI.

## Related Files

- [`README.md`](./README.md): overview and architecture
- [`USAGE.md`](./USAGE.md): dashboard, API and config reference
- [`TRACK.md`](./TRACK.md): development checklist
- [`deploy/README.md`](./deploy/README.md): Raspberry Pi deployment
- [`web/DESIGN.md`](./web/DESIGN.md): UI design system
- [`deploy/config/config.example.yaml`](./deploy/config/config.example.yaml): example configuration
