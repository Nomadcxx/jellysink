# Jellysink Backend-Frontend Integration Audit & Continuation Plan

## Executive Summary
A comprehensive audit of jellysink's backend capabilities versus frontend (TUI/CLI) integration reveals significant feature gaps. This document identifies all features that exist in the backend but are not yet accessible to users, and provides a detailed integration roadmap.

**Document Version:** 1.0
**Audit Date:** 2025-01-19
**Backend Branch:** main (0b70df8)

---

## 1. Backend Capabilities Audit

### 1.1 Scanner Package Features

#### ✅ Features Integrated
- Duplicate detection (movies/TV)
- Compliance checking
- TV show title conflict resolution
- Manual TV show renaming
- Basic backup management (create/verify/revert/list)

#### ❌ Features NOT Integrated

**Loose Files Management** (`internal/scanner/loose_files.go`)
- `ScanLooseFiles()` - Find videos not in proper Jellyfin structure
- `OrganizeLooseFiles()` - Auto-organize loose files into proper folder structure
- File classification (movies vs TV episodes)
- Suggested path generation
- Dry-run mode support
- **Impact:** High - Users can't auto-organize poorly structured libraries

**TV Show Title Resolution**
- `PreviewTVRename()` - Preview rename operations before execution (added yesterday)
- Detailed impact analysis (episode counts, collision warnings)
- Empty folder detection
- Duplicate target path detection
- **Impact:** Medium-High - Safely assess renames before committing

**Fuzzy Matching & Quality Scoring**
- Advanced duplicate quality comparison (resolution, source, codec, audio)
- Fuzzy name matching (85% similarity threshold)
- Release group marker removal
- Roman numeral conversion (II → 2)
- **Impact:** Low-Medium - Already used internally but not directly exposed

**Blacklist Management** (`internal/scanner/blacklist.go`)
- Blacklist pattern management from SRRDB
- Custom blacklist entries
- Blacklist application during scanning
- **Impact:** Medium - Improves detection accuracy

**Compliance Moon Scanning** (`internal/scanner/compliance_moon.go`)
- Enhanced compliance checking with backup
- Pre-modification scanning
- **Impact:** Medium - Would provide safer compliance operations

**Parallel Processing**
- Concurrent scan operations
- Progress tracking per operation
- **Impact:** Low - Already used but not user-visible

### 1.2 Cleaner Package Features

#### ✅ Features Integrated
- Basic duplicate deletion
- Compliance issue fixes
- Dry-run mode via CLI
- Operation logging

#### ❌ Features NOT Integrated

**Advanced Cleaning** (`internal/cleaner/cleaner.go`)
- Individual file selection in TUI (currently all-or-nothing)
- Selective compliance fixes by type
- Cleaning with backup snapshots
- Undo operations
- **Impact:** High - Users need fine-grained control

### 1.3 Config Package Features

#### ✅ Features Integrated
- Basic library path configuration
- Daemon frequency settings
- API key storage (TVDB/OMDB stubs)

#### ❌ Features NOT Integrated

**Advanced Configuration**
- Per-library scan settings
- Custom quality thresholds
- Notification preferences
- Log level persistence
- **Impact:** Medium - Power user features

### 1.4 Reporter Package Features

#### ✅ Features Integrated
- Basic JSON report generation
- Report viewing in TUI
- Summary statistics

#### ❌ Features NOT Integrated

**Streaming Reporting** (`internal/reporter/streaming.go`)
- Real-time report streaming
- Partial result viewing during scans
- **Impact:** Medium - Better user experience for long scans

**Enhanced Reporting**
- Historical trend analysis
- Storage efficiency metrics
- Duplicate pattern analysis
- **Impact:** Low-Medium - Analytics features

### 1.5 Daemon Package Features

#### ✅ Features Integrated
- Systemd timer management (enable/disable/status)
- Basic scan scheduling

#### ❌ Features NOT Integrated

**Advanced Daemon Control**
- Custom schedule configuration
- Multiple scan profiles
- On-demand daemon-triggered scans
- **Impact:** Medium - Flexible automation

## 2. CLI Gaps Analysis

### 2.1 Current CLI Commands
```bash
jellysink scan --library [movies|tv]    # ✅ Working
jellysink view <report>                 # ✅ Working
jellysink clean <report> --dry-run      # ✅ Working
jellysink config                        # ✅ Basic
jellysink version                       # ✅ Working
```

