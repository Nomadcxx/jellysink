#!/usr/bin/env python3
"""
Generate release group blacklist from srrDB dump.

Usage:
    python3 scripts/generate_blacklist_from_srrdb.py

Reads: archivedFiles (srrDB CSV dump)
Outputs: Go code snippet for blacklist.go
"""

import re
import sys
from collections import Counter

def extract_release_groups(csv_path):
    """Extract release groups from srrDB CSV dump."""
    groups = set()
    
    print(f"Reading {csv_path}...", file=sys.stderr)
    
    with open(csv_path, 'r', encoding='utf-8', errors='ignore') as f:
        for line_num, line in enumerate(f, 1):
            if line_num % 100000 == 0:
                print(f"  Processed {line_num:,} lines, found {len(groups):,} groups", file=sys.stderr)
            
            # Extract first column (release name)
            release_name = line.split(',')[0].strip()
            
            # Extract group suffix (last part after dash)
            # Examples: Movie.Name.2024-GROUP -> GROUP
            match = re.search(r'-([A-Za-z0-9]+)$', release_name)
            if match:
                groups.add(match.group(1))
    
    print(f"Total unique groups found: {len(groups):,}", file=sys.stderr)
    return groups

def filter_tv_movie_groups(csv_path):
    """Filter for TV and movie release groups only."""
    groups = set()
    
    # Patterns indicating TV/movie content
    tv_pattern = re.compile(r'S\d{2}E\d{2}|Season|Episode', re.IGNORECASE)
    movie_pattern = re.compile(r'DVDRip|BluRay|BRRip|WEBRip|HDTV|WEB-DL|1080p|720p|2160p|4K', re.IGNORECASE)
    
    print(f"Filtering TV/movie groups from {csv_path}...", file=sys.stderr)
    
    with open(csv_path, 'r', encoding='utf-8', errors='ignore') as f:
        for line_num, line in enumerate(f, 1):
            if line_num % 100000 == 0:
                print(f"  Processed {line_num:,} lines, found {len(groups):,} groups", file=sys.stderr)
            
            release_name = line.split(',')[0].strip()
            
            # Check if TV or movie
            is_tv = bool(tv_pattern.search(release_name))
            is_movie = bool(movie_pattern.search(release_name)) and not is_tv
            
            if is_tv or is_movie:
                match = re.search(r'-([A-Za-z0-9]+)$', release_name)
                if match:
                    groups.add(match.group(1))
    
    print(f"Total TV/movie groups: {len(groups):,}", file=sys.stderr)
    return groups

def filter_valid_groups(groups):
    """Filter out noise (pure numbers, test groups, etc.)."""
    valid = set()
    
    # Must start with letter and have 2+ alphanumeric chars
    valid_pattern = re.compile(r'^[A-Za-z][A-Za-z0-9]{1,}$')
    
    # Exclude known garbage patterns
    exclude_pattern = re.compile(r'^(TEST|SAMPLE|PROOF|RARBG|ETRG|YTS|YIFY)', re.IGNORECASE)
    
    for group in groups:
        if valid_pattern.match(group) and not exclude_pattern.match(group):
            valid.add(group)
    
    print(f"Valid groups after filtering: {len(valid):,}", file=sys.stderr)
    return valid

def generate_go_code(groups):
    """Generate Go code snippet for blacklist.go."""
    # Sort groups (case-insensitive)
    sorted_groups = sorted(groups, key=lambda x: x.lower())
    
    # Format as lowercase Go slice (8 groups per line)
    lines = []
    current_line = []
    
    for group in sorted_groups:
        current_line.append(f'"{group.lower()}"')
        
        if len(current_line) == 8:
            lines.append('\t\t' + ', '.join(current_line) + ',')
            current_line = []
    
    # Add remaining groups
    if current_line:
        lines.append('\t\t' + ', '.join(current_line) + ',')
    
    # Generate full code
    code = f'''// KnownReleaseGroups generated from srrDB dump
// Source: https://www.srrdb.com/open (archivedFiles)
// Total groups: {len(groups):,}
// Generated: {__import__('datetime').datetime.now().strftime('%Y-%m-%d %H:%M:%S')}

func buildReleaseGroupMap() map[string]bool {{
\treleaseGroups := []string{{
{chr(10).join(lines)}
\t}}

\t// === GENERIC GARBAGE TERMS ===
\tgenericTerms := []string{{
\t\t"group", "release", "rip", "encode", "repack",
\t\t"real", "readnfo", "limited",
\t\t"retail", "subbed", "dubbed", "multi", "unrated",
\t\t"extended", "theatrical", "directors", "cut", "edition",
\t\t"remux", "hybrid", "complete", "collection",
\t}}

\t// === QUALITY/SOURCE MARKERS ===
\tqualityMarkers := []string{{
\t\t"cam", "ts", "tc", "r5", "dvdscr", "screener",
\t\t"dvdrip", "bdrip", "brrip", "webrip", "webdl",
\t\t"hdtv", "pdtv", "dsr", "tvrip", "vodrip",
\t\t"bluray", "uhd", "fhd", "hd", "sd",
\t\t"1080p", "720p", "2160p", "480p", "4k",
\t}}

\t// Build map with case-insensitive keys (all lowercase)
\tgroups := make(map[string]bool, {len(groups) + 50})
\tallGroups := [][]string{{releaseGroups, genericTerms, qualityMarkers}}

\tfor _, groupList := range allGroups {{
\t\tfor _, group := range groupList {{
\t\t\tgroups[group] = true
\t\t}}
\t}}

\treturn groups
}}
'''
    
    return code

def main():
    csv_path = 'archivedFiles'
    
    # Extract TV/movie groups
    groups = filter_tv_movie_groups(csv_path)
    
    # Filter valid groups
    valid_groups = filter_valid_groups(groups)
    
    # Generate Go code
    go_code = generate_go_code(valid_groups)
    
    print("\n" + "="*80, file=sys.stderr)
    print("Generated Go code (copy to blacklist.go):", file=sys.stderr)
    print("="*80 + "\n", file=sys.stderr)
    print(go_code)
    
    # Save to file
    output_path = 'blacklist_generated.go.txt'
    with open(output_path, 'w') as f:
        f.write(go_code)
    
    print(f"\n✓ Saved to {output_path}", file=sys.stderr)

if __name__ == '__main__':
    main()
