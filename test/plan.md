# API Test Plan — homelabctl

> Status: plan only, nothing implemented yet.
> Scope: the HTTP API in `api/api.go`, plus the layers under it (`daemon/orchestrator.go`, `node/`, `sentinel/`) that decide what the API returns.

---

## 1. Current state (why the API can't be tested well yet)

- The repo has no `*_test.go` files. `make test` already runs `go test -race -coverprofile=coverage.out ./...`, so the tooling is in place.
- **Every handler returns HTTP 200.** Each handler calls `w.WriteHeader(http.StatusOK)` *before* asking the backend for a result, so errors, unknown nodes and refused actions all come back as 200 with `{"status": false, ...}`.
- **The API layer can't tell success from failure.** The backend functions (`StatusFunc`, `BootFunc`, …) return a plain `[]byte` holding already-encoded JSON. Even if the handlers wanted to pick a status code, they have no error value to base it on.
- **The router isn't exposed.** `Server.router` is private and `Run()` hard-codes `ListenAndServe(":3000")`, so tests would need a real port. `Run` also ignores `ctx`.
- **Nothing can be faked.** These are all called directly:
  - `utils.Ping`: real ICMP, needs privileges or the network
  - `gowol.NewMagicPacket(...).Send`: real UDP broadcast
  - `NewSSHClient` / `RunCommand`: real SSH on a hard-coded `:22`
  - `time.NewTimer`: real boot and shutdown waits of 180 s or more
- **Data races.** `Node.State`, `Node.LastSeen`, `Sentinel.State` and the sentinel timestamps are written by background tasks and read by HTTP handlers with no locking. `go test -race` will flag this once any concurrent test exists.

---

## 2. Target HTTP contract (what the tests assert)

These tests are written **first** and are **meant to fail against the current code**, because the code always returns 200. They should pass only once the handlers return proper status codes.

| # | Request | Situation | Expected code | Today |
|---|---|---|---|---|
| C1 | `GET /status` | normal | **200** | 200 ✅ |
| C2 | `GET /nodes` and `GET /nodes/` | normal | **200** | 200 ✅ (check that both paths route) |
| C3 | `GET /nodes/{name}` | node exists | **200** | 200 ✅ |
| C4 | `GET /nodes/{name}` | node unknown | **404** | 200 ❌ |
| C5 | `POST /nodes/{name}/boot` | WoL packet sent | **202 Accepted** (boot finishes in the background) | 200 ❌ |
| C6 | `POST /nodes/{name}/boot` | node unknown | **404** | 200 ❌ |
| C7 | `POST /nodes/{name}/boot` | node already `on` | **409 Conflict** | 200 ❌ |
| C8 | `POST /nodes/{name}/boot` | node `booting` / `shutting` / `unknown` | **409 Conflict** | 200 ❌ |
| C9 | `POST /nodes/{name}/boot` | sentinel not `Normal` (outage) | **503 Service Unavailable** | 200 ❌ |
| C10 | `POST /nodes/{name}/boot` | building/sending the WoL packet failed (e.g. bad MAC) | **502 Bad Gateway** (or 500) | 200 ❌ |
| C11 | `POST /nodes/{name}/shutdown` | SSH command sent | **202 Accepted** | 200 ❌ |
| C12 | `POST /nodes/{name}/shutdown` | node unknown | **404** | 200 ❌ |
| C13 | `POST /nodes/{name}/shutdown` | node already `off` | **409** | 200 ❌ |
| C14 | `POST /nodes/{name}/shutdown` | node `booting` / `shutting` / `unknown` | **409** | 200 ❌ |
| C15 | `POST /nodes/{name}/shutdown` | SSH dial, auth or command failed | **502** | 200 ❌ |
| C16 | `GET /nodes/{name}/boot` | wrong method | **405** | 405 ✅ (chi already does this; lock it in) |
| C17 | `GET /does-not-exist` | unknown route | **404** | 404 ✅ (chi already does this; lock it in) |
| C18 | any JSON route | — | `Content-Type: application/json` on success **and** error responses | partly |
| C19 | any error response | — | body has `status: false` and a non-empty `message` | ✅ shape exists; keep it |

