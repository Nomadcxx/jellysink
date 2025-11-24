# Scanning Progress Implementation Plan

**Status**: Partially Implemented  
**Created**: 2025-11-23  
**Last Updated**: 2025-11-23

## Overview

This document tracks the implementation of the comprehensive scanning progress system for jellysink, including a real-time TUI with viewport, scrolling logs, ETA calculations, and live statistics.

---

## Original Comprehensive Design Specification

### Goals
1. **Real-time progress reporting** during library scans
2. **Cancellable operations** with context support
3. **Full TUI interface** with viewport and scrolling logs
4. **Live statistics** (duplicates found, compliance issues, errors)
5. **ETA calculations** based on actual file processing rates
6. **Stage-based rendering** (counting files → scanning → analyzing → complete)

### Architecture Components

#### 1. Backend Progress Infrastructure (`internal/scanner/`)
- **ScanProgress struct**: Carries all progress data
  - Operation tracking (movies/TV/compliance)
  - Stage tracking (counting/scanning/analyzing/complete)
  - Current/Total counters
  - Percentage calculation (0-100)
  - Human-readable messages
  - Live statistics (duplicates, compliance issues, files processed)
  - Timing (start time, elapsed seconds, ETA)
  
- **ProgressReporter**: Helper for sending updates
  - Per-operation reporters
  - Automatic percentage calculation
  - Start/Update/Complete lifecycle
  
- **Progress-aware scanners**: All scan functions report progress
  - `ScanMoviesWithProgress()`
  - `ScanTVShowsWithProgress()`
  - `ScanMovieComplianceWithProgress()`
  - `ScanTVComplianceWithProgress()`

#### 2. Orchestration (`internal/scanner/orchestrator.go`)
- **RunFullScan()**: Coordinates all scan stages
  - Sequential execution with cancellation checks
  - Progress channel passing to all stages
  - Result aggregation
  - Statistics calculation

#### 3. Daemon Integration (`internal/daemon/daemon.go`)
- **RunScanWithProgress()**: Entry point for scans
  - Context cancellation support
  - Progress channel management
  - Report generation and saving

#### 4. TUI Scanning View (`internal/ui/`)
- **ViewScanning mode**: Full-screen scanning interface
- **Viewport integration**: Scrollable log buffer
- **Components**:
  - ASCII header with jellysink branding
  - Current operation/stage display
  - Progress bar with percentage (0-100%)
  - Live statistics panel:
    - Files processed
    - Duplicates found
    - Compliance issues detected
    - Errors encountered
  - ETA calculation and display
  - Scrolling log viewport (up to 1000 lines)
  - Help footer (navigation, cancel)

---

## Current Implementation Status

### ✅ COMPLETED Components

#### 1. Backend Progress Infrastructure
**File**: `internal/scanner/progress.go`  
**Status**: ✅ Complete and tested

- [x] `ScanProgress` struct with all fields:
  ```go
  type ScanProgress struct {
      Operation        string  // "scanning_movies", "scanning_tv", etc.
      Stage            string  // "counting_files", "scanning", "analyzing", "complete"
      Current          int     // Current file/item number
      Total            int     // Total files/items
      Percentage       float64 // 0-100
      Message          string  // Human-readable status
      
      // Statistics
      DuplicatesFound  int
      ComplianceIssues int
      FilesProcessed   int
      
      // Timing
      StartTime        time.Time
      ElapsedSeconds   int
  }
  ```

- [x] `ProgressReporter` helper class
- [x] `CountVideoFiles()` for accurate totals
- [x] Tests in `progress_test.go`

#### 2. Progress-Aware Scanners
**Files**: `internal/scanner/{movies,tvshows,compliance}.go`  
**Status**: ✅ Complete and tested

- [x] `ScanMoviesWithProgress()` - Reports per-file progress
- [x] `ScanTVShowsWithProgress()` - Reports per-episode progress
- [x] `ScanMovieComplianceWithProgress()` - Reports compliance checks
- [x] `ScanTVComplianceWithProgress()` - Reports TV compliance
- [x] All functions use `ProgressReporter`
- [x] All functions count files first, then report accurate percentages

