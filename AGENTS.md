# Agents Guide for homelabctl

This document provides essential context and patterns for agents (LLMs, CLI bots, or developers) working in the `homelabctl` repository.

## Project Overview

**homelabctl** is a Go-based CLI + daemon that orchestrates a home lab running on battery/inverter backup. It safely handles power outages by gracefully shutting down nodes before battery drain and automatically restoring them when power returns. It also manages scheduled power profiles ("backup mode", "movie night", "off") with manual overrides.

**Precedence hierarchy:** Sentinel (power outage) > Manual Override > Schedule

## Essential Commands

```bash
# Build both binaries
make build

# Run the daemon locally (requires config.yaml in current directory)
make run-daemon
make build-daemon && ./bin/homelabctld --config ./config.yaml
make build-cli && ./bin/homelabctl status

# Full test suite with coverage
make test

# Faster tests (local development)
make test-short

# Verify code quality (requires golangci-lint)
make check  # runs fmt, vet, lint, test
make vet
make lint

# Format Go code
make fmt

# Raspberry Pi 5 (falcon-control) deployment — see deploy/README.md
make build-pi        # static arm64 daemon binary in dist/
make deploy          # build, push to ~/.local/bin on the Pi, restart the user service
make deploy-config   # copy deploy/config.yaml (gitignored, real config) to ~/.config/homelabctl/

# Clean build artifacts
make clean
```

## Project Structure

```
cmd/homelabctl/        # CLI entrypoint (uses kong, not cobra)
cmd/homelabctld/       # Daemon entrypoint

config/                # Config parsing + validation
  - types.go           # All struct definitions: Config, NodeConfig, ProfileConfig, ScheduleEntry
  - load.go            # Load() function for reading YAML config
  - validate.go        # Validation rules (sentinel separation, MAC format, profile refs)

node/                  # Node operations
  - node.go            # Node model with state machine (Unknown/Off/Booting/On/Unreachable)
  - registry.go        # Registry of all nodes with Get/All/Managed methods
  [wol.go]             # Wake-on-LAN implementation
  [ssh.go]             # SSH client for shutdown commands
  [health.go]          # ICMP ping + readiness prober interface

api/                   # REST / WebSocket API server
  - api.go             # chi router with handlers for status, nodes, boot, shutdown

daemon/                # Orchestrator wiring everything together
  - orchestrator.go    # Core daemon: connects config → node registry, sentinel, API server

sentinel/              # Power-outage watchdog state machine
  - sentinel.go        # State transitions (Normal → OutageDetected → OutageConfirmed → ...)
  - watch.go           # Ping loop with grace period timers
  [actions.go]         # ShutdownAll(), RestoreAll() functions

deploy/                # Deployment artifacts
  - systemd/homelabctld.service   # user systemd unit (systemctl --user)
  - README.md                     # Pi deployment runbook
  - config/config.example.yaml    # Example configuration (unstructured YAML for documentation)
```

## Configuration Format

The daemon reads from `config.yaml` (resolved by default, or overridden with `--config`). Structure:

```yaml
ssh:                          # Global SSH defaults (per-node can override)
  user: root
  identity_file: /path/to/key
  port: 22
  connection_timeout: 10
  strict_host_key_checking: true
  known_hosts_file: /etc/homelabctl/keys/known_hosts

sentinel:                     # Power-watchdog proxy (NOT a managed node)
  host: falcon-lite
  address: 192.168.10.46       # MUST be on mains power
  check_interval: 30s        # How often to ping sentinel
  outage_grace_period: 900s  # 15min of no response before declaring outage
  restore_grace_period: 900s # 15min after sentinel returns before restoring nodes

nodes:                        # Managed nodes (on battery/inverter backup)
  falcon-heavy:
    address: 192.168.10.45
    mac: "aa:bb:cc:00:00:20"
    wait_after_boot: 180     # Static delay before considered "ready" (v1)
    services:                 # For future service discovery
      - jellyfin
      - immich
      - truenas

profiles:                     # Profiles define which nodes run, in what order
  backup:
    nodes: [falcon1, falcon-heavy]   # list order = boot/shutdown order
  movie:
    nodes: [falcon-heavy, falcon2]
  off:
    nodes: []

schedule:                     # Daily schedule with timezone
  timezone: "Asia/Kolkata"
  entries:
    - time: "09:00"                    # 24h format HH:MM
      profile: backup
    - time: "12:30"
      profile: movie
    - time: "23:00"
      profile: off

# overrides section is commented in example; CLI handles --for/--indefinite flags
```

