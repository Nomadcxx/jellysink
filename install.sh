#!/bin/bash
# jellysink installation script
# Builds and runs the TUI installer

set -e

echo "jellysink installer"
echo

# Check for Go
if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed or not in PATH"
    echo "Please install Go from https://golang.org/dl/"
    exit 1
fi

# Build the installer
echo "Building installer..."
go build -o install-jellysink ./cmd/installer/

# Run the installer
echo "Running installer..."
sudo ./install-jellysink

# Cleanup
rm -f install-jellysink

echo
echo "Installation complete!"
