# Conflict Review UI Guide

## Overview

The Conflict Review UI allows you to resolve TV show title conflicts interactively. This three-stage workflow ensures you make the right decisions before applying batch changes.

## Navigation

### Stage 1: Individual Conflict Review

**Navigating Between Conflicts:**
- `←` (Left Arrow) - Previous conflict
- `→` (Right Arrow) - Next conflict

**Making Decisions:**
- `1` - Select Option 1 (Folder title)
- `2` - Select Option 2 (Filename title)
- `E` - Enter custom title (Option 3)
- `S` - Skip this conflict (keep current/auto-resolved title)

**Text Input Mode (when editing custom title):**
- Type to enter custom title
- `Enter` - Save custom title
- `Esc` - Cancel editing

**Other Actions:**
- `Esc` - Return to summary view
- `Q` / `Ctrl+C` - Quit application

### Stage 2: Batch Summary

Once all conflicts have decisions, press `Enter` to proceed to the batch summary view.

**In Batch Summary:**
- Review all decisions in table format
- `Enter` - Apply all renames
- `Esc` - Go back to conflict review to make changes
- `Q` / `Ctrl+C` - Quit without applying

### Stage 3: Apply Changes

After confirming in batch summary:
- jellysink exits
- Run the returned command to apply renames
- Example: `./jellysink apply-renames --conflicts /path/to/conflicts.json`

## Workflow Example

### Example Session

```
┌─────────────────────────────────────────────────┐
│ TV SHOW TITLE CONFLICT                         │
│                                                 │
│ Reviewing conflict 1 of 3                      │
│                                                 │
│ ⚠ CONFLICTING TITLES DETECTED                  │
│                                                 │
│ Option 1: Folder Title                         │
│   Star Trek [confidence: 75%]                  │
│   Press '1' to select                          │
│                                                 │
│ Option 2: Filename Title                       │
│   Star Trek The Next Generation [85%]          │
│   ✓ SELECTED                                   │
│                                                 │
│ Option 3: Custom Title                         │
│   Press 'E' to enter custom title              │
│                                                 │
│ ────────────────────────────────────────────── │
│                                                 │
│ Conflict Reason: Filename contains show        │
│                  subtitle not in folder name   │
│ ℹ API verification unavailable                 │
│                                                 │
│ ────────────────────────────────────────────── │
│                                                 │
│ ✓ Decision recorded                            │
│   ← → to navigate conflicts                    │
│                                                 │
└─────────────────────────────────────────────────┘

[Press → to go to next conflict]
```

### After Resolving All Conflicts

```
┌─────────────────────────────────────────────────┐
│ ✓ Decision recorded                            │
│   ← → to navigate conflicts                    │
│   ✓ All conflicts resolved - Press Enter to   │
│     proceed to batch review                    │
└─────────────────────────────────────────────────┘
```

### Batch Summary View

```
┌──────────────────────────────────────────────────────────────┐
│ BATCH REVIEW SUMMARY                                        │
│                                                              │
│ Reviewing 3 decision(s) before applying changes             │
│                                                              │
│ ──────────────────────────────────────────────────────────  │
│ #    Show Name                   New Title             Source│
│ ──────────────────────────────────────────────────────────  │
│   1  Star Trek                   Star Trek TNG         Filenm│
│   2  Degrassi                    Degrassi TNG          Folder│
│   3  Lost                        Lost                  Skippe│
│ ──────────────────────────────────────────────────────────  │
│                                                              │
│ Next Steps:                                                  │
│   • Review the decisions above                              │
│   • Press Enter to apply all renames                        │
│   • Press Esc to go back and make changes                   │
│                                                              │
│ ⚠ Renames will be applied to both folders and filenames    │
│   for consistency                                           │
└──────────────────────────────────────────────────────────────┘
```

## Decision Types

### 1. Folder Title (Option 1)
- Uses the title extracted from the show's folder name
- Typically the "official" name you've organized by
- Best when folder structure is correct

### 2. Filename Title (Option 2)
- Uses the title extracted from episode filenames
- May include additional subtitle information
- Best when filenames are more complete

### 3. Custom Title (Option 3)
- Manually enter the correct title
- Best when both automated options are wrong
- Supports full show names with subtitles

### 4. Skip (Press S)
- Keep the current auto-resolved title
- jellysink will not rename this show
- Best when conflict is false positive

## Tips

1. **API Verification**: If available, jellysink will show whether the title was verified against TVDB/OMDB. This helps validate your choice.

2. **Confidence Scores**: Higher confidence (closer to 100%) indicates jellysink is more certain about the title extraction.

3. **Conflict Reason**: Read the reason to understand why jellysink flagged this as ambiguous.

4. **Review Before Applying**: Always review the batch summary before pressing Enter. This is your last chance to go back and fix mistakes.

5. **Arrow Keys**: The left/right arrow keys make it easy to navigate back and forth to review your decisions.

## Common Scenarios

### Scenario 1: Show with Subtitle
**Problem**: Folder is "Star Trek" but files are "Star Trek TNG S01E01.mkv"

**Solution**: 
- Option 2 (Filename) likely has the full name "Star Trek TNG"
- Select Option 2 to rename folder to match

### Scenario 2: Abbreviated Folder Name
**Problem**: Folder is "TNG" but files are "Star Trek TNG S01E01.mkv"

**Solution**:
- Option 2 (Filename) has the complete name
- Select Option 2 to expand abbreviation

### Scenario 3: Both Wrong
**Problem**: Folder is "ST_TNG" and files are "st.tng.s01e01.mkv"

**Solution**:
- Press 'E' to enter custom title
- Type "Star Trek The Next Generation"
- Press Enter to save

### Scenario 4: Release Group Artifacts
**Problem**: Both show markers like "x264", "1080p", etc.

**Solution**:
- Check which option has cleaner extraction
- If both are bad, use Option 3 (Custom)

## Keyboard Shortcuts Reference

| Key | Action |
|-----|--------|
| `←` | Previous conflict |
| `→` | Next conflict |
| `1` | Select folder title |
| `2` | Select filename title |
| `E` | Edit custom title |
| `S` | Skip conflict |
| `Enter` | Proceed to batch summary (when all resolved) |
| `Enter` | Apply renames (in batch summary) |
| `Esc` | Go back / Cancel |
| `Q` | Quit |

## Troubleshooting

**Q: I pressed Enter but nothing happened**
- A: You need to resolve ALL conflicts first. Navigate through all conflicts using arrow keys and make a decision for each.

**Q: How do I know if all conflicts are resolved?**
- A: When all conflicts have decisions, you'll see: "✓ All conflicts resolved - Press Enter to proceed to batch review"

**Q: Can I change my mind after making a decision?**
- A: Yes! Navigate back to the conflict using `←` and select a different option. You can also press `Esc` from batch summary to return to conflict review.

**Q: What happens if I skip a conflict?**
- A: jellysink will keep the current auto-resolved title and won't rename that show.

**Q: I want to quit without saving**
- A: Press `Q` or `Ctrl+C` at any time. No changes are applied until you confirm in the batch summary and run the apply command.

## Integration with API Verification

When API verification succeeds:
- Title is validated against TVDB or OMDB
- Conflict reason shows which API verified the title
- Confidence is adjusted based on API results

When API verification fails:
- "ℹ API verification unavailable" is shown
- You must rely on confidence scores and manual review
- API retry logic (3 attempts) runs automatically in background

## Related Documentation

- `API_RETRY_ENHANCEMENTS.md` - API caching and retry logic
- `MANUAL_INTERVENTION_FEATURE.md` - Original feature design
- `INTERVENTION.md` - Implementation progress tracking