**Key validation rules:**
- Sentinel host/address must NOT appear in `nodes:` map (enforced)
- Every node requires a valid MAC address format and IP/hostname
- Profile `nodes:` arrays must reference existing keys in `nodes:`
- No duplicate node names within a single profile's list
- Schedule entries have unique times, no duplicates
- At most one node may be `always_on: true` per daemon host (sanity check)

## Code Patterns & Architecture

### State Machine for Nodes

Each managed node has an explicit state tracked in memory:

```go
const (
    UNKNOWN     string = "unknown"
    OFF         string = "off"
    BOOTING     string = "booting"
    SHUTTING    string = "shutting"
    ON          string = "on"
    UNREACHABLE string = "unreachable"
)
```

Boot/shutdown flow:
1. **Boot:** Send WoL → set state `BOOTING` → goroutine waits for `wait_after_boot` duration → then ping → if pingable, set state `ON`
2. **Shutdown:** Run `sudo shutdown -h now` → handle expected connection drop → set state `SHUTTING` → goroutine waits for timeout → mark `OFF`

### API-First Design

The daemon is built as an API-first system:
- `homelabctld` exposes REST endpoints on `127.0.0.1:3000`
- The CLI (`homelabctl`) is a thin client that calls the daemon's API
- No separate code paths for CLI vs daemon actions

This means a future web UI is just another HTTP client — no rework needed.

### Config Validation Pipeline

```go
// Load → Merge defaults → Validate → Use
func Load(path string) (*Config, error)        // Read YAML
func (c *Config) Validate() error              // All validation rules in one place
```

Validation aggregates errors (`errors.Join` or custom `ValidationErrors[]error`) and checks all cross-references.

### Override Precedence Integration

Three actors compete for fleet control:

1. **Sentinel:** Highest priority — triggers on power outage, shuts down ALL managed nodes
2. **Override:** Manual CLI overrides (e.g., `--for 2h`) beat schedule but yield to sentinel
3. **Schedule:** Lowest priority — runs all other logic

Integration point (in `daemon/orchestrator.go`): scheduler loop sleeps until next profile transition, but listens on interrupt channels from both override-manager and sentinel for preemption.

## Testing Approach

- Table-driven tests scattered throughout packages
- Use `-race -coverprofile=coverage.out ./...` for full test run
- Generate HTML coverage with `make coverage`
- Integration tests live in `test/integration/` (stubbed, to be implemented)

Example from [`api/api_test.go`](./api/api_test.go) and [`node/node_test.go`](./node/node_test.go).

## Gotchas & Non-Obvious Patterns

### 1. Sentinel separation is enforced
The sentinel node exists purely as a power watchdog. It's never treated as part of the managed fleet — even though config allows arbitrary structure, validation explicitly rejects placing the sentinel host in `nodes:` map.

### 2. Static waits for readiness (v1)
Current implementation uses a `wait_after_boot` sleep instead of probing service ports. A `Prober` interface is stubbed so polling-based health checks can slot in later without changing calling code:

```go
type Prober interface {
    Ready(ctx context.Context, n *Node) (bool, error)
}
// StaticWaitProber uses wait_after_boot; future PollingProber would probe ports
```

### 3. Override expiry is timer-based, not polling
When you set an override with `--for 2h`, it doesn't poll every second to check time — a single `time.AfterFunc` fires when the duration elapses. This saves CPU and is simpler.

### 4. CLI uses Kong, not Cobra
Despite what README mentions, the code uses [`github.com/alecthomas/kong`](go.mod) for CLI definition (`cmd/homelabctl/main.go`). Commands are defined using Kong's `cmd`, `args`, `defaults` syntax, not cobra flags.

### 5. API is local-only by default (v1)
The REST server binds to `127.0.0.1:3000`. Auth isn't implemented until the web UI phase requires it — it's deliberately absent rather than bolted on later. This keeps security properties explicit.

### 6. Shutdown errors are expected and ignored
When running `sudo shutdown -h now` via SSH, the connection will drop. This error is handled as success:

```go
// Expected EOF/connection-drop after shutdown command — treat as OK
stdout, stderr, err := n.RunCommand(ctx, "shutdown now")
if err != nil { /* ignore, expected */ }
n.State = SHUTTING
```

