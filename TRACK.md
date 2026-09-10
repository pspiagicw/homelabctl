# homelabctl — Development Track

Granular, code-level breakdown of everything needed to build `homelabctl` from scratch through to daemon deployment on falcon-control. Each phase produces working, testable code before moving to the next. Checkboxes are meant to be checked off as you go.

---

## Phase 0 — Project Scaffolding

- [x] Init Go module: `go mod init github.com/<you>/homelabctl`
- [x] Create directory skeleton:
  ```
  cmd/homelabctl/        # CLI entrypoint (main.go)
  cmd/homelabctld/        # daemon entrypoint (main.go)
  internal/config/        # config parsing + validation
  internal/node/           # node model, SSH, WoL, health checks
  internal/sentinel/      # power-outage sentinel logic
  internal/scheduler/      # profile scheduler
  internal/override/       # manual override state machine
  internal/daemon/         # daemon core: orchestrator, state store
  internal/api/            # REST/WebSocket API server
  internal/apiclient/      # Go client used by CLI to talk to daemon
  internal/cli/             # cobra command definitions
  internal/logging/         # structured logger setup
  pkg/version/               # build version info
  deploy/systemd/            # unit files
  deploy/config/               # example config.yaml
  test/integration/            # integration tests
  ```
- [x] Add `.gitignore` (binaries, `*.log`, local config overrides)
- [ ] Pick and vendor core deps:
  - [ ] `gopkg.in/yaml.v3` — config parsing
  - [ ] `github.com/spf13/cobra` — CLI framework
  - [ ] `golang.org/x/crypto/ssh` — SSH client (shutdown commands)
  - [ ] `github.com/mdlayher/wol` (or hand-rolled UDP magic packet) — WoL
  - [ ] `github.com/prometheus-community/pro-bing` (or raw ICMP) — ping
  - [ ] `github.com/gorilla/websocket` — WS for API
  - [ ] `github.com/go-chi/chi/v5` — HTTP routing for REST API
  - [ ] `go.uber.org/zap` (or `log/slog`) — structured logging
- [ ] Set up `Makefile` with targets: `build`, `build-cli`, `build-daemon`, `test`, `lint`, `vet`
- [ ] Set up CI skeleton (GitHub Actions: build + vet + test on push)

---

## Phase 1 — Config Layer

### 1.1 Struct definitions (`internal/config/types.go`)
- [x] `type SSHConfig struct` — User, IdentityFile, Port, ConnectTimeout, StrictHostKeyChecking, KnownHostsFile
- [x] `type SentinelConfig struct` — Host, Address, CheckMethod, CheckInterval, OutageGracePeriod, RestoreGracePeriod
- [x] `type NodeConfig struct` — Address, MAC, AlwaysOn, WaitAfterBoot, Services []string, SSH *SSHConfig (nil = inherit global)
- [x] `type ProfileConfig struct` — Nodes []string (ordered)
- [x] `type ScheduleEntry struct` — Time string, Profile string
- [x] `type ScheduleConfig struct` — Timezone string, Entries []ScheduleEntry
- [ ] `type OverridesConfig struct` — DefaultDuration time.Duration
- [x] `type Config struct` — top-level: SSH, Sentinel, Nodes map[string]NodeConfig, Profiles map[string]ProfileConfig, Schedule, Overrides
- [ ] Custom `UnmarshalYAML` for duration fields (`900s`, `1h`) → `time.Duration`
- [ ] Custom `UnmarshalYAML` for `ScheduleEntry.Time` → validate `HH:MM` 24h format

### 1.2 Loading (`internal/config/load.go`)
- [x] `func Load(path string) (*Config, error)` — reads file, unmarshal YAML

These 2 functions are part of command parsing, not loading. Loading should only take care of only loading.

- [ ] Support `--config` flag override + default path resolution (`/etc/homelabctl/config.yaml`, then `./config.yaml`)
- [ ] Env var override support (e.g. `HOMELABCTL_CONFIG`)

### 1.3 Validation (`internal/config/validate.go`)
- [ ] `func (c *Config) Validate() error` orchestrator, returns aggregated `error` (use `errors.Join` or a custom `ValidationErrors []error`)
- [ ] Validate sentinel host is **not** present in `nodes:` (enforce separation)
- [ ] Validate every node has a valid MAC address format (regex or `net.ParseMAC`)
- [ ] Validate every node has a valid IP/hostname in `address`
- [ ] Validate every profile's `nodes:` entries reference a key that exists in `nodes:`
- [ ] Validate no duplicate node names within a single profile's list
- [ ] Validate schedule entries: valid time format, no duplicate times, profile reference exists
- [ ] Validate `overrides.default_duration` parses to a positive duration
- [ ] Validate SSH: identity file path exists and is readable (warn, not fail, if missing — daemon may run before keys are provisioned)
- [ ] Validate at most one node has `always_on: true` per physical daemon host (sanity check, not a hard rule — decide during review)

