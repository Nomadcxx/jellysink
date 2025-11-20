#!/bin/bash
# jellysink uninstallation script

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${RED}Jellysink Uninstaller${NC}"
echo "======================================"
echo

# Detect installation type
if [ -f "/usr/local/bin/jellysink" ]; then
    INSTALL_PREFIX="/usr/local"
    SYSTEMD_DIR="/etc/systemd/system"
    SUDO_CMD="sudo"
    echo -e "${YELLOW}System-wide installation detected${NC}"
elif [ -f "$HOME/.local/bin/jellysink" ]; then
    INSTALL_PREFIX="$HOME/.local"
    SYSTEMD_DIR="$HOME/.config/systemd/user"
    SUDO_CMD=""
    echo -e "${YELLOW}User installation detected${NC}"
else
    echo -e "${RED}No jellysink installation found${NC}"
    exit 1
fi

# Confirm
read -p "Are you sure you want to uninstall jellysink? (y/N) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Uninstall cancelled."
    exit 0
fi

# Stop and disable timer
echo "Stopping and disabling timer..."
if [ -n "$SUDO_CMD" ]; then
    $SUDO_CMD systemctl stop jellysink.timer 2>/dev/null || true
    $SUDO_CMD systemctl disable jellysink.timer 2>/dev/null || true
else
    systemctl --user stop jellysink.timer 2>/dev/null || true
    systemctl --user disable jellysink.timer 2>/dev/null || true
fi

# Remove binaries
echo "Removing binaries..."
$SUDO_CMD rm -f "$INSTALL_PREFIX/bin/jellysink"
$SUDO_CMD rm -f "$INSTALL_PREFIX/bin/jellysinkd"

# Remove systemd files
echo "Removing systemd files..."
$SUDO_CMD rm -f "$SYSTEMD_DIR/jellysink.service"
$SUDO_CMD rm -f "$SYSTEMD_DIR/jellysink.timer"

# Reload systemd
if [ -n "$SUDO_CMD" ]; then
    $SUDO_CMD systemctl daemon-reload
else
    systemctl --user daemon-reload
fi

echo
echo -e "${GREEN}Uninstall complete!${NC}"
echo
echo "Configuration and reports preserved at:"
echo "  Config: $HOME/.config/jellysink/"
echo "  Reports: $HOME/.local/share/jellysink/"
echo
echo "To remove all data:"
echo "  rm -rf $HOME/.config/jellysink"
echo "  rm -rf $HOME/.local/share/jellysink"
echo
