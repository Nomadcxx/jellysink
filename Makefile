.PHONY: build install clean test daemon all

# Build main TUI/CLI binary
build:
	@echo "Building jellysink..."
	@go build -o jellysink cmd/jellysink/main.go

# Build daemon binary
daemon:
	@echo "Building jellysinkd..."
	@go build -o jellysinkd cmd/jellysinkd/main.go

# Build both binaries
all: build daemon

# Install binaries to system
install: all
	@echo "Installing binaries..."
	@sudo cp jellysink /usr/local/bin/jellysink
	@sudo cp jellysinkd /usr/local/bin/jellysinkd
	@sudo chmod +x /usr/local/bin/jellysink
	@sudo chmod +x /usr/local/bin/jellysinkd
	@echo "Installing systemd service and timer..."
	@sudo cp systemd/jellysink.service /etc/systemd/system/
	@sudo cp systemd/jellysink.timer /etc/systemd/system/
	@sudo systemctl daemon-reload
	@echo "Installation complete!"
	@echo "Enable timer with: sudo systemctl enable --now jellysink.timer"

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
	@echo "Uninstalling..."
	@sudo systemctl stop jellysink.timer || true
	@sudo systemctl disable jellysink.timer || true
	@sudo rm -f /usr/local/bin/jellysink
	@sudo rm -f /usr/local/bin/jellysinkd
	@sudo rm -f /etc/systemd/system/jellysink.service
	@sudo rm -f /etc/systemd/system/jellysink.timer
	@sudo systemctl daemon-reload
	@echo "Uninstall complete!"