### 1.4 Defaults (`internal/config/defaults.go`)
- [ ] Define default SSH port (22), connect timeout (10s), check_interval (30s) applied when omitted
- [ ] Merge function: per-node `SSH` overrides merged onto global `SSH` defaults (`func MergeSSH(global, override *SSHConfig) SSHConfig`)

### 1.5 Tests (`internal/config/*_test.go`)
- [ ] Table-driven test: valid config parses without error
- [ ] Test: sentinel host duplicated in `nodes:` → validation error
- [ ] Test: profile references undefined node → validation error
- [ ] Test: malformed duration string → parse error
- [ ] Test: per-node SSH override merges correctly, leaves other nodes on global default
- [ ] Golden-file test against the finalized example `deploy/config/config.example.yaml`

---

## Phase 2 — Node Primitives

### 2.1 Node model (`internal/node/node.go`)
- [x] `type Node struct` — wraps `config.NodeConfig` + runtime state (Name, LastSeen, PowerState enum: Unknown/Off/Booting/On/Unreachable)
- [x] `type PowerState int` + `String()` method
- [x] `func NewNode(name string, cfg config.NodeConfig, sshDefaults config.SSHConfig) *Node`

### 2.2 Wake-on-LAN (`internal/node/wol.go`)
- [x] `func SendMagicPacket(mac string, broadcastAddr string) error`
- [ ] Construct magic packet byte sequence (6x 0xFF + 16x MAC repeat) if not using a library
- [ ] Unit test: verify packet byte structure for a known MAC
- [ ] `func (n *Node) Boot(ctx context.Context) error` — sends WoL, logs event

### 2.3 SSH client (`internal/node/ssh.go`)
- [ ] `func NewSSHClient(cfg config.SSHConfig, host string) (*ssh.Client, error)` — loads identity file, sets up `ssh.ClientConfig`
- [ ] Host key verification callback using `known_hosts_file` when `strict_host_key_checking: true`; `ssh.InsecureIgnoreHostKey()` fallback path when false (log a warning every time this path is used)
- [ ] `func (n *Node) RunCommand(ctx context.Context, cmd string) (stdout, stderr string, err error)` — opens session, runs, captures output
- [ ] `func (n *Node) Shutdown(ctx context.Context) error` — runs `sudo shutdown -h now` (or configurable command), handles expected connection-drop error as success
- [ ] Retry/backoff wrapper for transient SSH connection failures (`internal/node/retry.go`)
- [ ] Unit test with an in-process fake SSH server (`golang.org/x/crypto/ssh` test server pattern) to verify `RunCommand` and `Shutdown` without real hardware

### 2.4 Health checks (`internal/node/health.go`)
- [ ] `func Ping(ctx context.Context, address string, timeout time.Duration) (bool, error)` — ICMP echo
- [ ] `func (n *Node) IsReachable(ctx context.Context) bool` wraps `Ping`
- [ ] `func (n *Node) WaitUntilReady(ctx context.Context) error` — static `wait_after_boot` sleep implementation (v1)
- [ ] Stub `Prober` interface now so Phase-9 polling-based readiness can slot in without reshaping callers:
  ```go
  type Prober interface {
      Ready(ctx context.Context, n *Node) (bool, error)
  }
  ```
- [ ] `StaticWaitProber` struct implementing `Prober` using `wait_after_boot` (default/current implementation)

### 2.5 Node registry (`internal/node/registry.go`)
- [ ] `type Registry struct` — map of all configured nodes + sentinel, built from `config.Config` at daemon startup
- [ ] `func NewRegistry(cfg *config.Config) (*Registry, error)`
- [ ] `func (r *Registry) Get(name string) (*Node, error)`
- [ ] `func (r *Registry) All() []*Node` (excludes sentinel/always-on where relevant, with explicit filter methods: `func (r *Registry) Managed() []*Node`)

---

## Phase 3 — Sentinel (Power-Outage Watchdog)

