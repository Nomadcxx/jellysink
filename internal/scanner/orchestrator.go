package scanner

import (
	"context"
	"fmt"
)

// ScanResult contains all scan results and statistics
type ScanResult struct {
	MovieDuplicates  []MovieDuplicate
	TVDuplicates     []TVDuplicate
	ComplianceIssues []ComplianceIssue
	AmbiguousTVShows []*TVTitleResolution

	TotalDuplicates    int
	TotalFilesToDelete int
	SpaceToFree        int64
}

// RunFullScan orchestrates all scan operations with progress reporting and cancellation support
func RunFullScan(ctx context.Context, moviePaths, tvPaths []string, progressCh chan<- ScanProgress) (*ScanResult, error) {
	result := &ScanResult{}

	// Stage 1: Scan movies for duplicates
	if len(moviePaths) > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		movieDuplicates, err := ScanMoviesWithProgress(moviePaths, progressCh)
		if err != nil {
			return nil, fmt.Errorf("movie duplicate scan failed: %w", err)
		}
		result.MovieDuplicates = MarkKeepDelete(movieDuplicates)
	}

	// Stage 2: Scan TV shows for duplicates
	if len(tvPaths) > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		tvDuplicates, err := ScanTVShowsWithProgress(tvPaths, progressCh)
		if err != nil {
			return nil, fmt.Errorf("TV duplicate scan failed: %w", err)
		}
		result.TVDuplicates = MarkKeepDeleteTV(tvDuplicates)
	}

	// Stage 3: Movie compliance check
	if len(moviePaths) > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Exclude files marked for deletion
		filesToDelete := GetDeleteList(result.MovieDuplicates)

		complianceIssues, err := ScanMovieComplianceWithProgress(moviePaths, progressCh, filesToDelete...)
		if err != nil {
			return nil, fmt.Errorf("movie compliance scan failed: %w", err)
		}
		result.ComplianceIssues = append(result.ComplianceIssues, complianceIssues...)
	}

	// Stage 4: TV compliance check
	if len(tvPaths) > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Exclude files marked for deletion
		tvFilesToDelete := GetTVDeleteList(result.TVDuplicates)

		tvComplianceIssues, err := ScanTVComplianceWithProgress(tvPaths, progressCh, tvFilesToDelete...)
		if err != nil {
			return nil, fmt.Errorf("TV compliance scan failed: %w", err)
		}
		result.ComplianceIssues = append(result.ComplianceIssues, tvComplianceIssues...)
	}

	// Calculate statistics
	result.TotalDuplicates = len(result.MovieDuplicates) + len(result.TVDuplicates)

	for _, dup := range result.MovieDuplicates {
		result.TotalFilesToDelete += len(dup.Files) - 1
		for i := 1; i < len(dup.Files); i++ {
			result.SpaceToFree += dup.Files[i].Size
		}
	}

	for _, dup := range result.TVDuplicates {
		result.TotalFilesToDelete += len(dup.Files) - 1
		for i := 1; i < len(dup.Files); i++ {
			result.SpaceToFree += dup.Files[i].Size
		}
	}

	return result, nil
}
