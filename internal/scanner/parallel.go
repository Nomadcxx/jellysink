package scanner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// ParallelConfig holds configuration for parallel scanning
type ParallelConfig struct {
	Workers int // Number of concurrent workers (default: number of CPUs)
}

// DefaultParallelConfig returns optimal parallel scanning configuration
func DefaultParallelConfig() ParallelConfig {
	return ParallelConfig{
		Workers: runtime.NumCPU(),
	}
}

// ScanMoviesParallel scans movie libraries using worker pools for improved performance
// Processes multiple library paths concurrently
// Supports context cancellation for graceful shutdown
func ScanMoviesParallel(ctx context.Context, paths []string, config ParallelConfig) ([]MovieDuplicate, error) {
	if config.Workers <= 0 {
		config.Workers = runtime.NumCPU()
	}

	// Shared data structure (protected by mutex)
	var mu sync.Mutex
	movieGroups := make(map[string]*MovieDuplicate)

	// WaitGroup to track worker completion
	var wg sync.WaitGroup

	// Channel for paths to process
	pathChan := make(chan string, len(paths))

	// Error handling
	errChan := make(chan error, config.Workers)
	var scanErr error
	var errOnce sync.Once

	// Start worker pool
	for i := 0; i < config.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					errOnce.Do(func() {
						scanErr = ctx.Err()
						errChan <- ctx.Err()
					})
					return
				case libPath, ok := <-pathChan:
					if !ok {
						return
					}
					if err := scanMoviePathParallel(ctx, libPath, movieGroups, &mu); err != nil {
						errOnce.Do(func() {
							scanErr = err
							errChan <- err
						})
						return
					}
				}
			}
		}()
	}

	// Send paths to workers
	for _, path := range paths {
		select {
		case <-ctx.Done():
			close(pathChan)
			wg.Wait()
			return nil, ctx.Err()
		case pathChan <- path:
		}
	}
	close(pathChan)

	// Wait for all workers to complete
	wg.Wait()
	close(errChan)

	// Check for errors
	if scanErr != nil {
		return nil, scanErr
	}

	// Filter to only duplicates (2+ files per group)
	var duplicates []MovieDuplicate
	for _, group := range movieGroups {
		if len(group.Files) > 1 {
			duplicates = append(duplicates, *group)
		}
	}

	return duplicates, nil
}

// scanMoviePathParallel scans a single movie library path (thread-safe)
// Supports context cancellation
func scanMoviePathParallel(ctx context.Context, libPath string, movieGroups map[string]*MovieDuplicate, mu *sync.Mutex) error {
	// Verify path exists
	if _, err := os.Stat(libPath); err != nil {
		return fmt.Errorf("library path not accessible: %s: %w", libPath, err)
	}

	// Walk directory tree with context cancellation support
	err := filepath.Walk(libPath, func(path string, info os.FileInfo, err error) error {
		// Check for cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Only process video files
		if !isVideoFile(path) {
			return nil
		}

		// Extract movie info from filename/path
		movieFile := parseMovieFile(path, info)

		// Extract movie title from parent directory name (Jellyfin format)
		parentDir := filepath.Base(filepath.Dir(path))
		movieTitle := parentDir
		if parentDir == "." || parentDir == "/" {
			// Fallback to filename
			movieTitle = filepath.Base(path)
		}

		// Create group key: normalized_name|year
		normalized := NormalizeName(movieTitle)
		year := ExtractYear(movieTitle)
		key := normalized + "|" + year

		// Thread-safe access to shared map
		mu.Lock()
		if _, exists := movieGroups[key]; !exists {
			movieGroups[key] = &MovieDuplicate{
				NormalizedName: normalized,
				Year:           year,
				Files:          []MovieFile{},
			}
		}
		movieGroups[key].Files = append(movieGroups[key].Files, movieFile)
		mu.Unlock()

		return nil
	})

	if err != nil {
		return fmt.Errorf("error scanning %s: %w", libPath, err)
	}

	return nil
}

// ScanTVShowsParallel scans TV library paths using worker pools for improved performance
// Processes multiple library paths concurrently
// Supports context cancellation for graceful shutdown
func ScanTVShowsParallel(ctx context.Context, paths []string, config ParallelConfig) ([]TVDuplicate, error) {
	if config.Workers <= 0 {
		config.Workers = runtime.NumCPU()
	}

	// Shared data structure (protected by mutex)
	var mu sync.Mutex
	episodeGroups := make(map[string]*TVDuplicate)

	// WaitGroup to track worker completion
	var wg sync.WaitGroup

	// Channel for paths to process
	pathChan := make(chan string, len(paths))

	// Error handling
	errChan := make(chan error, config.Workers)
	var scanErr error
	var errOnce sync.Once

	// Start worker pool
	for i := 0; i < config.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					errOnce.Do(func() {
						scanErr = ctx.Err()
						errChan <- ctx.Err()
					})
					return
				case libPath, ok := <-pathChan:
					if !ok {
						return
					}
					if err := scanTVPathParallel(ctx, libPath, episodeGroups, &mu); err != nil {
						errOnce.Do(func() {
							scanErr = err
							errChan <- err
						})
						return
					}
				}
			}
		}()
	}

	// Send paths to workers
	for _, path := range paths {
		select {
		case <-ctx.Done():
			close(pathChan)
			wg.Wait()
			return nil, ctx.Err()
		case pathChan <- path:
		}
	}
	close(pathChan)

	// Wait for all workers to complete
	wg.Wait()
	close(errChan)

	// Check for errors
	if scanErr != nil {
		return nil, scanErr
	}

	// Filter to only duplicates (2+ files per episode)
	var duplicates []TVDuplicate
	for _, group := range episodeGroups {
		if len(group.Files) > 1 {
			duplicates = append(duplicates, *group)
		}
	}

	return duplicates, nil
}

// scanTVPathParallel scans a single TV library path (thread-safe)
// Supports context cancellation
func scanTVPathParallel(ctx context.Context, libPath string, episodeGroups map[string]*TVDuplicate, mu *sync.Mutex) error {
	// Verify path exists
	if _, err := os.Stat(libPath); err != nil {
		return fmt.Errorf("library path not accessible: %s: %w", libPath, err)
	}

	// Walk directory tree with context cancellation support
	err := filepath.Walk(libPath, func(path string, info os.FileInfo, err error) error {
		// Check for cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Only process video files
		if !isVideoFile(path) {
			return nil
		}

		// Extract episode info from filename
		season, episode, found := ExtractEpisodeInfo(filepath.Base(path))
		if !found {
			// Not a TV episode format, skip
			return nil
		}

		// Parse TV file metadata
		tvFile := parseTVFile(path, info)

		// Extract show name from parent directory structure
		showDir := filepath.Dir(filepath.Dir(path)) // Go up two levels
		showName := filepath.Base(showDir)

		// Normalize show name
		normalized := NormalizeName(showName)

		// Create group key: normalized_show|S##E##
		key := fmt.Sprintf("%s|S%02dE%02d", normalized, season, episode)

		// Thread-safe access to shared map
		mu.Lock()
		if _, exists := episodeGroups[key]; !exists {
			episodeGroups[key] = &TVDuplicate{
				ShowName: normalized,
				Season:   season,
				Episode:  episode,
				Files:    []TVFile{},
			}
		}
		episodeGroups[key].Files = append(episodeGroups[key].Files, tvFile)
		mu.Unlock()

		return nil
	})

	if err != nil {
		return fmt.Errorf("error scanning %s: %w", libPath, err)
	}

	return nil
}