### 2.2 Missing CLI Commands

#### High Priority
```bash
jellysink backup create [name]          # Create library backup
jellysink backup list                   # List backups
jellysink backup preview <id>           # Preview restore
jellysink backup restore <id>           # Restore from backup
jellysink organize --dry-run            # Scan for loose files
jellysink organize --apply              # Organize loose files
jellysink rename --preview              # Preview TV renames
```

#### Medium Priority
```bash
jellysink scan --compliance-only        # Compliance check only
jellysink scan --duplicates-only        # Duplicates only
jellysink report stats <report>         # Show detailed stats
jellysink blacklist update              # Update blacklist from SRRDB
jellysink daemon trigger                # Manual daemon trigger
```

#### Low Priority
```bash
jellysink library validate              # Validate library paths
jellysink config export                 # Export config
jellysink config import <file>          # Import config
```

## 3. TUI Gaps Analysis

### 3.1 Main Menu Analysis

#### ✅ Integrated (8 items)
1. Run Manual Scan
2. View Last Report
3. Manage Backups
4. Configure Frequency
5. Enable/Disable Daemon
6. Configure Libraries
7. Configure API Keys
8. Exit

#### ❌ Missing Menu Options

**High Priority**
- **Organize Loose Files** - Scan and organize poorly structured media
- **Preview TV Renames** - Safe rename preview before execution
- **Selective Clean** - Choose specific files/compliance issues to fix
- **Backup Scheduling** - Schedule automatic backups

**Medium Priority**
- **View All Reports** - List and browse historical reports
- **Report Statistics** - In-depth analytics for selected report
- **Blacklist Management** - View/update duplicate detection patterns
- **Log Viewer** - Browse operation logs with filtering

**Low Priority**
- **Storage Analysis** - Visual storage usage and efficiency
- **Trend Analysis** - Duplicate/compliance trends over time
- **Batch Operations** - Multiple report operations

### 3.2 Detailed TUI Screen Gaps

#### Backup Management Screen (Partially Implemented)
**Current:**
- Create backup ✅
- List backups ✅
- Verify backup ✅
- Revert to latest ✅

**Missing:**
- Preview restore (file lists)
- Selective restore (choose operations)
- Backup scheduling configuration
- Backup retention policies
- Detailed backup contents view
- Backup comparison (diff view)

#### Loose Files Organizer Screen (NOT Implemented)
**Missing:**
- Scan progress with file discovery
- Results table (file, current location, suggested location, type)
- Selective organization (checkboxes)
- Conflict resolution (existing files)
- Batch preview (show all changes)
- Apply organization with progress
- Undo organization (from backup)

#### Rename Preview Screen (NEW - Just Added to Backend)
**Missing:**
- Preview results display
- Collision warnings highlighting
- Empty folder warnings
- Episode count summaries
- Multi-library impact analysis
- Approval workflow (review → approve → execute)

#### Enhanced Clean Screen (Partially Implemented)
**Current:**
- Show all duplicates/compliance issues ✅
- Table view with selection ✅
- Execute deletion ✅

**Missing:**
- Filter by type (movies only, TV only, compliance only)
- Sort by impact (size, count)
- Bulk selection patterns (largest in each group, etc.)
- Preview impact analysis
- Undo functionality (restore from recycle/backup)
- Scheduling options (clean during off-hours)

#### Report Viewer Enhancement
**Current:**
- View duplicates/compliance issues ✅
- Manual TV title editing ✅
- Conflict resolution ✅
- Execute clean ✅

**Missing:**
- Export report (JSON, CSV, PDF)
- Report comparison (vs. previous scan)
- Search/filter issues
- Custom issue tagging
- Notes/annotations on issues
- Report sharing (network accessible)

## 4. Integration Priority Matrix

### Priority 1: Critical User Value + Easy Implementation
- [ ] **Loose files CLI commands** - High impact, uses existing functions
- [ ] **Rename preview CLI** - Just added to backend, simple to expose
- [ ] **Backup CLI commands** - Functions exist, just need CLI wrappers
- [ ] **Selective clean in TUI** - Extend existing clean screen

