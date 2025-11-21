.PHONY: build install clean test daemon all check validate installer

# Version information
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')

# Installation paths (can be overridden with PREFIX)
PREFIX ?= /usr/local
BINDIR := $(PREFIX)/bin
SYSTEMD_DIR := /etc/systemd/system

# Build flags
LDFLAGS := -X main.version=$(VERSION) \
           -X main.commit=$(COMMIT) \
           -X main.buildTime=$(BUILD_TIME) \
           -s -w

# Validation target - run before build
validate:
	@echo "Validating environment..."
	@command -v go >/dev/null 2>&1 || (echo "Error: Go is not installed" && exit 1)
	@echo "Go version: $$(go version)"
	@echo "Checking go.mod..."
	@test -f go.mod || (echo "Error: go.mod not found" && exit 1)
	@echo "Validation passed!"

# Build main TUI/CLI binary
build: validate
	@echo "Building jellysink $(VERSION)..."
	@go build -ldflags "$(LDFLAGS)" -o jellysink ./cmd/jellysink/
	@echo "Built jellysink successfully"

# Build daemon binary
daemon: validate
	@echo "Building jellysinkd $(VERSION)..."
	@go build -ldflags "$(LDFLAGS)" -o jellysinkd ./cmd/jellysinkd/
	@echo "Built jellysinkd successfully"

# Build installer
installer: validate
	@echo "Building install-jellysink $(VERSION)..."
	@go build -ldflags "$(LDFLAGS)" -o install-jellysink ./cmd/installer/
	@echo "Built install-jellysink successfully"

# Build all binaries
all: build daemon installer

# Verify binaries work after building
check: all
	@echo "Verifying binaries..."
	@./jellysink version 2>/dev/null || ./jellysink help >/dev/null || echo "Warning: jellysink binary verification failed"
	@test -x jellysinkd && echo "jellysinkd is executable" || echo "Error: jellysinkd not executable"
	@echo "Check complete!"

# Install binaries to system (requires root for system-wide install)
install: all check
	@echo "Installing to $(BINDIR)..."
	@install -d $(BINDIR)
	@install -m 755 jellysink $(BINDIR)/jellysink
	@install -m 755 jellysinkd $(BINDIR)/jellysinkd
	@install -m 755 install-jellysink $(BINDIR)/install-jellysink
	@echo "Installing systemd service and timer..."
	@install -d $(SYSTEMD_DIR)
	@install -m 644 systemd/jellysink.service $(SYSTEMD_DIR)/jellysink.service
	@install -m 644 systemd/jellysink.timer $(SYSTEMD_DIR)/jellysink.timer
	@systemctl daemon-reload 2>/dev/null || echo "Note: Could not reload systemd (not running as root?)"
	@echo ""
	@echo "Installation complete!"
	@echo "Binaries installed to: $(BINDIR)"
	@echo ""
	@echo "Next steps:"
	@echo "  1. Configure: edit ~/.config/jellysink/config.toml"
	@echo "  2. Enable timer: sudo systemctl enable --now jellysink.timer"
	@echo "  3. Manual scan: jellysink scan"

# Run tests
test:
	@echo "Running tests..."
	@go test -v ./...

# Run tests with coverage
coverage:
	@echo "Running tests with coverage..."
	@go test -cover ./...
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -f jellysink jellysinkd
	@rm -f coverage.out coverage.html
	@echo "Clean complete!"

# Uninstall from system
uninstall:
	@echo "Uninstalling jellysink..."
	@systemctl stop jellysink.timer 2>/dev/null || true
	@systemctl disable jellysink.timer 2>/dev/null || true
	@rm -f $(BINDIR)/jellysink
	@rm -f $(BINDIR)/jellysinkd
	@rm -f $(BINDIR)/install-jellysink
	@rm -f $(SYSTEMD_DIR)/jellysink.service
	@rm -f $(SYSTEMD_DIR)/jellysink.timer
	@systemctl daemon-reload 2>/dev/null || true
	@echo "Uninstall complete!"
	@echo "Config preserved at ~/.config/jellysink/"