### 3.1 State machine (`internal/sentinel/state.go`)
- [ ] `type SentinelState int` — enum: `Normal`, `OutageDetected`, `OutageConfirmed`, `Restoring`, `Restored`
- [ ] `type Sentinel struct` — holds state, config.SentinelConfig, last-seen timestamp, transition timestamps
- [ ] `func NewSentinel(cfg config.SentinelConfig) *Sentinel`

### 3.2 Watch loop (`internal/sentinel/watch.go`)
- [ ] `func (s *Sentinel) Run(ctx context.Context, onOutage, onRestore func(context.Context)) error` — ticks every `check_interval`
- [ ] Transition logic: ping fails → start grace timer → if still failing after `outage_grace_period` → fire `onOutage`
- [ ] Transition logic: ping succeeds after outage → start restore timer → if still succeeding after `restore_grace_period` → fire `onRestore`
- [ ] Handle flapping: a single successful ping during the outage grace period resets the grace timer (define this explicitly — decide and document the exact debounce rule)
- [ ] Emit structured log line on every state transition (for later API/WebUI surfacing)

### 3.3 Outage/restore actions (`internal/sentinel/actions.go`)
- [ ] `func (s *Sentinel) ShutdownAll(ctx context.Context, reg *node.Registry) error` — shuts down managed nodes in reverse of last-known-active profile order
- [ ] `func (s *Sentinel) RestoreAll(ctx context.Context, reg *node.Registry, resumeProfile func() string) error` — WoLs nodes back per the profile that *should* be active now (queries scheduler for current time slot)
- [ ] Persist "sentinel is currently overriding" flag so scheduler/override logic can check and defer (`internal/sentinel/override_flag.go` or shared state in daemon)

### 3.4 Tests
- [ ] Simulate ping failures via fake `Pinger` interface — verify grace period timing triggers outage exactly once
- [ ] Simulate flap during grace period — verify timer reset behavior matches documented rule
- [ ] Verify restore triggers correct profile resumption (mock scheduler)

---

## Phase 4 — Scheduler

### 4.1 Profile resolution (`internal/scheduler/profile.go`)
- [ ] `func (s *Scheduler) CurrentProfile(now time.Time) (string, error)` — walks `schedule.entries`, finds the latest entry whose time is ≤ now (with wraparound for entries before midnight vs after)
- [ ] `func (s *Scheduler) NextTransition(now time.Time) (time.Time, string, error)` — for daemon's sleep-until-next-tick loop

### 4.2 Scheduler runtime (`internal/scheduler/scheduler.go`)
- [ ] `type Scheduler struct` — holds `config.ScheduleConfig`, `config.Profiles`, reference to `node.Registry`
- [ ] `func (s *Scheduler) ApplyProfile(ctx context.Context, name string) error` — computes diff between currently-active nodes and target profile's nodes
- [ ] Boot new nodes in **list order** (respecting `wait_after_boot`/Prober between each, or parallel-with-stagger — decide and document)
- [ ] Shut down nodes no longer in profile, in **reverse list order** of the *previous* profile
- [ ] `func (s *Scheduler) Run(ctx context.Context) error` — main loop: sleep until `NextTransition`, call `ApplyProfile`, repeat; select on ctx.Done() and an interrupt channel (for override/sentinel preemption)

