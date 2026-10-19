MODULE      := github.com/pspiagicw/homelabctl
CLI_PKG     := ./cmd/homelabctl
DAEMON_PKG  := ./cmd/homelabctld

BIN_DIR     := bin
DIST_DIR    := dist

VERSION     := $(shell git describe --tags --dirty --always 2>/dev/null || echo "dev")
COMMIT      := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_DATE  := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS     := -X '$(MODULE)/version.Version=$(VERSION)' \
               -X '$(MODULE)/version.Commit=$(COMMIT)' \
               -X '$(MODULE)/version.BuildDate=$(BUILD_DATE)'

GO          := go
GOFLAGS     :=

# Raspberry Pi 5 (falcon-control) target. Override at the command line if
# needed, e.g. `make deploy PI_HOST=192.168.10.5`.
PI_GOOS     := linux
PI_GOARCH   := arm64
PI_HOST     ?= falcon-control
PI_BIN      := $(DIST_DIR)/homelabctld_$(PI_GOOS)_$(PI_GOARCH)

# PI_HOST is an ~/.ssh/config alias with key auth. BatchMode makes a missing
# key or alias fail immediately instead of hanging on a password prompt.
SSH         := ssh -o BatchMode=yes
SCP         := scp -o BatchMode=yes

.PHONY: all
all: build

## ---- Build ----------------------------------------------------------------

.PHONY: build
build: build-cli build-daemon ## Build both CLI and daemon binaries

.PHONY: build-cli
build-cli: ## Build the homelabctl CLI binary
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/homelabctl $(CLI_PKG)

.PHONY: build-daemon
build-daemon: ## Build the homelabctld daemon binary
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/homelabctld $(DAEMON_PKG)

## ---- Cross-compile (falcon-control deployment) -----------------------------

# Daemon only: the CLI doesn't compile yet.
.PHONY: build-pi
build-pi: ## Cross-compile a static daemon binary for the Raspberry Pi
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=$(PI_GOOS) GOARCH=$(PI_GOARCH) $(GO) build \
		-ldflags "$(LDFLAGS)" -o $(PI_BIN) $(DAEMON_PKG)
	@echo "Built for $(PI_GOOS)/$(PI_GOARCH) -> $(PI_BIN)"

## ---- Test / Lint / Vet -----------------------------------------------------

.PHONY: test
test: ## Run the full test suite with coverage
	$(GO) test -race -coverprofile=coverage.out ./...

.PHONY: test-short
test-short: ## Run tests without race detector or coverage (faster, local iteration)
	$(GO) test ./...

.PHONY: coverage
coverage: test ## Generate an HTML coverage report from the last test run
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

.PHONY: lint
lint: ## Run golangci-lint (requires golangci-lint installed)
	golangci-lint run ./...

.PHONY: vet
vet: ## Run go vet
	$(GO) vet ./...

.PHONY: fmt
fmt: ## Format all Go source files
	$(GO) fmt ./...

.PHONY: check
check: fmt vet lint test ## Run fmt, vet, lint, and test together (pre-commit gate)

## ---- Dependencies -----------------------------------------------------------

.PHONY: tidy
tidy: ## Tidy and verify go.mod/go.sum
	$(GO) mod tidy
	$(GO) mod verify

## ---- Install / Deploy -------------------------------------------------------

.PHONY: install
install: build ## Install both binaries to /usr/local/bin (local machine)
	sudo install -m 0755 $(BIN_DIR)/homelabctl /usr/local/bin/homelabctl
	sudo install -m 0755 $(BIN_DIR)/homelabctld /usr/local/bin/homelabctld

# Pushes the binary and restarts the user service. The mv swaps the file in
# one step, so the service never runs a partial copy. One-time Pi setup
# (unit, linger, ping sysctl) is manual: see deploy/README.md.
# Restarting while the sentinel is down shuts managed nodes down ~30s later.
.PHONY: deploy
deploy: build-pi ## Push the daemon to the Pi and restart it
	$(SSH) $(PI_HOST) 'mkdir -p ~/.local/bin'
	$(SCP) $(PI_BIN) $(PI_HOST):.local/bin/homelabctld.new
	$(SSH) $(PI_HOST) 'mv ~/.local/bin/homelabctld.new ~/.local/bin/homelabctld && \
		~/.local/bin/homelabctld --version && \
		systemctl --user restart homelabctld'

# Copies the real config (gitignored) with mode 0600. Restart separately.
.PHONY: deploy-config
deploy-config: ## Copy deploy/config.yaml to ~/.config/homelabctl/ on the Pi
	$(SSH) $(PI_HOST) 'mkdir -p ~/.config/homelabctl'
	$(SCP) deploy/config.yaml $(PI_HOST):.config/homelabctl/config.yaml.new
	$(SSH) $(PI_HOST) 'install -m 0600 ~/.config/homelabctl/config.yaml.new ~/.config/homelabctl/config.yaml && \
		rm ~/.config/homelabctl/config.yaml.new'

## ---- Housekeeping -----------------------------------------------------------

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf $(BIN_DIR) $(DIST_DIR) coverage.out coverage.html

.PHONY: run-daemon
run-daemon: build-daemon ## Build and run the daemon locally against ./config.yaml
	$(BIN_DIR)/homelabctld --config ./config.yaml

.PHONY: version
version: ## Print the version string that will be embedded in binaries
	@echo "$(VERSION) ($(COMMIT), $(BUILD_DATE))"

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
