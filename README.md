# jellysink

**Automated media library maintenance for Jellyfin/Plex**

jellysink is a Go-based daemon and TUI that automatically scans your media libraries for duplicates, generates reports, and helps you clean up your collection. Built for users who want a "set it and forget it" solution to keep their media library pristine.

## Features

- 🔍 **Smart Duplicate Detection**
  - Fuzzy matching handles capitalization, punctuation, release groups
  - Quality-aware: keeps highest resolution/best source
  - Empty folder detection

- 📺 **Multi-Library Support**
  - Movies and TV shows
  - Multiple storage paths per library type
  - Scans `/mnt/STORAGE*/MOVIES`, `/mnt/STORAGE*/TVSHOWS`, etc.

- 📊 **Automated Reporting**
  - Weekly scans via systemd timer
  - Timestamped reports with summaries
  - Top offenders list
  - Space-to-free calculations

- 🎨 **Beautiful TUI**
  - Bubble Tea interface
  - Table view for duplicates
  - In-app deletion with confirmation
  - Config editor for library paths

- ⚙️ **Jellyfin/Plex Compliant**
  - Enforces `Movie (Year)` format
  - TV shows: `Show (Year)/Season ##/S##E##`
  - Detects and can rename release group folders

## Installation

**Requirements:** Go 1.21+

```bash
git clone https://github.com/Nomadcxx/jellysink.git
cd jellysink
make build
sudo make install
```

## Quick Start

```bash
# Launch TUI to configure libraries
jellysink

# Scan manually
jellysink scan --library movies
jellysink scan --library tv

# View latest report
jellysink review

# Clean duplicates from report
jellysink clean --report-id 20250119_143025

# Enable automated scanning
sudo systemctl enable --now jellysink.timer
```

## Configuration

Config file: `~/.config/jellysink/config.toml`

```toml
[libraries.movies]
paths = [
  "/mnt/STORAGE1/MOVIES",
  "/mnt/STORAGE5/MOVIES",
  "/mnt/STORAGE10/MOVIES"
]

[libraries.tv]
paths = [
  "/mnt/STORAGE1/TVSHOWS",
  "/mnt/STORAGE5/TVSHOWS",
  "/mnt/STORAGE10/TVSHOWS"
]

[daemon]
scan_frequency = "weekly"  # daily, weekly, biweekly
report_on_complete = true
```

## How It Works

1. **Daemon runs on schedule** (weekly by default)
2. **Scans configured libraries** for duplicates using fuzzy matching
3. **Generates timestamped report** in `~/.local/share/jellysink/scan_results/`
4. **Launches TUI notification** showing summary and top offenders
5. **User reviews and confirms** deletion via TUI
6. **Safe deletion** with protected paths and confirmation

## Duplicate Detection Logic

### Movies
- Normalizes names: case-insensitive, punctuation removal
- Fuzzy matching: 85% similarity threshold (SequenceMatcher)
- Groups by normalized name + year
- Keeps largest/non-empty version
- Resolution-aware: only marks same quality as duplicate

### TV Shows
- Extracts `S##E##` pattern from filenames
- Groups by show name + season + episode
- Quality scoring: resolution > source > codec > audio
- Keeps highest quality version

### Examples

**Detected as duplicates:**
- `Movie (2024)` vs `Movie.2024.1080p.BluRay.x264-GROUP`
- `Lost In Translation (2003)` vs `Lost in Translation (2003)`
- `The Nun II (2023)` vs `The Nun 2 (2023)` (roman numerals)
- Empty folders vs actual content

**NOT duplicates:**
- `Movie (2024)` 4K version vs 1080p version (different quality kept)
- `Show S01` on Storage1 vs `Show S02` on Storage2 (different seasons)

## Development

See [CLAUDE.md](CLAUDE.md) for architecture details and development guidelines.

```bash
# Run tests
go test ./...

# Build
make build

# Run TUI
./jellysink

# Check ready work
bd ready
```

## License

MIT License - see [LICENSE](LICENSE)

## Acknowledgments

- Inspired by [moonbit](https://github.com/Nomadcxx/moonbit) for system cleaning
- Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) TUI framework
- Uses [bd (beads)](https://github.com/steveyegge/beads) for issue tracking
