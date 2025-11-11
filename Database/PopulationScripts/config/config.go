package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
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
// If envFile is empty, it will try to load from .env (non-fatal if not found)
// Validates that all required configuration variables are set
func LoadFromFile(envFile string) (*Config, error) {
	// If no file specified, try to load .env (but don't fail if it doesn't exist)
	if envFile == "" {
		_ = godotenv.Load() // Ignore error - .env is optional
	} else {
		// If a specific file is requested, fail if it doesn't exist
		if err := godotenv.Load(envFile); err != nil {
			return nil, fmt.Errorf("failed to load environment file %s: %w", envFile, err)
		}
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
		RateLimitDelayMilliseconds: getEnvAsInt("RATE_LIMIT_DELAY_MS", 1000),
		NFLSeasonYear:              getEnvAsInt("NFL_SEASON_YEAR", currentYear),
		NFLSeasonType:              strings.ToUpper(getEnvAsString("NFL_SEASON_TYPE", SeasonTypeRegular)),
		NFLWeek:                    getEnvAsInt("NFL_WEEK", 1),
		NBASeasonYear:              getEnvAsInt("NBA_SEASON_YEAR", currentYear),
		NBASeasonType:              strings.ToUpper(getEnvAsString("NBA_SEASON_TYPE", SeasonTypeRegular)),
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

// getEnvAsInt reads an environment variable as an integer or returns a default value
func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

// getEnvAsString reads an environment variable as a string or returns a default value
func getEnvAsString(key string, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
