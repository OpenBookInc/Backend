package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/openbook/shared/envloader"
)

// Season type constants
const (
	SeasonTypeRegular    = "REG" // Regular season
	SeasonTypePostSeason = "PST" // Post-season/Playoffs
	SeasonTypePreSeason  = "PRE" // Pre-season (NFL only)
)

// Config holds all configuration for the application
type Config struct {
	// API Configuration
	SportradarAPIKey string

	// Rate Limiting Configuration
	RateLimitDelayMilliseconds int

	// Season Configuration
	NFLSeasonYear int    // Year for NFL season (default: current year)
	NFLSeasonType string // Type of NFL season: SeasonTypeRegular, SeasonTypePostSeason, SeasonTypePreSeason (default: SeasonTypeRegular)
	NFLWeek       int    // NFL week for injury data (default: 1)
	NBASeasonYear int    // Year for NBA season (default: current year)
	NBASeasonType string // Type of NBA season: SeasonTypeRegular, SeasonTypePostSeason (default: SeasonTypeRegular)

	// Database Configuration
	PGHost     string
	PGPort     string
	PGDatabase string
	PGUser     string
	PGPassword string
	PGKeyPath  string // Path to SSL certificate file
}

// LoadFromFile loads environment variables from the specified file, then reads configuration
// If envFile is empty, it will load from .env in the current directory (required)
// Validates that all required configuration variables are set
func LoadFromFile(envFile string) (*Config, error) {
	// Load .env file - fail if not found
	if err := envloader.LoadEnvFile(envFile); err != nil {
		return nil, err
	}

	cfg := Load()

	// Validate required fields
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Load reads configuration from environment variables
// Required fields (no defaults): SPORTRADAR_API_KEY, PG_HOST, PG_PORT, PG_DATABASE, PG_USER, PG_PASSWORD, PG_KEY_PATH
// Optional fields (with defaults):
//   - RATE_LIMIT_DELAY_MS (default: 1000)
//   - NFL_SEASON_YEAR (default: current year)
//   - NFL_SEASON_TYPE (default: "REG")
//   - NFL_WEEK (default: 1)
//   - NBA_SEASON_YEAR (default: current year)
//   - NBA_SEASON_TYPE (default: "REG")
// This does not load any .env files - use LoadFromFile for that
func Load() *Config {
	currentYear := time.Now().Year()

	cfg := &Config{
		SportradarAPIKey:           os.Getenv("SPORTRADAR_API_KEY"),
		RateLimitDelayMilliseconds: envloader.GetEnvAsIntWithDefault("RATE_LIMIT_DELAY_MS", 1000),
		NFLSeasonYear:              envloader.GetEnvAsIntWithDefault("NFL_SEASON_YEAR", currentYear),
		NFLSeasonType:              strings.ToUpper(envloader.GetEnvAsStringWithDefault("NFL_SEASON_TYPE", SeasonTypeRegular)),
		NFLWeek:                    envloader.GetEnvAsIntWithDefault("NFL_WEEK", 1),
		NBASeasonYear:              envloader.GetEnvAsIntWithDefault("NBA_SEASON_YEAR", currentYear),
		NBASeasonType:              strings.ToUpper(envloader.GetEnvAsStringWithDefault("NBA_SEASON_TYPE", SeasonTypeRegular)),
		PGHost:                     os.Getenv("PG_HOST"),
		PGPort:                     os.Getenv("PG_PORT"),
		PGDatabase:                 os.Getenv("PG_DATABASE"),
		PGUser:                     os.Getenv("PG_USER"),
		PGPassword:                 os.Getenv("PG_PASSWORD"),
		PGKeyPath:                  os.Getenv("PG_KEY_PATH"),
	}

	return cfg
}

// Validate checks that all required configuration fields are set
func (c *Config) Validate() error {
	if c.SportradarAPIKey == "" {
		return fmt.Errorf("missing required environment variable: SPORTRADAR_API_KEY")
	}

	// Database configuration validation
	if c.PGHost == "" {
		return fmt.Errorf("missing required environment variable: PG_HOST")
	}
	if c.PGPort == "" {
		return fmt.Errorf("missing required environment variable: PG_PORT")
	}
	if c.PGDatabase == "" {
		return fmt.Errorf("missing required environment variable: PG_DATABASE")
	}
	if c.PGUser == "" {
		return fmt.Errorf("missing required environment variable: PG_USER")
	}
	if c.PGPassword == "" {
		return fmt.Errorf("missing required environment variable: PG_PASSWORD")
	}
	if c.PGKeyPath == "" {
		return fmt.Errorf("missing required environment variable: PG_KEY_PATH")
	}

	// Season type validation
	validNFLSeasonTypes := []string{SeasonTypeRegular, SeasonTypePostSeason, SeasonTypePreSeason}
	if !isValidSeasonType(c.NFLSeasonType, validNFLSeasonTypes) {
		return fmt.Errorf("invalid NFL_SEASON_TYPE '%s': must be one of %v", c.NFLSeasonType, validNFLSeasonTypes)
	}

	validNBASeasonTypes := []string{SeasonTypeRegular, SeasonTypePostSeason}
	if !isValidSeasonType(c.NBASeasonType, validNBASeasonTypes) {
		return fmt.Errorf("invalid NBA_SEASON_TYPE '%s': must be one of %v", c.NBASeasonType, validNBASeasonTypes)
	}

	return nil
}

// isValidSeasonType checks if a season type is in the list of valid types (case-insensitive)
func isValidSeasonType(seasonType string, validTypes []string) bool {
	upperSeasonType := strings.ToUpper(seasonType)
	for _, valid := range validTypes {
		if upperSeasonType == valid {
			return true
		}
	}
	return false
}
