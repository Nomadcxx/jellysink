# jellysink

Automated media library maintenance for Jellyfin/Plex. Scans for duplicates and naming compliance issues.

## Features

- Duplicate detection with quality scoring
- Jellyfin/Plex naming compliance checking
- TUI for reviewing and cleaning
- Automated scheduled scans via systemd
- Comprehensive scene release pattern handling
- Safe deletion with protected paths and size limits

## Installation

```bash
# Clone the repository
git clone https://github.com/Nomadcxx/jellysink
cd jellysink

# Run the TUI installer
./install.sh
```

The installer will guide you through:
- Installing binaries to `/usr/local/bin`
- Creating configuration at `~/.config/jellysink/config.toml`
- Setting up systemd service and timer

## Configuration

Create `~/.config/jellysink/config.toml`:

```toml
[libraries.movies]
paths = ["/path/to/your/movies"]

[libraries.tv]
paths = ["/path/to/your/tvshows"]

[daemon]
scan_frequency = "weekly"  # daily, weekly, biweekly
```

## Usage

### Manual Scan
```bash
jellysink scan                    # Scan configured libraries
jellysink view <report.json>      # View report in TUI
jellysink clean <report.json>     # Clean duplicates
jellysink config                  # Show configuration
```

### Automated Scans

Enable the systemd timer for scheduled scans:

```bash
# System-wide
sudo systemctl enable --now jellysink.timer

# User service
systemctl --user enable --now jellysink.timer
```

The daemon will:
1. Scan libraries on schedule
2. Generate report
3. Send desktop notification
4. Launch TUI for review

## Naming Conventions

### Movies
```
Movies/
└── Movie Name (2024)/
    └── Movie Name (2024).mkv
```

### TV Shows
```
TV Shows/
└── Show Name (2010)/
    └── Season 01/
        └── Show Name (2010) S01E01.mkv
```

## Uninstall

```bash
./uninstall.sh
```

## Development

```bash
go test ./...      # Run tests
make clean         # Clean build artifacts
bd ready           # Check tasks (requires beads)
```

## License

MIT License - See LICENSE file.