### How the tests catch "always 200"

- **Exact checks.** Every test checks `rec.Code == want`, never just "is 2xx". Specifically:
  - C4, C6–C10 and C12–C15 all fail today because they get 200.
  - C5 and C11 also fail today, since they expect 202 rather than 200. (If you'd prefer 200 on success, change those two rows; the error rows still do the job.)
- **Consistency check across all cases.** One table-driven test, `TestErrorResponsesAreNot200`, runs every error scenario and checks two things together:
  - `body.status == false` ⇒ `rec.Code >= 400`
  - `rec.Code >= 400` ⇒ `body.status == false`

  So the code can never again send a failure body with a 200 status, or the reverse.
- **Guard for success paths.** Another test, `TestSuccessResponsesHaveStatusTrue`, checks that 2xx responses always carry `status: true`. Without it, a later change could "fix" the codes by returning 4xx for everything and still pass.

---

## 3. Code changes needed before or alongside the tests

### 3.1 Error kinds (new package, e.g. `apierr/` or `internal/errs/`)
- Add shared error values: `ErrNotFound`, `ErrConflict`, `ErrUnavailable`, `ErrUpstream`.
- `node`, `registry` and `orchestrator` wrap their errors with them using `fmt.Errorf("...: %w", errs.ErrConflict)`. Each wrapped error must still read clearly as the user-facing `message`.
- The API layer maps them to codes with `errors.Is`:
  - `ErrNotFound` → 404
  - `ErrConflict` → 409
  - `ErrUnavailable` → 503
  - `ErrUpstream` → 502
  - anything else → 500
- Where each error comes from:
  - `registry.NodeStatus`, `Boot` and `Shutdown` "invalid node" → `ErrNotFound`
  - `node.Boot` / `node.Shutdown` state guards → `ErrConflict`
  - `orchestrator.Boot` sentinel check → `ErrUnavailable`
  - WoL and SSH failures → `ErrUpstream`

### 3.2 API layer (`api/api.go`)
- **Use a backend interface instead of the five `func` fields and `SetFunc`.** Today's positional `SetFunc` is easy to wire up wrong.
  ```go
  type Backend interface {
      Status(ctx context.Context) (any, error)
      ListNodes(ctx context.Context) (any, error)
      GetNode(ctx context.Context, name string) (any, error)
      Boot(ctx context.Context, name string) (any, error)
      Shutdown(ctx context.Context, name string) (any, error)
  }
  ```
  - The `api` package must **not** import `daemon`, because `daemon` already imports `api` and that would be an import cycle. Hence the `any` return values.
  - `Orchestrator` (or a small adapter in `daemon`) implements this interface.
- **Move JSON encoding into the API layer** with a `writeJSON(w, code, v)` helper. Set `Content-Type` first, then `WriteHeader(code)`, then encode. `utils.ToJSON` stops being used from the orchestrator.
- **Add `writeError(w, err)`**, which maps the error to a status code and writes `{"status": false, "message": err.Error()}`.
- **Add `func (s *Server) Handler() http.Handler`** so tests can use `httptest.NewRecorder()` / `httptest.NewServer(s.Handler())` without binding `:3000`.
- **Make `Run(ctx)` use `http.Server{Addr: cfg...}`** and call `srv.Shutdown` when `ctx` is done (this closes the existing TODO). Make the address configurable.
- Optional: a `GET /health` route returning the version, replacing `/hello`.

### 3.3 Node layer (`node/`) — dependency injection for mocking
- Add small interfaces/function types and pass them in through `NewNode` / `NewRegistry` (an options struct or functional options). Production defaults stay the same.
  ```go
  type Pinger      func(address string) bool                       // default: utils.Ping
  type WakeSender  func(mac string) error                          // default: gowol packet + Send("255.255.255.255")
  type Runner      interface { Run(ctx context.Context, addr, cmd string) (stdout, stderr string, err error) } // default: SSH
  type After       func(d time.Duration) <-chan time.Time          // default: time.After
  ```
  - With `After`, tests can finish `FinishBooting` / `FinishShutdown` instantly.
- Protect `State` and `LastSeen` with a `sync.RWMutex`. Add `State()` / `setState()` accessors and stop reading the field directly from outside the package.
- Bugs these tests will catch (fix them as they turn up):
  - `RunCommand` never copies the `b` / `e` buffers into the returned `stdout` / `stderr`, so they're always empty.
  - `FinishShutdown` waits for `BootTimeout`. It needs its own shutdown timeout, or at least a deliberate decision to reuse it.
  - The SSH port is hard-coded to `:22` and ignores `SSHConfig.Port`. `ConnectionTimeout` and `KnownHostsFile` are also ignored.
  - `ShutdownAll` ignores the error from each node and runs them one after another.

### 3.4 Sentinel (`sentinel/`)
- Inject a `Pinger` the same way.
- Pull the body of the `switch` in `Run` out into `func (s *Sentinel) step(ctx, pingOK bool)`. That way the state machine can be tested with no ticker and no network.
- Guard `State`, the grace counters and the timestamps with a mutex, and add a `State()` accessor. `Orchestrator.Boot` currently reads `o.Sentinel.State` directly.
- Bugs these tests will catch:
  - In `Restoring`, when the ping fails, it moves to `OutageDetected` but then doesn't `break`. `GraceUpTime` still increases and could even move it to `Restored`.
  - `OutageConfirmed` calls `OnOutage` on **every tick** until power comes back. Decide whether that should happen once or repeatedly, and pin it down with a test.
  - `Restored` sets the state twice (`s.State = Restored` and then `SetState(Restored)`), which is harmless but noisy.
  - If `Init()` is never called, the state stays `Unknown`. The `switch` has no case for it, so the sentinel stays stuck there forever.

### 3.5 Orchestrator (`daemon/`)
- Have it depend on small interfaces (`registry`, `sentinelReader`) instead of the concrete `*node.Registry` and `*sentinel.Sentinel`. Orchestrator tests can then use fakes, for example "sentinel in outage" → boot gives `ErrUnavailable`.

---

## 4. Test suites

### 4.1 `api/api_test.go` — handler tests (fake `Backend`)
- `fakeBackend` returns whatever result or error each test sets up, and records the calls it receives (method and node name).
- Table-driven over contract rows C1–C19. Each test checks:
  - the status code
  - the `Content-Type` header
  - that the body decodes as JSON
  - `status` / `message`
  - that the backend got the right node name
- Includes `TestErrorResponsesAreNot200` and `TestSuccessResponsesHaveStatusTrue` (see §2).
- Edge cases:
  - URL-encoded names (`/nodes/falcon%2Dheavy`)
  - an empty name (`/nodes//boot`)
  - a trailing slash
  - a backend that panics → 500. The default middleware stack has no recoverer, so add `middleware.Recoverer`.
- Context: if the request context is cancelled, the backend sees the cancellation.

### 4.2 `api/contract_test.go` — JSON shape / golden tests
- Saved "golden" copies of the expected JSON for `/status`, `/nodes` and `/nodes/{name}`.
- These protect the field names a future web UI will rely on (`power_state`, `last_seen`, `address`, `mac`, `boot_timeout`, `sentinel.state`, …).
- Decide how to handle the zero `last_seen` time (`0001-01-01T00:00:00Z`): leave the field out or return `null`, then test it.

### 4.3 `daemon/orchestrator_test.go` — full API tests with fakes
- Build a real `Orchestrator` + real `api.Server`, but give the node/registry/sentinel layers fake pingers, WoL senders, SSH runners and clocks. Then drive the whole system with `httptest.NewServer`.
- Scenarios:
  - Boot an `off` node → 202. `/nodes/x` now shows `booting`. Advance the fake clock and make the fake ping succeed → `on`.
  - Boot during an outage → 503, and the WoL sender is **not** called.
  - Shutdown an `on` node → 202, and the runner received `shutdown now` → `shutting` → (clock advances, ping fails) → `off`.
  - Make the fake SSH runner fail → 502, and the state is **unchanged** (still `on`).
  - Make the fake WoL sender fail → 502, and the state is still `off`.
  - Unknown node on every route → 404.

### 4.4 `node/node_test.go` and `node/registry_test.go`
- Table of every `(State, action)` pair → expected error kind and next state.
- `Init` sets `on` or `off` (and `LastSeen`) depending on the fake ping result.
- `FinishBooting` / `FinishShutdown` re-check reachability after the fake timer fires.
- `Registry.Status` includes every configured node.
- `ShutdownAll` calls the runner once per `on` node and skips `off` nodes.
- Concurrency: run `Boot` and `Status` from several goroutines under `-race`.

### 4.5 `sentinel/sentinel_test.go` — state machine
- Table-driven tests on `step()`:
  - `Normal` + ping fail → `OutageDetected`
  - `OutageDetected` + ping ok → `Normal`
  - `OutageDetected` + enough failed pings to reach `OutageGracePeriod` → `OutageConfirmed`, and the outage handler is called
  - `OutageConfirmed` + ping ok → `Restoring`
  - `Restoring` + ping fail → `OutageDetected`, and the uptime counter does **not** increase (regression test for the missing `break`)
  - `Restoring` + enough ok pings → `Restored` → `Normal`, and the restore handler is called once
- Check that the grace counters reset on every transition and that `LastTransitioned` is updated (using a fake clock).
- `Run(ctx)` exits promptly when `ctx` is cancelled.

### 4.6 Concurrency / race tests
- Hit `GET /status` from N goroutines while the fake clock and pinger drive node and sentinel changes. Run under `go test -race` (already the default in `make test`).
- This test is expected to **fail under `-race` today**, which proves the mutex work in §3.3 and §3.4 is needed.

### 4.7 Smaller unit tests
- `config/load_test.go`:
  - load a valid YAML file from `testdata/`
  - a missing file → error
  - bad YAML → error
- `config/validate_test.go`: `Validate` is a stub that always returns nil today. Write tests for the rules it should enforce, which will fail at first:
  - unknown node in a profile
  - bad MAC address
  - bad schedule time
  - a check interval of 0
  - a grace period shorter than the check interval
- `utils/utils_test.go`: `ToJSON` on a struct and on `nil`. It returns `null` on error, and the error is logged at Info level.

### 4.8 Optional integration tests (behind a build tag, `//go:build integration`)
- **SSH:** start an in-process SSH server using `golang.org/x/crypto/ssh` on a random port. This tests `RunCommand` for real, including the stdout/stderr bug and port handling.
- **WoL:** listen on a UDP socket on localhost and check that the magic packet bytes are correct. This needs the broadcast address to be configurable.
- **Ping:** skip unless running with privileges or `CAP_NET_RAW`.

---

## 5. Order of work

1. **Add the "red" tests.**
   - Add `api/api_test.go`, wired up against the router through a tiny `Handler()` accessor.
   - Encode the contract table from §2. C4–C15 are expected to fail. That failure proves the always-200 bug exists.
2. **Error kinds + API refactor** (§3.1, §3.2): backend interface, `writeJSON` / `writeError`, `Handler()`, context-aware `Run`. The API tests go green.
3. **Dependency injection in node and sentinel** (§3.3, §3.4) plus the `step()` extraction. Add the node, registry and sentinel unit tests.
4. **Orchestrator interfaces and full API tests** (§3.5, §4.3).
5. **Mutexes**, then the race tests (§4.6).
6. **Golden/contract tests** (§4.2). Freeze the JSON shape before any web UI work.
7. Config and utils tests, then optional integration tests.

## 6. Open decisions
- Should a successful boot or shutdown return **202** (it finishes in the background) or **200**?
- Should a boot during an outage return **503** or **409**?
- Should WoL or SSH failures return **502** or **500**?
- Should `OnOutage` fire once per outage, or on every tick while the outage is confirmed?
- Should the zero `last_seen` time be left out or returned as `null`?

## 7. Unrelated issue spotted
- The `makefile` sets `LDFLAGS` with `$(MODULE)/pkg/version.*`, but the package is at `$(MODULE)/version`. The version info is therefore probably never set in built binaries. This is worth its own small test (`version.Get()` with ldflags) or a manual check.
