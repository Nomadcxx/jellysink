package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config holds all jellysink configuration
type Config struct {
	Libraries LibraryConfig `toml:"libraries"`
	Daemon    DaemonConfig  `toml:"daemon"`
}

// LibraryConfig defines media library paths
type LibraryConfig struct {
	Movies MovieLibrary `toml:"movies"`
	TV     TVLibrary    `toml:"tv"`
}

// MovieLibrary holds movie library paths
type MovieLibrary struct {
	Paths []string `toml:"paths"`
}

// TVLibrary holds TV show library paths
type TVLibrary struct {
	Paths []string `toml:"paths"`
}

// DaemonConfig holds daemon scheduling and behavior settings
type DaemonConfig struct {
	ScanFrequency    string `toml:"scan_frequency"`     // daily, weekly, biweekly
	ReportOnComplete bool   `toml:"report_on_complete"` // launch TUI on scan complete
}

// DefaultConfig returns a config with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Libraries: LibraryConfig{
			Movies: MovieLibrary{
				Paths: []string{},
			},
			TV: TVLibrary{
				Paths: []string{},
			},
		},
		Daemon: DaemonConfig{
			ScanFrequency:    "weekly",
			ReportOnComplete: true,
		},
	}
}

// ConfigPath returns the path to the config file
func ConfigPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to get config directory: %w", err)
	}

	jellysinkDir := filepath.Join(configDir, "jellysink")
	configFile := filepath.Join(jellysinkDir, "config.toml")

	return configFile, nil
}

// EnsureConfigDir creates the config directory if it doesn't exist
func EnsureConfigDir() error {
	configFile, err := ConfigPath()
	if err != nil {
		return err
	}

	configDir := filepath.Dir(configFile)

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	return nil
}

// Load reads the config file, creating it with defaults if it doesn't exist
func Load() (*Config, error) {
	configFile, err := ConfigPath()
	if err != nil {
		return nil, err
	}

	// Create config directory if needed
	if err := EnsureConfigDir(); err != nil {
		return nil, err
	}

	// If config doesn't exist, create it with defaults
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		cfg := DefaultConfig()
		if err := Save(cfg); err != nil {
			return nil, fmt.Errorf("failed to create default config: %w", err)
		}
		return cfg, nil
	}

	// Load existing config
	var cfg Config
	if _, err := toml.DecodeFile(configFile, &cfg); err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return &cfg, nil
}

// Save writes the config to disk
func Save(cfg *Config) error {
	configFile, err := ConfigPath()
	if err != nil {
		return err
	}

	// Ensure config directory exists
	if err := EnsureConfigDir(); err != nil {
		return err
	}

	// Open file for writing
	f, err := os.Create(configFile)
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	defer f.Close()

	// Encode config as TOML
	encoder := toml.NewEncoder(f)
	if err := encoder.Encode(cfg); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// Validate checks if the config is valid
func (c *Config) Validate() error {
	// Check scan frequency
	validFrequencies := map[string]bool{
		"daily":    true,
		"weekly":   true,
		"biweekly": true,
	}

	if !validFrequencies[c.Daemon.ScanFrequency] {
		return fmt.Errorf("invalid scan frequency: %s (must be daily, weekly, or biweekly)", c.Daemon.ScanFrequency)
	}

	// Check that at least one library path is configured
	if len(c.Libraries.Movies.Paths) == 0 && len(c.Libraries.TV.Paths) == 0 {
		return fmt.Errorf("no library paths configured")
	}

	// Validate all paths exist and are readable
	allPaths := append(c.Libraries.Movies.Paths, c.Libraries.TV.Paths...)
	for _, path := range allPaths {
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("library path %s: %w", path, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("library path %s is not a directory", path)
		}
	}

	return nil
}

// AddMoviePath adds a movie library path
func (c *Config) AddMoviePath(path string) error {
	// Check if path exists
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("path does not exist: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", path)
	}

	// Check if already exists
	for _, existing := range c.Libraries.Movies.Paths {
		if existing == path {
			return fmt.Errorf("path already configured: %s", path)
		}
	}

	c.Libraries.Movies.Paths = append(c.Libraries.Movies.Paths, path)
	return nil
}

// AddTVPath adds a TV show library path
func (c *Config) AddTVPath(path string) error {
	// Check if path exists
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("path does not exist: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", path)
	}

	// Check if already exists
	for _, existing := range c.Libraries.TV.Paths {
		if existing == path {
			return fmt.Errorf("path already configured: %s", path)
		}
	}

	c.Libraries.TV.Paths = append(c.Libraries.TV.Paths, path)
	return nil
}

// RemoveMoviePath removes a movie library path
func (c *Config) RemoveMoviePath(path string) error {
	for i, existing := range c.Libraries.Movies.Paths {
		if existing == path {
			c.Libraries.Movies.Paths = append(c.Libraries.Movies.Paths[:i], c.Libraries.Movies.Paths[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("path not found: %s", path)
}

// RemoveTVPath removes a TV show library path
func (c *Config) RemoveTVPath(path string) error {
	for i, existing := range c.Libraries.TV.Paths {
		if existing == path {
			c.Libraries.TV.Paths = append(c.Libraries.TV.Paths[:i], c.Libraries.TV.Paths[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("path not found: %s", path)
}

// GetAllPaths returns all configured library paths
func (c *Config) GetAllPaths() []string {
	return append(c.Libraries.Movies.Paths, c.Libraries.TV.Paths...)
}