### 4.3 Interruption handling (`internal/scheduler/interrupt.go`)
- [ ] Channel-based signal so `override` and `sentinel` packages can pause/resume the scheduler loop without racing the ticker
- [ ] `func (s *Scheduler) Pause(reason string)` / `func (s *Scheduler) Resume()`
- [ ] On resume, recompute `CurrentProfile(time.Now())` and reconcile (don't blindly replay missed transitions)

### 4.4 Tests
- [ ] Table-driven test for `CurrentProfile` across all schedule entries + edge cases (exactly on a boundary, before first entry, after last entry, midnight wraparound)
- [ ] Test `ApplyProfile` diff logic: verify correct boot/shutdown sets and ordering for several from→to profile transitions
- [ ] Test pause/resume doesn't double-apply a profile

---

## Phase 5 — Manual Overrides

### 5.1 Override state (`internal/override/override.go`)
- [ ] `type Override struct` — Profile string, ExpiresAt *time.Time (nil = indefinite), CreatedAt time.Time
- [ ] `type Manager struct` — current override (or nil), reference to Scheduler
- [ ] `func (m *Manager) Set(ctx context.Context, profile string, duration *time.Duration) error` — nil duration + explicit `indefinite bool` flag, or duration defaulting to `overrides.default_duration`
- [ ] `func (m *Manager) Clear(ctx context.Context) error` — cancels early, hands control back to scheduler, immediately reconciles to `CurrentProfile(now)`

### 5.2 Expiry watcher (`internal/override/expiry.go`)
- [ ] Timer-based expiry (not polling) using `time.AfterFunc`, cancelable on `Clear`
- [ ] On expiry, call `Manager.Clear` internally and log the transition

### 5.3 Precedence integration
- [ ] Wire into daemon orchestrator: precedence order is **Sentinel > Override > Schedule** — document this explicitly in code comments at the integration point
- [ ] Ensure sentinel outage detection forcibly clears/suspends an active override (per finalized design: sentinel fully overrides both)

### 5.4 Tests
- [ ] Test override with `--for 30m` expires and correctly hands back to schedule's current profile
- [ ] Test `--indefinite` never auto-expires until explicit `clear`
- [ ] Test sentinel outage during an active override suspends it, and restore correctly resumes prior override state (or discards it — decide and document, then test that exact behavior)

---

## Phase 6 — Daemon Core

### 6.1 Orchestrator (`internal/daemon/orchestrator.go`)
- [ ] `type Orchestrator struct` — owns Registry, Scheduler, Override Manager, Sentinel; single source of truth for "what should be running right now"
- [ ] `func (o *Orchestrator) Start(ctx context.Context) error` — starts sentinel watch loop, scheduler loop, override expiry watcher as goroutines under an `errgroup.Group`
- [ ] `func (o *Orchestrator) Status() DaemonStatus` — snapshot struct for API/CLI consumption (current profile, override info, sentinel state, per-node power state)
- [ ] Graceful shutdown: `func (o *Orchestrator) Stop(ctx context.Context) error` — cancels goroutines, does **not** shut down managed nodes (daemon restart ≠ power event)

### 6.2 Event log / audit trail (`internal/daemon/eventlog.go`)
- [ ] `type Event struct` — Timestamp, Type (boot/shutdown/override_set/override_cleared/sentinel_outage/sentinel_restore), Detail
- [ ] In-memory ring buffer (bounded size) + optional append to log file
- [ ] `func (o *Orchestrator) RecentEvents(n int) []Event` for API/CLI `homelabctl status --history`

### 6.3 Entrypoint (`cmd/homelabctld/main.go`)
- [ ] Parse flags (`--config`, `--log-level`)
- [ ] Load + validate config, exit non-zero with clear error on failure
- [ ] Set up logger (`internal/logging`)
- [ ] Construct Registry → Scheduler → Sentinel → Override Manager → Orchestrator
- [ ] Start orchestrator, start API server (Phase 7), block on OS signal (SIGINT/SIGTERM) for graceful shutdown

---

## Phase 7 — Internal API (REST/WebSocket)

### 7.1 Route definitions (`internal/api/router.go`)
- [ ] `GET /v1/status` — full `DaemonStatus` JSON
- [ ] `GET /v1/nodes` — list nodes + state
- [ ] `GET /v1/nodes/{name}` — single node detail
- [ ] `POST /v1/nodes/{name}/boot` — manual single-node boot (out-of-band of profiles)
- [ ] `POST /v1/nodes/{name}/shutdown`
- [ ] `GET /v1/profiles` — list configured profiles
- [ ] `POST /v1/override` — body: `{profile, duration_seconds|indefinite}`
- [ ] `DELETE /v1/override` — clear active override
- [ ] `GET /v1/schedule` — current schedule + next transition
- [ ] `GET /v1/events?limit=N` — recent event log
- [ ] `GET /v1/ws` — WebSocket upgrade for live status/event streaming

### 7.2 Handlers (`internal/api/handlers.go`)
- [ ] Each route → thin handler calling into `Orchestrator`/`Manager`, JSON encode/decode, proper HTTP status codes (400 for bad profile name, 409 for conflicting override, etc.)
- [ ] Request validation middleware (decode + validate body before hitting orchestrator)

### 7.3 WebSocket broadcaster (`internal/api/ws.go`)
- [ ] Hub pattern: `type Hub struct` managing connected clients, broadcast channel
- [ ] Orchestrator emits status-change events → Hub broadcasts to all connected clients
- [ ] Ping/pong keepalive, clean disconnect handling

### 7.4 Middleware (`internal/api/middleware.go`)
- [ ] Request logging middleware
- [ ] Recover-from-panic middleware
- [ ] (Local-only for v1 — bind to `127.0.0.1` per config; note auth is explicitly deferred until WebUI phase needs remote exposure)

### 7.5 Server lifecycle (`internal/api/server.go`)
- [ ] `func NewServer(cfg config.DaemonConfig, orch *daemon.Orchestrator) *Server`
- [ ] Graceful `http.Server` shutdown wired into daemon's Stop()

### 7.6 Tests
- [ ] `httptest`-based tests for every route: happy path + validation-error path
- [ ] WebSocket test: connect, trigger a status change via orchestrator, assert message received

---

## Phase 8 — CLI (Thin API Client)

### 8.1 API client (`internal/apiclient/client.go`)
- [ ] `type Client struct` — base URL/socket, `http.Client`
- [ ] One method per API route from Phase 7 (`Status()`, `ListNodes()`, `SetOverride()`, `ClearOverride()`, etc.)
- [ ] Connection error handling with a clear "is the daemon running?" message

### 8.2 Root command (`internal/cli/root.go`)
- [ ] `cobra.Command` root, persistent flags (`--config`, `--api-addr`, `--json` for machine-readable output)

### 8.3 Subcommands (`internal/cli/*.go`) — one file per command
- [ ] `status.go` — `homelabctl status` → pretty-print `DaemonStatus`
- [ ] `nodes.go` — `homelabctl nodes list`, `homelabctl nodes show <name>`
- [ ] `boot.go` — `homelabctl boot <node>`
- [ ] `shutdown.go` — `homelabctl shutdown <node>`
- [ ] `profile.go` — `homelabctl profile list`, `homelabctl profile show <name>`
- [ ] `override.go` — `homelabctl override <profile> [--for DURATION] [--indefinite]`, `homelabctl override clear`
- [ ] `schedule.go` — `homelabctl schedule show`
- [ ] `events.go` — `homelabctl events [--limit N]`
- [ ] `version.go` — build info

### 8.4 Output formatting (`internal/cli/output.go`)
- [ ] Table renderer for human output (align columns, no color yet per finalized decision to defer)
- [ ] JSON renderer for `--json` flag (shared struct, just marshal)

### 8.5 Tests
- [ ] Cobra command tests using a mocked `apiclient.Client` (interface-based so it's swappable)

---

## Phase 9 — Deployment (falcon-control)

- [ ] Procure/image the Raspberry Pi for falcon-control (hardware step — outside code)
- [ ] Write `deploy/systemd/homelabctld.service` unit file (matches `daemon.systemd` config block: unit name, restart policy, `After=network-online.target`)
- [ ] Write install script (`deploy/install.sh`): copies binaries to `/usr/local/bin`, config to `/etc/homelabctl/`, sets up `/etc/homelabctl/keys/`, enables + starts systemd unit
- [ ] Cross-compile for Raspberry Pi arch (`GOOS=linux GOARCH=arm64` or `arm` depending on Pi model) in `Makefile`
- [ ] Migrate Pi-hole, UpSnap, Tailscale from falcon1 → falcon-control (infra step, document runbook in `deploy/MIGRATION.md`)
- [ ] Generate/distribute the shared SSH keypair (or per-node keys, per earlier decision) to all managed nodes' `authorized_keys`
- [ ] Smoke test: full profile transition cycle (`backup` → `movie` → `off`) against real hardware
- [ ] Smoke test: simulate an outage (unplug/pause falcon-lite reachability) and confirm shutdown/restore cycle end-to-end

---

## Phase 10 — Polish & Deferred Items

- [ ] Replace `StaticWaitProber` with a real polling `Prober` implementation (e.g. HTTP health-check against Jellyfin/Immich ports on falcon-heavy) — swap-in only, thanks to the interface from Phase 2.4
- [ ] Color output for CLI tables (respect `NO_COLOR` env var, TTY detection)
- [ ] WebUI: htmx + server-rendered Go templates consuming the same `internal/api` routes (separate track — flag for its own development plan once core CLI is stable)
- [ ] Config hot-reload (SIGHUP or file-watch) instead of requiring daemon restart on config changes
- [ ] Structured metrics endpoint (`/v1/metrics` — Prometheus format) if you want dashboarding later

---

## Suggested Build Order Summary

1. Phase 0 → 1 (scaffolding + config) — nothing else can start without this
2. Phase 2 (node primitives) — can be built/tested against real nodes early, in parallel with Phase 3–5 logic using mocks
3. Phase 3, 4, 5 in parallel (sentinel / scheduler / override are independent state machines until Phase 6 wires them together)
4. Phase 6 (orchestrator) — integration point
5. Phase 7 (API) then Phase 8 (CLI) — CLI is trivial once API is solid, by design
6. Phase 9 (deploy) — first time touching real falcon-control hardware
7. Phase 10 — ongoing
