#!/usr/bin/env python3
"""
PreDB.club Release Group Scraper
Fetches unique release group names from PreDB.club API and generates Go code for blacklist.go
Usage: python3 scripts/update_blacklist.py
"""

import requests
import time
from collections import Counter

API_KEY = "677884cfb17b94418c0a2e3f33a199bdc8c5c54fa4e74908c01b27ff82efbf3d"
API_BASE = "https://predb.club/api/v1"
HEADERS = {"x-api-key": API_KEY}

# Categories to scrape
TV_CATEGORIES = ["TV-720P", "TV-1080P", "TV-SD", "TV-4K"]
MOVIE_CATEGORIES = ["MOVIE-720P", "MOVIE-1080P", "MOVIE-SD", "MOVIE-4K"]

def fetch_releases(limit=1000, offset=0):
    """Fetch releases from PreDB.club API"""
    try:
        url = f"{API_BASE}/?limit={limit}&offset={offset}"
        response = requests.get(url, headers=HEADERS, timeout=30)
        response.raise_for_status()
        data = response.json()
        
        if data.get("status") == "success":
            return data.get("data", {}).get("rows", [])
        else:
            print(f"API error: {data.get('message')}")
            return []
    except Exception as e:
        print(f"Error fetching releases: {e}")
        return []

def extract_groups(releases):
    """Extract unique release groups from releases"""
    groups = set()
    for release in releases:
        team = release.get("team", "").strip()
        if team and team.lower() != "none":
            groups.add(team.lower())
    return groups

def categorize_groups(releases):
    """Categorize groups by media type"""
    tv_groups = set()
    movie_groups = set()
    
    for release in releases:
        team = release.get("team", "").strip().lower()
        cat = release.get("cat", "").upper()
        
        if not team or team == "none":
            continue
            
        if "TV" in cat:
            tv_groups.add(team)
        elif "MOVIE" in cat or "X264" in cat or "XVID" in cat:
            movie_groups.add(team)
    
    return tv_groups, movie_groups

def scrape_groups(batch_count=500, batch_size=1000):
    """Scrape release groups from PreDB.club"""
    print(f"Scraping up to {batch_count * batch_size} releases from PreDB.club...")
    print(f"This will take ~{batch_count} seconds due to rate limiting...")
    
    all_tv = set()
    all_movie = set()
    
    for i in range(batch_count):
        offset = i * batch_size
        
        releases = fetch_releases(limit=batch_size, offset=offset)
        if not releases:
            print(f"\nNo more releases found at offset {offset}")
            break
        
        tv, movie = categorize_groups(releases)
        all_tv.update(tv)
        all_movie.update(movie)
        
        # Progress update every 10 batches
        if (i + 1) % 10 == 0:
            print(f"Batch {i+1}/{batch_count}: {len(all_tv)} TV groups, {len(all_movie)} movie groups (offset={offset})")
        
        # Rate limiting
        time.sleep(0.5)
    
    return all_tv, all_movie

def generate_go_arrays(tv_groups, movie_groups):
    """Generate Go code arrays for the groups"""
    
    # Sort groups alphabetically
    tv_sorted = sorted(tv_groups)
    movie_sorted = sorted(movie_groups)
    
    print("\n=== NEW TV GROUPS ===")
    print('tvGroups := []string{')
    for i in range(0, len(tv_sorted), 10):
        batch = tv_sorted[i:i+10]
        quoted = [f'"{g}"' for g in batch]
        print(f'    {", ".join(quoted)},')
    print('}')
    
    print("\n=== NEW MOVIE GROUPS ===")
    print('movieGroups := []string{')
    for i in range(0, len(movie_sorted), 10):
        batch = movie_sorted[i:i+10]
        quoted = [f'"{g}"' for g in batch]
        print(f'    {", ".join(quoted)},')
    print('}')
    
    print(f"\n=== STATS ===")
    print(f"Total unique TV groups: {len(tv_groups)}")
    print(f"Total unique movie groups: {len(movie_groups)}")
    print(f"Overlap: {len(tv_groups & movie_groups)}")

if __name__ == "__main__":
    print("PreDB.club Release Group Scraper")
    print("=" * 50)
    
    tv_groups, movie_groups = scrape_groups(batch_count=500, batch_size=1000)
    
    if tv_groups or movie_groups:
        generate_go_arrays(tv_groups, movie_groups)
        
        print("\n=== INSTRUCTIONS ===")
        print("1. Review the generated arrays above")
        print("2. Manually merge with existing blacklist.go")
        print("3. Remove any groups that conflict with legitimate titles")
        print("4. Run tests: go test ./internal/scanner -run TestBlacklist")
    else:
        print("No groups found. Check API connectivity.")