#### 3. Orchestration Layer
**File**: `internal/scanner/orchestrator.go`  
**Status**: ✅ Complete and tested

- [x] `RunFullScan()` function
- [x] Sequential stage execution (movies → TV → compliance checks)
- [x] Context cancellation support
- [x] Progress channel threading to all stages
- [x] Result aggregation
- [x] Statistics calculation

#### 4. Daemon Integration
**File**: `internal/daemon/daemon.go`  
**Status**: ✅ Complete

- [x] `RunScanWithProgress(ctx, progressCh)` method
- [x] Calls `scanner.RunFullScan()` with progress channel
- [x] Report generation and saving
- [x] JSON report format

#### 5. CLI Progress Display
**File**: `cmd/jellysink/main.go` (`runScan` function)  
**Status**: ✅ Complete (simple terminal output)

- [x] Consumes progress channel
- [x] Displays operation and message
- [x] Shows percentage
- [x] Terminal-friendly output (not TUI)

#### 6. Basic TUI Scanning Model
**File**: `internal/ui/menu.go` (`ScanningModel`)  
**Status**: ⚠️ **PARTIALLY COMPLETE** (simplified version)

**What's implemented:**
- [x] Context cancellation support
- [x] Progress channel (buffered to 100)
- [x] `waitForProgress()` message handler
- [x] `runScan()` background execution
- [x] Progress message handling in `Update()`
- [x] Simple progress bar in `View()`
- [x] Log buffer (last 10 messages)
- [x] Report loading and transition on completion

**What's MISSING (from comprehensive spec):**
- [ ] **Viewport integration** - Currently simple string building, no viewport
- [ ] **Scrolling logs** - Only shows last 10 messages (truncated), should scroll up to 1000
- [ ] **ETA calculation** - No ETA displayed
- [ ] **Live statistics panel** - No duplicates/compliance stats shown during scan
- [ ] **Stage indicators** - No visual stage progression
- [ ] **Keyboard navigation** - No up/down scrolling through logs
- [ ] **Better layout** - Current view is basic, not using full viewport potential

---

## 🚨 GAPS: What Still Needs Implementation

### 1. Full TUI Scanning View with Viewport

**File**: `internal/ui/menu.go` (or new `internal/ui/scanning.go`)

#### Current Simplified Implementation
```go
type ScanningModel struct {
    config       *config.Config
    width        int
    height       int
    ctx          context.Context
    cancel       context.CancelFunc
    progressCh   chan scanner.ScanProgress
    progress     float64      // 0-100
    currentPhase string
    logMessages  []string     // Only last 10!
}
```

#### Needed Comprehensive Implementation
```go
type ScanningModel struct {
    config       *config.Config
    width        int
    height       int
    ctx          context.Context
    cancel       context.CancelFunc
    progressCh   chan scanner.ScanProgress
    
    // Progress state
    currentProgress scanner.ScanProgress
    
    // Log viewport
    viewport     viewport.Model   // Bubble Tea viewport
    logBuffer    []string         // Up to 1000 lines
    
    // Statistics (live updates)
    stats        ScanStats
    
    // ETA calculation
    startTime    time.Time
    eta          time.Duration
    
    // Keyboard state
    scrollOffset int
}

type ScanStats struct {
    FilesProcessed   int
    DuplicatesFound  int
    ComplianceIssues int
    ErrorsEncountered int
}
```

#### Required Changes

**A. Add Viewport for Scrolling Logs**
```go
import "github.com/charmbracelet/bubbles/viewport"

func NewScanningModel(cfg *config.Config) ScanningModel {
    ctx, cancel := context.WithCancel(context.Background())
    
    vp := viewport.New(80, 20)  // Initial size
    vp.SetContent("")
    
    return ScanningModel{
        config:      cfg,
        ctx:         ctx,
        cancel:      cancel,
        progressCh:  make(chan scanner.ScanProgress, 100),
        viewport:    vp,
        logBuffer:   []string{},
        startTime:   time.Now(),
    }
}
```