### 7. Profile order matters for boot/shutdown sequence
Boot order: profile nodes list left-to-right
Shutdown order: reverse of the *previous* profile's list (implemented via `node.Registry.ShutdownAll`)

### 8. Grace periods debounce flapping
During an outage, a single successful ping resets the restore grace period timer. This prevents premature wake-ups when network blips occur. The exact rule is documented in `sentinel/watch.go` transition logic.

## Coding Guidelines

### Edit in small steps
When refactoring or adding code, make multiple small edits rather than large replacements. For example:

1. Add the function signature as one edit
2. Add the function body as another edit (or split further into helper functions)
3. Continue splitting until each change is under 10-15 lines

This approach reduces whitespace-matching failures during find/replace and makes reviews easier. The edit tool requires exact text matches, so smaller changes are less error-prone.

### Avoid changing entire files at once
If you'd like to replace an entire function or struct definition:
- Read the file to get the exact boundaries
- Use `lsp_replace_symbol` instead of `edit` when supported by LSP (Go does)
- This eliminates whitespace-matching failures and handles scoping automatically

### File organization patterns
- Keep config-related code in one place (`config/`)
- Avoid cross-package dependency loops; keep packages cohesive
- Use `log/slog` consistently for structured logging with contextual fields

### Testing while developing
After each small edit, if feasible:
1. Run `make fmt vet` first to catch syntax errors
2. Then run unit tests in that package
3. Check LSP diagnostics before committing

## Utilities Package

[`utils/utils.go`](./utils/utils.go) contains helper functions used across packages:

- `Ping(address, timeout)` → ICMP echo
- `Wake(mac, broadcastAddr)` → UDP magic packet
- `SSHRun(host, cmd, sshCfg)` → SSH execution with output capture
- `After(duration)` → returns cancelable timer (used for boot/shutdown waits)
- `ParseErrCode()` / `ToJSON()` → API response helpers
- `ErrNotFound`, `ErrConflict`, `ErrUpstream` — error types used internally

## Agents Package ([./agents/agents.go](agents.go))

[`agents/agents.go`](./agents/agents.go) defines interfaces that plug into the node operations:

```go
type Pinger   interface { Err (Context, string) (bool, error ) } // ICMP or custom health check
type Runner   interface { Err (Context, string, SSHConfig) (Stdout, Stderr, error) } // SSH execution
type WakeSender interface { Err (string) } // WoL sender
type After interface { Cancel (func()) } // Timer wrapper for boot/shutdown waits
```

These let you swap implementations: v1 uses `utils.*` functions; future work can provide polling-based services or alternative health checks.

## Common Tasks

### Boot a single node outside of profile/override
```bash
homelabctl boot falcon-heavy
```

### Override the scheduler for 3 hours
```bash
homelabctl override movie --for 3h
# This doesn't change the schedule; it just forces "movie" profile for 3h.
# At expiry, scheduler recomputes current-time profile without replaying missed transitions.
```

### Check what profile should be active now
```bash
go build ./cmd/homelabctl && ./bin/homelabctl status
# Outputs: Profile: movie (active since 12:30, next: off @ 23:00)
```

### Deploy to the Raspberry Pi
The daemon runs as the user (not root) under a user systemd unit, with config
in `~/.config/homelabctl/`. One-time setup (unit, `loginctl enable-linger`,
`ping_group_range` sysctl) is manual and documented in `deploy/README.md`.
```bash
make deploy-config   # first time, and after config changes
make deploy          # every release
```

## Future Work (Not Yet Implemented)

- `test/integration/` — integration test suite is stubbed, needs implementation scenarios
- [`cli/argparse.go`](./cli/argparse.go) missing CMD type implementations for Kong subcommands (compile errors present)
- Scheduled: webhook notifications on node state changes
- Scheduled: color output for CLI tables
- Scheduled: Web UI using htmx + Go templates

## Related Files

- [`README.md`](./README.md) — Architecture overview and design principles
- [`USAGE.md`](./USAGE.md) — End-user walkthrough (draft, with GIF placeholders)
- [`TRACK.md`](./TRACK.md) — Granular development checklist (Phase 0 through Phase X)
- [`deploy/config/config.example.yaml`](./deploy/config/config.example.yaml) — Example configuration
- [`makefile`](./makefile) — Build/test/lint targets
