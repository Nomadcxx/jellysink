#!/bin/bash
# jellysink installation script

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${GREEN}Jellysink Installer${NC}"
echo "======================================"
echo

# Check if running as root for system install
if [ "$EUID" -eq 0 ]; then
    INSTALL_PREFIX="/usr/local"
    SYSTEMD_DIR="/etc/systemd/system"
    echo -e "${YELLOW}Installing system-wide...${NC}"
else
    INSTALL_PREFIX="$HOME/.local"
    SYSTEMD_DIR="$HOME/.config/systemd/user"
    echo -e "${YELLOW}Installing for current user...${NC}"
fi

# Create directories
echo "Creating directories..."
mkdir -p "$INSTALL_PREFIX/bin"
mkdir -p "$SYSTEMD_DIR"
mkdir -p "$HOME/.config/jellysink"
mkdir -p "$HOME/.local/share/jellysink/scan_results"

# Build binaries
echo "Building binaries..."
make all

# Install binaries
echo "Installing binaries to $INSTALL_PREFIX/bin..."
cp jellysink "$INSTALL_PREFIX/bin/jellysink"
cp jellysinkd "$INSTALL_PREFIX/bin/jellysinkd"
chmod +x "$INSTALL_PREFIX/bin/jellysink"
chmod +x "$INSTALL_PREFIX/bin/jellysinkd"

# Install systemd files
echo "Installing systemd service and timer..."
if [ "$EUID" -eq 0 ]; then
    cp systemd/jellysink.service "$SYSTEMD_DIR/"
    cp systemd/jellysink.timer "$SYSTEMD_DIR/"
    systemctl daemon-reload
else
    cp systemd/jellysink.service "$SYSTEMD_DIR/"
    cp systemd/jellysink.timer "$SYSTEMD_DIR/"
    systemctl --user daemon-reload
fi

# Check for config
if [ ! -f "$HOME/.config/jellysink/config.toml" ]; then
    echo -e "${YELLOW}No configuration found.${NC}"
    echo "Create config at: $HOME/.config/jellysink/config.toml"
    echo
    echo "Example config:"
    cat <<'EOF'
[libraries.movies]
paths = ["/path/to/your/movies"]

[libraries.tv]
paths = ["/path/to/your/tvshows"]

[daemon]
scan_frequency = "weekly"  # daily, weekly, biweekly
EOF
    echo
fi

echo
echo -e "${GREEN}Installation complete!${NC}"
echo
echo "Next steps:"
echo "1. Create config: $HOME/.config/jellysink/config.toml"
echo "2. Test scan:     jellysink scan"
if [ "$EUID" -eq 0 ]; then
    echo "3. Enable timer:  systemctl enable --now jellysink.timer"
    echo "4. Check status:  systemctl status jellysink.timer"
else
    echo "3. Enable timer:  systemctl --user enable --now jellysink.timer"
    echo "4. Check status:  systemctl --user status jellysink.timer"
fi
echo