**B. Update Message Handling**
```go
func (m ScanningModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmd tea.Cmd
    
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "ctrl+c":
            m.cancel()
            close(m.progressCh)
            return m, tea.Quit
        case "up", "k":
            m.viewport, cmd = m.viewport.Update(msg)
            return m, cmd
        case "down", "j":
            m.viewport, cmd = m.viewport.Update(msg)
            return m, cmd
        case "pgup":
            m.viewport, cmd = m.viewport.Update(msg)
            return m, cmd
        case "pgdown":
            m.viewport, cmd = m.viewport.Update(msg)
            return m, cmd
        }
        
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
        
        // Resize viewport (leave space for header, progress, stats, footer)
        headerHeight := 10  // ASCII art + progress bar + stats
        footerHeight := 2   // Help text
        m.viewport.Width = msg.Width - 4
        m.viewport.Height = msg.Height - headerHeight - footerHeight
        m.viewport.SetContent(m.renderLogs())
        return m, nil
        
    case scanner.ScanProgress:
        // Store full progress object
        m.currentProgress = msg
        
        // Update statistics
        m.stats.FilesProcessed = msg.FilesProcessed
        m.stats.DuplicatesFound = msg.DuplicatesFound
        m.stats.ComplianceIssues = msg.ComplianceIssues
        
        // Calculate ETA
        if msg.Total > 0 && msg.Current > 0 {
            elapsed := time.Since(m.startTime)
            rate := float64(msg.Current) / elapsed.Seconds()
            remaining := msg.Total - msg.Current
            m.eta = time.Duration(float64(remaining)/rate) * time.Second
        }
        
        // Add to log buffer (max 1000 lines)
        logEntry := fmt.Sprintf("[%02d:%02d] [%s] %s",
            msg.ElapsedSeconds/60,
            msg.ElapsedSeconds%60,
            msg.Operation,
            msg.Message)
        
        m.logBuffer = append(m.logBuffer, logEntry)
        if len(m.logBuffer) > 1000 {
            m.logBuffer = m.logBuffer[1:]
        }
        
        // Update viewport content
        m.viewport.SetContent(m.renderLogs())
        m.viewport.GotoBottom()  // Auto-scroll to latest
        
        return m, m.waitForProgress
        
    case scanStatusMsg:
        // Scan complete - transition to report
        close(m.progressCh)
        
        if msg.err != nil {
            return m, tea.Printf("Scan failed: %v", msg.err)
        }
        
        report, err := loadReportJSON(msg.reportPath)
        if err != nil {
            return m, tea.Printf("Failed to load report: %v", err)
        }
        
        reportModel := NewModel(report)
        return reportModel, func() tea.Msg {
            return tea.WindowSizeMsg{Width: m.width, Height: m.height}
        }
    }
    
    return m, nil
}
```