### Priority 2: High User Value + Moderate Implementation
- [ ] **Loose files TUI screen** - New screen but clear requirements
- [ ] **Rename preview TUI screen** - New backend function needs UI
- [ ] **Enhanced backup management** - Extend existing backup screen
- [ ] **Report statistics** - Analyzer functions exist

### Priority 3: Medium User Value + High Implementation Complexity
- [ ] **Trend analysis** - Requires database/history tracking
- [ ] **Custom scan profiles** - Major config system changes
- [ ] **Multi-user support** - Architecture changes needed
- [ ] **Network interface** - Significant new subsystem

### Priority 4: Nice-to-Have + Low Complexity
- [ ] **Additional CLI commands** (config export/import, library validate)
- [ ] **Log viewer screen** - Simple log file parsing
- [ ] **Blacklist management** - Small config UI

## 5. Detailed Implementation Plan

### Phase 1: CLI Expansion (Week 1-2)
**Goal:** Expose existing backend functions via CLI immediately

**Commands to Add:**
```bash
# Backup management
jellysink backup create [--name <name>]
jellysink backup list
jellysink backup info <backup-id>
jellysink backup verify <backup-id>
jellysink backup restore <backup-id> [--dry-run]

# Loose file management
jellysink files scan [--library <movies|tv>]
jellysink files organize [--dry-run] [--apply]

# Rename preview
jellysink rename preview --old <title> --new <title> --library <path>
jellysink rename execute --old <title> --new <title> --library <path>

# Enhanced daemon control
jellysink daemon trigger
jellysink daemon logs
jellysink daemon config

# Report management
jellysink report list
jellysink report stats <report-id>
```

**Implementation Notes:**
- Most functions already exist in scanner package
- Add Cobra command definitions in main.go
- Follow existing patterns (RunE functions, error handling)
- Add tests for each command

### Phase 2: Core TUI Screens (Week 3-5)
**Goal:** Build missing TUI screens for high-value features

**Screens to Build:**

1. **Loose Files Organizer Screen** (New)
   - Menu: `Manage Loose Files` in main menu
   - Flow: Scan → Preview → Select → Apply
   - Components: Progress, table, selection checkboxes, confirm dialog
   - Use ScanningModel as template for progress
   - Use report view table patterns for results

2. **Rename Preview Screen** (New)
   - Trigger: Before TV rename operations
   - Flow: Preview → Confirm → Execute
   - Components: Summary view, warnings list, action buttons
   - Use new `PreviewTVRename()` backend function
   - Integrate into existing rename workflows

3. **Enhanced Clean Screen** (Extend)
   - Add: Filters (type, size, library)
   - Add: Selection patterns (all, largest, newest)
   - Add: Preview impact before executing
   - Add: Undo option using backup system
   - Modify existing clean flow in TUI

4. **Report Statistics Screen** (New)
   - Accessible from report viewer
   - Show: Trends, patterns, efficiency metrics
   - Use existing reporter package functions

### Phase 3: Advanced Integration (Week 6-8)
**Goal:** Polish and add medium-priority features

**Features:**
- Export reports (JSON, CSV)
- Comparison view (current vs. previous scan)
- Config management enhancements
- Blacklist management screen
- Log viewer screen
- Notification preferences

### Phase 4: Optimization & Testing (Week 9-10)
**Goal:** Performance, edge cases, and UX polish

**Focus Areas:**
- Handle large libraries (100k+ files)
- Graceful cancellation at all points
- Recovery from crashes/interruptions
- Accessibility improvements
- Documentation updates

## 6. Technical Architecture Notes

### Current State
- **Backend:** Well-structured, comprehensive feature set
- **CLI:** Minimal but functional
- **TUI:** Good foundation, missing key screens
- **Integration:** Many backend features orphaned

### Recommended Patterns

#### For CLI Commands
```go
var newCmd = &cobra.Command{
    Use:   "command [args]",
    Short: "Brief description",
    RunE: func(cmd *cobra.Command, args []string) error {
        // Check root
        if !isRunningAsRoot() {
            reexecWithSudo()
            return nil
        }
        
        // Load config
        cfg, err := loadConfig()
        if err != nil {
            return fmt.Errorf("loading config: %w", err)
        }
        
        // Call backend function
        result, err := scanner.BackendFunction(cfg, args[0])
        if err != nil {
            return fmt.Errorf("operation failed: %w", err)
        }
        
        // Print result
        fmt.Printf("Success: %v\n", result)
        return nil
    },
}
```

