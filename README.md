# jellysink

Media library maintenance for Jellyfin and Plex. Finds duplicate files, validates naming conventions, and cleans up your libraries automatically.

## What it does

jellysink scans your media libraries to find:
- Duplicate movies and TV episodes (keeps highest quality)
- Files that don't match Jellyfin/Plex naming conventions
- Scene release patterns and codec information

It runs scans on a schedule and presents everything in an interactive TUI where you can review and approve deletions.

## Installation

One-line install:

```bash
curl -sSL https://raw.githubusercontent.com/Nomadcxx/jellysink/main/install.sh | sudo bash
```

Or clone and run:

```bash
git clone https://github.com/Nomadcxx/jellysink
cd jellysink
sudo ./install.sh
```

Requirements: Go 1.21+, git

## Usage

Launch the interactive menu:

```bash
sudo jellysink
```

The TUI lets you:
- Add and remove library paths for movies and TV shows
- Configure scan frequency (daily, weekly, biweekly)
- Enable or disable the automatic daemon
- Run manual scans and view reports
- Review duplicates and approve deletions

CLI commands for automation:

```bash
sudo jellysink scan              # Run headless scan
jellysink view <report>          # View a report
sudo jellysink clean <report>    # Clean from a report
jellysink version                # Show version
```

The daemon runs via systemd and generates reports that launch the TUI for review. All deletions require explicit approval.

## Configuration

jellysink stores config at `~/.config/jellysink/config.toml`. The TUI handles all configuration through its menus, but you can edit manually if needed:

```toml
[libraries.movies]
paths = ["/path/to/movies", "/another/path/movies"]

[libraries.tv]
paths = ["/path/to/tv"]

[daemon]
scan_frequency = "weekly"
```

## Naming conventions

jellysink expects media to follow Jellyfin/Plex standards:

Movies:
```
Movies/Movie Name (2024)/Movie Name (2024).mkv
```

TV Shows:
```
TV Shows/Show Name (2010)/Season 01/Show Name (2010) S01E01.mkv
```

Files that don't match get flagged in compliance reports with suggested fixes.

## How duplicates work

When jellysink finds multiple copies of the same content, it scores them by:
- Resolution (4K > 1080p > 720p)
- Codec (H.265 > H.264)
- File size
- Audio quality

The highest-scoring file is marked as "KEEP" and others are marked for deletion. You review and approve each deletion in the TUI.

## Safety features

- Protected system paths (won't delete from /usr, /etc, etc.)
- 3TB per-operation size limit
- File ownership preservation (prevents root takeover when running with sudo)
- Operation logging for audit trails
- Dry-run mode for testing

## Why sudo

jellysink needs root privileges to:
- Control systemd services (enable/disable the daemon)
- Delete files from any location in your media libraries

File ownership is preserved during all operations, so your libraries stay owned by your user account even when running as root.

## Development

```bash
go test ./...                    # Run tests
go build ./cmd/jellysink/        # Build main binary
go build ./cmd/installer/        # Build installer
```

## License

MIT