**C. Enhanced View Rendering**
```go
func (m ScanningModel) View() string {
    if m.width == 0 || m.height == 0 {
        return "Initializing..."
    }
    
    var content strings.Builder
    
    // 1. ASCII Header
    content.WriteString(FormatASCIIHeader())
    content.WriteString("\n\n")
    
    // 2. Current Operation and Stage
    operationText := strings.ToUpper(m.currentProgress.Operation)
    stageText := m.currentProgress.Stage
    
    headerStyle := lipgloss.NewStyle().
        Bold(true).
        Foreground(RAMARed).
        Align(lipgloss.Center).
        Width(m.width - 8)
    
    content.WriteString(headerStyle.Render(
        fmt.Sprintf("%s - %s", operationText, stageText)))
    content.WriteString("\n\n")
    
    // 3. Progress Bar (60 chars wide)
    barWidth := int(m.currentProgress.Percentage * 60 / 100)
    if barWidth > 60 {
        barWidth = 60
    }
    if barWidth < 0 {
        barWidth = 0
    }
    
    progressBar := strings.Repeat("█", barWidth) + strings.Repeat("░", 60-barWidth)
    progressStyle := lipgloss.NewStyle().
        Foreground(RAMARed).
        Align(lipgloss.Center).
        Width(m.width - 8)
    
    progressText := fmt.Sprintf("[%s] %.1f%%", progressBar, m.currentProgress.Percentage)
    if m.currentProgress.Total > 0 {
        progressText += fmt.Sprintf(" (%d/%d files)", m.currentProgress.Current, m.currentProgress.Total)
    }
    
    content.WriteString(progressStyle.Render(progressText))
    content.WriteString("\n\n")
    
    // 4. Live Statistics Panel
    statsStyle := lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        BorderForeground(RAMARed).
        Padding(0, 2).
        Width(m.width - 10)
    
    statsContent := fmt.Sprintf(
        "Files Processed: %d  |  Duplicates: %d  |  Compliance Issues: %d  |  Errors: %d",
        m.stats.FilesProcessed,
        m.stats.DuplicatesFound,
        m.stats.ComplianceIssues,
        m.stats.ErrorsEncountered,
    )
    
    content.WriteString(statsStyle.Render(statsContent))
    content.WriteString("\n\n")
    
    // 5. ETA Display
    if m.eta > 0 {
        etaStyle := lipgloss.NewStyle().
            Foreground(ColorInfo).
            Align(lipgloss.Center).
            Width(m.width - 8)
        
        etaText := fmt.Sprintf("Estimated Time Remaining: %s", m.eta.Round(time.Second))
        content.WriteString(etaStyle.Render(etaText))
        content.WriteString("\n\n")
    }
    
    // 6. Scrolling Log Viewport
    logTitleStyle := lipgloss.NewStyle().
        Bold(true).
        Foreground(ColorInfo)
    
    content.WriteString(logTitleStyle.Render("Activity Log:"))
    content.WriteString("\n")
    content.WriteString(m.viewport.View())
    content.WriteString("\n")
    
    // 7. Help Footer
    helpStyle := lipgloss.NewStyle().
        Foreground(lipgloss.Color("240")).
        Align(lipgloss.Center).
        Width(m.width - 8)
    
    helpText := "↑/↓: Scroll logs  •  PgUp/PgDn: Page scroll  •  Ctrl+C: Cancel"
    content.WriteString(helpStyle.Render(helpText))
    
    // Wrap in main container
    mainStyle := lipgloss.NewStyle().
        Padding(1, 2).
        Width(m.width)
    
    return mainStyle.Render(content.String())
}

func (m ScanningModel) renderLogs() string {
    return strings.Join(m.logBuffer, "\n")
}
```

### 2. Error Tracking

**Currently Missing**: Error counter and error message collection

**Need to add**:
- `Errors []string` field in `ScanProgress`
- Error reporting in scanner functions
- Error display in stats panel
- Red highlighting for errors in log

### 3. Stage Progression Indicators

**Visual enhancement**: Show completed stages with checkmarks

Example:
```
✓ Counting files (complete)
✓ Scanning movies (complete)  
→ Scanning TV shows (in progress - 45%)
  Compliance check (pending)
  Generating report (pending)
```

### 4. Performance Optimizations

**Current issues**:
- Log buffer grows unbounded during long scans
- Viewport re-renders entire content on every update

**Solutions**:
- Implement circular buffer for logs (fixed 1000 lines)
- Use viewport content diffing
- Rate-limit progress updates (max 10 updates/sec)

---

## Implementation Priority

### Phase 1: Critical (Minimum Viable)
1. ✅ Backend progress infrastructure (DONE)
2. ✅ Progress-aware scanners (DONE)
3. ✅ Orchestration layer (DONE)
4. ✅ Daemon integration (DONE)
5. ⚠️ **Basic TUI scanning (PARTIAL - needs viewport)**

### Phase 2: Enhanced UX (Current Gap)
1. ❌ **Add viewport for scrolling logs**
2. ❌ **Add ETA calculation and display**
3. ❌ **Add live statistics panel**
4. ❌ **Add keyboard navigation (up/down/pgup/pgdn)**
5. ❌ **Add stage progression indicators**