#### For TUI Screens
```go
// Follow existing patterns from menu.go
// Use: list.Model for selections
// Use: textinput.Model for input
// Use: viewport.Model for logs/content
// Use: progress.Model for progress bars
// Follow RAMA theme styling
```

#### For Progress Reporting
```go
// Backend functions should accept progressCh chan<- ScanProgress
// TUI should create channel and pass to backend
// CLI should read from channel and print updates
// Follow existing patterns in daemon.go
```

### Testing Strategy
- Unit tests for all new CLI commands
- Unit tests for TUI model functions
- Integration tests for full workflows
- Test with real media libraries
- Performance testing with large datasets

## 7. Code Quality Considerations

### Technical Debt to Address

1. **Error Handling**
   - Standardize error types across packages
   - User-friendly error messages
   - Error recovery strategies

2. **Configuration**
   - Move from toml to something with schema validation
   - Config migration system
   - Config validation at load time

3. **Logging**
   - Structured logging throughout
   - Log rotation configuration
   - Centralized log management

4. **Testing**
   - Increase test coverage (currently ~60%)
   - More edge case tests
   - Performance benchmark suite

5. **Documentation**
   - User guide for all features
   - API documentation for backend functions
   - TUI navigation help
   - CLI command reference

### Performance Targets
- Scan 100k files in < 5 minutes
- TUI startup < 1 second
- Report loading < 2 seconds
- Rename operations < 10 seconds for 1000 files
- Memory usage < 1GB for typical libraries

## 8. User Experience Improvements

### Immediate Wins (Backend Already Supports)
- [ ] Show scan progress ETA
- [ ] Detailed progress by operation
- [ ] Alert modals for critical errors
- [ ] Operation logs with severity levels
- [ ] Auto-scroll to latest log entries

### Medium-term UX
- [ ] Config validation feedback in TUI
- [ ] Undo operations via backup system
- [ ] Dark/light theme toggle
- [ ] Customizable keybindings
- [ ] Mouse support in TUI

## 9. Security Considerations

### Current State
- Root required for destructive operations ✅
- Path validation to prevent system damage ✅
- Backup system for recovery ✅

### Missing Features
- [ ] Config file encryption for API keys
- [ ] Operation audit logging
- [ ] User confirmation for destructive operations
- [ ] Dry-run preview for all destructive operations
- [ ] Operation rollback capabilities

## 10. Documentation Requirements

### User Documentation
- [ ] Quick start guide
- [ ] Feature overview
- [ ] CLI command reference
- [ ] TUI navigation guide
- [ ] Best practices
- [ ] Troubleshooting guide

### Developer Documentation
- [ ] Architecture overview
- [ ] Backend package API docs
- [ ] TUI component guide
- [ ] Adding new features guide
- [ ] Code style guide

## 11. Success Metrics

### Integration Completion
- [ ] 95% of backend functions accessible via CLI
- [ ] 90% of backend features accessible via TUI
- [ ] 0 backend functions that are "orphaned" (not called from frontend)

### User Adoption
- [ ] Loose file organizer used by 80% of users
- [ ] Rename preview used for for preview used before 90% of rename operations
- [ ] Backup system used by 90% of users before cleaning

### Code Quality
- [ ] Test coverage > 80%
- [ ] Zero linting errors
- [ ] All public functions documented
- [ ] Performance benchmarks met

## 12. Conclusion

Jellysink's backend is mature and feature-rich, but ~40% of capabilities are not accessible to users. The integration gaps represent significant user value left unrealized. The planned integration work will transform jellysink from a basic duplicate finder into a comprehensive media library management tool.

**Next Steps:**
1. Review and approve this plan
2. Begin Phase 1: CLI expansion
3. Create detailed technical specs for Phase 2
4. Implement user feedback mechanisms
5. Establish release cadence (bi-weekly)

**Estimated Timeline:** 10 weeks for complete integration
**Priority Order:** Follow Priority 1→4 matrix
**Success Criteria:** 90% backend feature exposure in frontend

---

**Document Authors:** Code Audit Team
**Review Status:** Pending Review
**Last Updated:** 2025-01-19