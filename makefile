MODULE      := github.com/pspiagicw/homelabctl
CLI_PKG     := ./cmd/homelabctl
DAEMON_PKG  := ./cmd/homelabctld

BIN_DIR     := bin
DIST_DIR    := dist

VERSION     := $(shell git describe --tags --dirty --always 2>/dev/null || echo "dev")
COMMIT      := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_DATE  := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS     := -X '$(MODULE)/pkg/version.Version=$(VERSION)' \
               -X '$(MODULE)/pkg/version.Commit=$(COMMIT)' \
               -X '$(MODULE)/pkg/version.BuildDate=$(BUILD_DATE)'

GO          := go
GOFLAGS     :=

# Raspberry Pi cross-compile target. Override at the command line if needed,
# e.g. `make deploy-build PI_GOARCH=arm PI_GOARM=7` for 32-bit Pi models.
PI_GOOS     := linux
PI_GOARCH   := arm64

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

.PHONY: deploy-build
deploy-build: ## Cross-compile both binaries for the Raspberry Pi (falcon-control)
	@mkdir -p $(DIST_DIR)
	GOOS=$(PI_GOOS) GOARCH=$(PI_GOARCH) GOARM=$(PI_GOARM) $(GO) build \
		-ldflags "$(LDFLAGS)" -o $(DIST_DIR)/homelabctl_$(PI_GOOS)_$(PI_GOARCH) $(CLI_PKG)
	GOOS=$(PI_GOOS) GOARCH=$(PI_GOARCH) GOARM=$(PI_GOARM) $(GO) build \
		-ldflags "$(LDFLAGS)" -o $(DIST_DIR)/homelabctld_$(PI_GOOS)_$(PI_GOARCH) $(DAEMON_PKG)
	@echo "Built for $(PI_GOOS)/$(PI_GOARCH) -> $(DIST_DIR)/"

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

.PHONY: install-service
install-service: ## Install and enable the homelabctld systemd unit
	sudo install -m 0644 deploy/systemd/homelabctld.service /etc/systemd/system/homelabctld.service
	sudo systemctl daemon-reload
	sudo systemctl enable homelabctld

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