### Phase 3: Polish
1. ❌ Error tracking and display
2. ❌ Performance optimizations (rate limiting, circular buffer)
3. ❌ Color-coded log levels (info/warning/error)
4. ❌ Sound/notification on completion
5. ❌ Scan history view (previous scans)

---

## Testing Checklist

### Backend (✅ Complete)
- [x] All 72 tests passing
- [x] Progress percentages accurate
- [x] Context cancellation works
- [x] Multi-library scanning works

### TUI (⚠️ Needs Testing)
- [ ] Viewport scrolling works
- [ ] ETA calculation is accurate
- [ ] Statistics update in real-time
- [ ] Window resize handled correctly
- [ ] Keyboard navigation responsive
- [ ] Long scans (>1000 log lines) don't crash
- [ ] Cancellation cleans up properly

### Integration (⚠️ Needs Testing)
- [ ] CLI scan works end-to-end
- [ ] TUI scan works end-to-end
- [ ] Daemon scheduled scans work
- [ ] Reports generated correctly
- [ ] Report view transition smooth

---

## Known Issues

### 1. Progress Channel Blocking (Fixed 2025-11-23)
**Issue**: Channel buffer of 10 was too small, causing deadlock  
**Solution**: Increased to 100, added continuous `waitForProgress` calls  
**Status**: ✅ Fixed

### 2. TUI Not Using Real Progress (Fixed 2025-11-23)
**Issue**: Menu.go ScanningModel was using old fake progress  
**Solution**: Updated to use `RunScanWithProgress()` and handle real messages  
**Status**: ✅ Fixed

### 3. Viewport Not Implemented (Current)
**Issue**: Logs truncated to 10 lines, no scrolling  
**Solution**: Need to implement Phase 2 enhancements (see above)  
**Status**: ❌ Not started

### 4. No ETA Calculation (Current)
**Issue**: Users can't estimate scan duration  
**Solution**: Implement ETA based on processing rate  
**Status**: ❌ Not started

---

## Files Reference

### Backend
- `internal/scanner/progress.go` - Progress types and helpers
- `internal/scanner/orchestrator.go` - RunFullScan coordination
- `internal/scanner/movies.go` - Movie scanning with progress
- `internal/scanner/tvshows.go` - TV scanning with progress
- `internal/scanner/compliance.go` - Compliance checking with progress
- `internal/daemon/daemon.go` - Daemon scan execution

### Frontend
- `internal/ui/ui.go` - Base TUI model and ViewMode enum
- `internal/ui/menu.go` - Menu and ScanningModel (needs enhancement)
- `cmd/jellysink/main.go` - CLI entry point and simple progress display

### Tests
- `internal/scanner/progress_test.go` - Progress infrastructure tests
- `internal/scanner/*_test.go` - Scanner function tests (72 total)

---

## Next Steps

**Immediate (To Get Full TUI Working)**:
1. Add viewport import to `menu.go`
2. Add viewport field to `ScanningModel`
3. Implement log buffer (1000 lines)
4. Update `Update()` to populate viewport
5. Update `View()` to render viewport
6. Add keyboard handlers (up/down/pgup/pgdn)

**Then**:
7. Add ETA calculation
8. Add statistics panel
9. Add stage progression indicators
10. Test with real library scans
11. Fix any performance issues

**Finally**:
12. Add error tracking
13. Add color-coded logs
14. Polish layout and styling
15. Document TUI keyboard shortcuts

---

## Conclusion

**Current State**: We have a **solid backend** with full progress reporting infrastructure, but the **TUI is only showing a simplified version** without viewport, scrolling, ETA, or live statistics.

**What Works**:
- ✅ Progress reporting from all scanners
- ✅ Context cancellation
- ✅ Report generation
- ✅ Basic progress bar in TUI
- ✅ CLI progress display

**What's Missing**:
- ❌ Viewport for scrolling logs (only 10 lines visible)
- ❌ ETA calculation
- ❌ Live statistics panel
- ❌ Keyboard navigation
- ❌ Stage indicators

**Estimated Work**: ~4-6 hours to implement Phase 2 enhancements and get the full comprehensive TUI working as originally designed.
