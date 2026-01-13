package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/openbook/population-scripts/client/sportradar"
	"github.com/openbook/shared/envloader"
)

// Season type constants
const (
	SeasonTypeRegular    = "REG" // Regular season
	SeasonTypePostSeason = "PST" // Post-season/Playoffs
	SeasonTypePreSeason  = "PRE" // Pre-season (NFL only)
)

// BaseConfig holds common configuration shared across all scripts
type BaseConfig struct {
	// API Configuration
	SportradarAPIKeys     []sportradar.ApiKeyParameters
	SportradarAccessLevel sportradar.AccessLevel

	// Rate Limiting Configuration
	RateLimitDelayMilliseconds int

	// Database Configuration
	PGHost     string
	PGPort     string
	PGDatabase string
	PGUser     string
	PGPassword string
	PGKeyPath  string // Path to SSL certificate file
}

// ReferenceDataConfig holds configuration for the reference data population script
type ReferenceDataConfig struct {
	BaseConfig

	// Season Configuration
	NFLSeasonStartYear int    // Start year for NFL season (default: current year)
	NFLSeasonType      string // Type of NFL season: SeasonTypeRegular, SeasonTypePostSeason, SeasonTypePreSeason (default: SeasonTypeRegular)
	NFLWeek            int    // NFL week for injury data (default: 1)
	NBASeasonStartYear int    // Start year for NBA season (default: current year)
	NBASeasonType      string // Type of NBA season: SeasonTypeRegular, SeasonTypePostSeason (default: SeasonTypeRegular)
}

// PlayByPlayConfig holds configuration for the play-by-play stats script
type PlayByPlayConfig struct {
	BaseConfig

	// Game Configuration
	NFLGameID int // Database game ID (not vendor UUID)
	NBAGameID int // Database game ID (not vendor UUID)
}

// BoxScoreConfig holds configuration for the box score generation script
type BoxScoreConfig struct {
	BaseConfig

	// Game Configuration
	NFLGameID int // Database game ID (not vendor UUID)
	NBAGameID int // Database game ID (not vendor UUID)
}

// CompareBoxScoreConfig holds configuration for the box score comparison tool
type CompareBoxScoreConfig struct {
	// Database Configuration
	PGHost     string
	PGPort     string
	PGDatabase string
	PGUser     string
	PGPassword string
	PGKeyPath  string

	// Sportradar API Configuration
	SportradarAPIKeys                    []sportradar.ApiKeyParameters
	SportradarAccessLevel                sportradar.AccessLevel
	SportradarRateLimitDelayMilliseconds int

	// NFL Date Range (inclusive)
	NFLGameDateStartInclusive time.Time
	NFLGameDateEndInclusive   time.Time

	// NBA Date Range (inclusive)
	NBAGameDateStartInclusive time.Time
	NBAGameDateEndInclusive   time.Time
}

// BatchUpdateConfig holds configuration for the batch update script
type BatchUpdateConfig struct {
	BaseConfig

	// NFL Date Range (inclusive)
	NFLGameDateStartInclusive time.Time
	NFLGameDateEndInclusive   time.Time

	// NBA Date Range (inclusive)
	NBAGameDateStartInclusive time.Time
	NBAGameDateEndInclusive   time.Time
}

// parseApiKeys parses the SPORTRADAR_API_KEYS environment variable.
// Format: "key1:limit1,key2:limit2,..."
// Returns nil if the string is empty.
// Returns an error if the format is invalid.
func parseApiKeys(envValue string) ([]sportradar.ApiKeyParameters, error) {
	if envValue == "" {
		return nil, nil
	}

	var keys []sportradar.ApiKeyParameters
	pairs := strings.Split(envValue, ",")

	for i, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		parts := strings.SplitN(pair, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid format for API key %d: expected 'key:limit', got '%s'", i+1, pair)
		}

		apiKey := strings.TrimSpace(parts[0])
		limitStr := strings.TrimSpace(parts[1])

		if apiKey == "" {
			return nil, fmt.Errorf("empty API key in entry %d", i+1)
		}

		limit, err := strconv.Atoi(limitStr)
		if err != nil {
			return nil, fmt.Errorf("invalid request limit for API key %d: '%s' is not a valid integer", i+1, limitStr)
		}

		if limit <= 0 {
			return nil, fmt.Errorf("request limit for API key %d must be positive, got %d", i+1, limit)
		}

		keys = append(keys, sportradar.ApiKeyParameters{
			ApiKey:       apiKey,
			RequestLimit: limit,
		})
	}

	return keys, nil
}

// loadBaseConfig reads common configuration from environment variables
func loadBaseConfig() BaseConfig {
	accessLevel := sportradar.AccessLevel(strings.ToLower(os.Getenv("SPORTRADAR_ACCESS_LEVEL")))

	// Parse API keys - errors are handled in Validate()
	apiKeys, _ := parseApiKeys(os.Getenv("SPORTRADAR_API_KEYS"))

	return BaseConfig{
		SportradarAPIKeys:          apiKeys,
		SportradarAccessLevel:      accessLevel,
		RateLimitDelayMilliseconds: envloader.GetEnvAsIntWithDefault("SPORTRADAR_RATE_LIMIT_DELAY_MS", 0),
		PGHost:                     os.Getenv("PG_HOST"),
		PGPort:                     os.Getenv("PG_PORT"),
		PGDatabase:                 os.Getenv("PG_DATABASE"),
		PGUser:                     os.Getenv("PG_USER"),
		PGPassword:                 os.Getenv("PG_PASSWORD"),
		PGKeyPath:                  os.Getenv("PG_KEY_PATH"),
	}
}

// Validate checks that all required base configuration fields are set
func (c *BaseConfig) Validate() error {
	// Validate API keys - re-parse to get detailed error message
	apiKeysEnv := os.Getenv("SPORTRADAR_API_KEYS")
	if apiKeysEnv == "" {
		return fmt.Errorf("missing required environment variable: SPORTRADAR_API_KEYS (format: key1:limit1,key2:limit2)")
	}
	apiKeys, err := parseApiKeys(apiKeysEnv)
	if err != nil {
		return fmt.Errorf("invalid SPORTRADAR_API_KEYS: %w", err)
	}
	if len(apiKeys) == 0 {
		return fmt.Errorf("SPORTRADAR_API_KEYS must contain at least one key:limit pair")
	}
	c.SportradarAPIKeys = apiKeys

	// Validate access level
	if c.SportradarAccessLevel == "" {
		return fmt.Errorf("missing required environment variable: SPORTRADAR_ACCESS_LEVEL")
	}
	if err := c.SportradarAccessLevel.Validate(); err != nil {
		return fmt.Errorf("invalid SPORTRADAR_ACCESS_LEVEL: %w", err)
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
	if c.RateLimitDelayMilliseconds <= 0 {
		return fmt.Errorf("missing or invalid required environment variable: SPORTRADAR_RATE_LIMIT_DELAY_MS (must be a positive integer)")
	}

	return nil
}

// LoadReferenceDataConfigFromFile loads environment variables from the specified file, then reads configuration
// If envFile is empty, it will load from .env in the current directory (required)
// Validates that all required configuration variables are set
func LoadReferenceDataConfigFromFile(envFile string) (*ReferenceDataConfig, error) {
	// Load .env file - fail if not found
	if err := envloader.LoadEnvFile(envFile); err != nil {
		return nil, err
	}

	cfg := LoadReferenceDataConfig()

	// Validate required fields
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// LoadReferenceDataConfig reads reference data configuration from environment variables
// Required fields (no defaults): SPORTRADAR_API_KEY, SPORTRADAR_ACCESS_LEVEL, SPORTRADAR_RATE_LIMIT_DELAY_MS,
//
//	PG_HOST, PG_PORT, PG_DATABASE, PG_USER, PG_PASSWORD, PG_KEY_PATH,
//	NFL_SEASON_START_YEAR, NFL_SEASON_TYPE, NFL_WEEK, NBA_SEASON_START_YEAR, NBA_SEASON_TYPE
//
// This does not load any .env files - use LoadReferenceDataConfigFromFile for that
func LoadReferenceDataConfig() *ReferenceDataConfig {
	return &ReferenceDataConfig{
		BaseConfig:         loadBaseConfig(),
		NFLSeasonStartYear: envloader.GetEnvAsIntWithDefault("NFL_SEASON_START_YEAR", 0),
		NFLSeasonType:      strings.ToUpper(os.Getenv("NFL_SEASON_TYPE")),
		NFLWeek:            envloader.GetEnvAsIntWithDefault("NFL_WEEK", 0),
		NBASeasonStartYear: envloader.GetEnvAsIntWithDefault("NBA_SEASON_START_YEAR", 0),
		NBASeasonType:      strings.ToUpper(os.Getenv("NBA_SEASON_TYPE")),
	}
}

// Validate checks that all required reference data configuration fields are set
func (c *ReferenceDataConfig) Validate() error {
	// Validate base config first
	if err := c.BaseConfig.Validate(); err != nil {
		return err
	}

	// Season configuration validation
	if c.NFLSeasonStartYear <= 0 {
		return fmt.Errorf("missing or invalid required environment variable: NFL_SEASON_START_YEAR (must be a positive integer)")
	}
	if c.NFLSeasonType == "" {
		return fmt.Errorf("missing required environment variable: NFL_SEASON_TYPE")
	}
	if c.NFLWeek <= 0 {
		return fmt.Errorf("missing or invalid required environment variable: NFL_WEEK (must be a positive integer)")
	}
	if c.NBASeasonStartYear <= 0 {
		return fmt.Errorf("missing or invalid required environment variable: NBA_SEASON_START_YEAR (must be a positive integer)")
	}
	if c.NBASeasonType == "" {
		return fmt.Errorf("missing required environment variable: NBA_SEASON_TYPE")
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

// LoadPlayByPlayConfigFromFile loads environment variables from the specified file, then reads configuration
// If envFile is empty, it will load from .env in the current directory (required)
// Validates that all required configuration variables are set
func LoadPlayByPlayConfigFromFile(envFile string) (*PlayByPlayConfig, error) {
	// Load .env file - fail if not found
	if err := envloader.LoadEnvFile(envFile); err != nil {
		return nil, err
	}

	cfg := LoadPlayByPlayConfig()

	// Validate required fields
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// LoadPlayByPlayConfig reads play-by-play configuration from environment variables
// Required fields (no defaults): SPORTRADAR_API_KEY, SPORTRADAR_ACCESS_LEVEL, SPORTRADAR_RATE_LIMIT_DELAY_MS,
//
//	PG_HOST, PG_PORT, PG_DATABASE, PG_USER, PG_PASSWORD, PG_KEY_PATH, NFL_GAME_ID or NBA_GAME_ID
//
// This does not load any .env files - use LoadPlayByPlayConfigFromFile for that
func LoadPlayByPlayConfig() *PlayByPlayConfig {
	return &PlayByPlayConfig{
		BaseConfig: loadBaseConfig(),
		NFLGameID:  envloader.GetEnvAsIntWithDefault("NFL_GAME_ID", 0),
		NBAGameID:  envloader.GetEnvAsIntWithDefault("NBA_GAME_ID", 0),
	}
}

// Validate checks that all required play-by-play configuration fields are set
func (c *PlayByPlayConfig) Validate() error {
	// Validate base config first
	if err := c.BaseConfig.Validate(); err != nil {
		return err
	}

	// Play-by-play specific validation - at least one game ID must be set
	if c.NFLGameID == 0 && c.NBAGameID == 0 {
		return fmt.Errorf("missing or invalid required environment variable: NFL_GAME_ID or NBA_GAME_ID (must be a positive integer database ID)")
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

// LoadBoxScoreConfigFromFile loads environment variables from the specified file, then reads configuration
// If envFile is empty, it will load from .env in the current directory (required)
// Validates that all required configuration variables are set
func LoadBoxScoreConfigFromFile(envFile string) (*BoxScoreConfig, error) {
	// Load .env file - fail if not found
	if err := envloader.LoadEnvFile(envFile); err != nil {
		return nil, err
	}

	cfg := LoadBoxScoreConfig()

	// Validate required fields
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// LoadBoxScoreConfig reads box score configuration from environment variables
// Required fields (no defaults): PG_HOST, PG_PORT, PG_DATABASE, PG_USER, PG_PASSWORD, PG_KEY_PATH, NFL_GAME_ID or NBA_GAME_ID
// Note: SPORTRADAR_API_KEY and SPORTRADAR_RATE_LIMIT_DELAY_MS are not required for box score generation (no API calls)
//
// This does not load any .env files - use LoadBoxScoreConfigFromFile for that
func LoadBoxScoreConfig() *BoxScoreConfig {
	return &BoxScoreConfig{
		BaseConfig: loadBaseConfig(),
		NFLGameID:  envloader.GetEnvAsIntWithDefault("NFL_GAME_ID", 0),
		NBAGameID:  envloader.GetEnvAsIntWithDefault("NBA_GAME_ID", 0),
	}
}

// Validate checks that all required box score configuration fields are set
func (c *BoxScoreConfig) Validate() error {
	// Validate database config (skip API key validation since we don't need it)
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

	// Box score specific validation - at least one game ID must be set
	if c.NFLGameID == 0 && c.NBAGameID == 0 {
		return fmt.Errorf("missing or invalid required environment variable: NFL_GAME_ID or NBA_GAME_ID (must be a positive integer database ID)")
	}

	return nil
}

// LoadCompareBoxScoreConfigFromFile loads environment variables from the specified file, then reads configuration
// If envFile is empty, it will load from .env in the current directory (required)
// Validates that all required configuration variables are set
func LoadCompareBoxScoreConfigFromFile(envFile string) (*CompareBoxScoreConfig, error) {
	// Load .env file - fail if not found
	if err := envloader.LoadEnvFile(envFile); err != nil {
		return nil, err
	}

	cfg := LoadCompareBoxScoreConfig()

	// Validate required fields
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// LoadCompareBoxScoreConfig reads compare box score configuration from environment variables
// Required fields (no defaults): PG_HOST, PG_PORT, PG_DATABASE, PG_USER, PG_PASSWORD, PG_KEY_PATH,
//
//	SPORTRADAR_API_KEYS, SPORTRADAR_ACCESS_LEVEL, SPORTRADAR_RATE_LIMIT_DELAY_MS
//
// Date ranges (format: YYYY-MM-DD):
//
//	NFL: NFL_GAME_DATE_START_INCLUSIVE, NFL_GAME_DATE_END_INCLUSIVE
//	NBA: NBA_GAME_DATE_START_INCLUSIVE, NBA_GAME_DATE_END_INCLUSIVE
//
// This does not load any .env files - use LoadCompareBoxScoreConfigFromFile for that
func LoadCompareBoxScoreConfig() *CompareBoxScoreConfig {
	accessLevel := sportradar.AccessLevel(strings.ToLower(os.Getenv("SPORTRADAR_ACCESS_LEVEL")))
	apiKeys, _ := parseApiKeys(os.Getenv("SPORTRADAR_API_KEYS"))

	return &CompareBoxScoreConfig{
		PGHost:                               os.Getenv("PG_HOST"),
		PGPort:                               os.Getenv("PG_PORT"),
		PGDatabase:                           os.Getenv("PG_DATABASE"),
		PGUser:                               os.Getenv("PG_USER"),
		PGPassword:                           os.Getenv("PG_PASSWORD"),
		PGKeyPath:                            os.Getenv("PG_KEY_PATH"),
		SportradarAPIKeys:                    apiKeys,
		SportradarAccessLevel:                accessLevel,
		SportradarRateLimitDelayMilliseconds: envloader.GetEnvAsIntWithDefault("SPORTRADAR_RATE_LIMIT_DELAY_MS", 0),
		NFLGameDateStartInclusive:            parseDate(os.Getenv("NFL_GAME_DATE_START_INCLUSIVE")),
		NFLGameDateEndInclusive:              parseDate(os.Getenv("NFL_GAME_DATE_END_INCLUSIVE")),
		NBAGameDateStartInclusive:            parseDate(os.Getenv("NBA_GAME_DATE_START_INCLUSIVE")),
		NBAGameDateEndInclusive:              parseDate(os.Getenv("NBA_GAME_DATE_END_INCLUSIVE")),
	}
}

// Validate checks that all required compare box score configuration fields are set
func (c *CompareBoxScoreConfig) Validate() error {
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

	// Sportradar API configuration validation
	apiKeysEnv := os.Getenv("SPORTRADAR_API_KEYS")
	if apiKeysEnv == "" {
		return fmt.Errorf("missing required environment variable: SPORTRADAR_API_KEYS (format: key1:limit1,key2:limit2)")
	}
	apiKeys, err := parseApiKeys(apiKeysEnv)
	if err != nil {
		return fmt.Errorf("invalid SPORTRADAR_API_KEYS: %w", err)
	}
	if len(apiKeys) == 0 {
		return fmt.Errorf("SPORTRADAR_API_KEYS must contain at least one key:limit pair")
	}
	c.SportradarAPIKeys = apiKeys

	if c.SportradarAccessLevel == "" {
		return fmt.Errorf("missing required environment variable: SPORTRADAR_ACCESS_LEVEL")
	}
	if err := c.SportradarAccessLevel.Validate(); err != nil {
		return fmt.Errorf("invalid SPORTRADAR_ACCESS_LEVEL: %w", err)
	}

	if c.SportradarRateLimitDelayMilliseconds <= 0 {
		return fmt.Errorf("missing or invalid required environment variable: SPORTRADAR_RATE_LIMIT_DELAY_MS (must be a positive integer)")
	}

	// Validate NFL date range
	if c.NFLGameDateStartInclusive.IsZero() || c.NFLGameDateEndInclusive.IsZero() {
		return fmt.Errorf("missing required environment variables: NFL_GAME_DATE_START_INCLUSIVE and NFL_GAME_DATE_END_INCLUSIVE (format: YYYY-MM-DD)")
	}
	if c.NFLGameDateStartInclusive.After(c.NFLGameDateEndInclusive) {
		return fmt.Errorf("NFL_GAME_DATE_START_INCLUSIVE must be before or equal to NFL_GAME_DATE_END_INCLUSIVE")
	}

	// Validate NBA date range
	if c.NBAGameDateStartInclusive.IsZero() || c.NBAGameDateEndInclusive.IsZero() {
		return fmt.Errorf("missing required environment variables: NBA_GAME_DATE_START_INCLUSIVE and NBA_GAME_DATE_END_INCLUSIVE (format: YYYY-MM-DD)")
	}
	if c.NBAGameDateStartInclusive.After(c.NBAGameDateEndInclusive) {
		return fmt.Errorf("NBA_GAME_DATE_START_INCLUSIVE must be before or equal to NBA_GAME_DATE_END_INCLUSIVE")
	}

	return nil
}

// LoadBatchUpdateConfigFromFile loads environment variables from the specified file, then reads configuration
// If envFile is empty, it will load from .env in the current directory (required)
// Validates that all required configuration variables are set
func LoadBatchUpdateConfigFromFile(envFile string) (*BatchUpdateConfig, error) {
	// Load .env file - fail if not found
	if err := envloader.LoadEnvFile(envFile); err != nil {
		return nil, err
	}

	cfg := LoadBatchUpdateConfig()

	// Validate required fields
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// LoadBatchUpdateConfig reads batch update configuration from environment variables
// Required fields: SPORTRADAR_API_KEY, SPORTRADAR_ACCESS_LEVEL, SPORTRADAR_RATE_LIMIT_DELAY_MS,
//
//	PG_HOST, PG_PORT, PG_DATABASE, PG_USER, PG_PASSWORD, PG_KEY_PATH
//
// At least one date range must be set (NFL or NBA)
// Date format: YYYY-MM-DD
//
// This does not load any .env files - use LoadBatchUpdateConfigFromFile for that
func LoadBatchUpdateConfig() *BatchUpdateConfig {
	return &BatchUpdateConfig{
		BaseConfig:                loadBaseConfig(),
		NFLGameDateStartInclusive: parseDate(os.Getenv("NFL_GAME_DATE_START_INCLUSIVE")),
		NFLGameDateEndInclusive:   parseDate(os.Getenv("NFL_GAME_DATE_END_INCLUSIVE")),
		NBAGameDateStartInclusive: parseDate(os.Getenv("NBA_GAME_DATE_START_INCLUSIVE")),
		NBAGameDateEndInclusive:   parseDate(os.Getenv("NBA_GAME_DATE_END_INCLUSIVE")),
	}
}

// parseDate parses a date string in YYYY-MM-DD format, returns zero time if empty or invalid
func parseDate(dateStr string) time.Time {
	if dateStr == "" {
		return time.Time{}
	}
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return time.Time{}
	}
	return t
}

// Validate checks that all required batch update configuration fields are set
func (c *BatchUpdateConfig) Validate() error {
	// Validate base config first
	if err := c.BaseConfig.Validate(); err != nil {
		return err
	}

	// At least one complete date range must be set
	nflRangeSet := !c.NFLGameDateStartInclusive.IsZero() && !c.NFLGameDateEndInclusive.IsZero()
	nbaRangeSet := !c.NBAGameDateStartInclusive.IsZero() && !c.NBAGameDateEndInclusive.IsZero()

	if !nflRangeSet && !nbaRangeSet {
		return fmt.Errorf("at least one complete date range must be set: NFL_GAME_DATE_START_INCLUSIVE/NFL_GAME_DATE_END_INCLUSIVE or NBA_GAME_DATE_START_INCLUSIVE/NBA_GAME_DATE_END_INCLUSIVE")
	}

	// Validate partial NFL range
	if (!c.NFLGameDateStartInclusive.IsZero() && c.NFLGameDateEndInclusive.IsZero()) ||
		(c.NFLGameDateStartInclusive.IsZero() && !c.NFLGameDateEndInclusive.IsZero()) {
		return fmt.Errorf("NFL date range incomplete: both NFL_GAME_DATE_START_INCLUSIVE and NFL_GAME_DATE_END_INCLUSIVE must be set together")
	}

	// Validate partial NBA range
	if (!c.NBAGameDateStartInclusive.IsZero() && c.NBAGameDateEndInclusive.IsZero()) ||
		(c.NBAGameDateStartInclusive.IsZero() && !c.NBAGameDateEndInclusive.IsZero()) {
		return fmt.Errorf("NBA date range incomplete: both NBA_GAME_DATE_START_INCLUSIVE and NBA_GAME_DATE_END_INCLUSIVE must be set together")
	}

	// Validate date order
	if nflRangeSet && c.NFLGameDateStartInclusive.After(c.NFLGameDateEndInclusive) {
		return fmt.Errorf("NFL_GAME_DATE_START_INCLUSIVE must be before or equal to NFL_GAME_DATE_END_INCLUSIVE")
	}
	if nbaRangeSet && c.NBAGameDateStartInclusive.After(c.NBAGameDateEndInclusive) {
		return fmt.Errorf("NBA_GAME_DATE_START_INCLUSIVE must be before or equal to NBA_GAME_DATE_END_INCLUSIVE")
	}

	return nil
}

// HasNFLDateRange returns true if a complete NFL date range is configured
func (c *BatchUpdateConfig) HasNFLDateRange() bool {
	return !c.NFLGameDateStartInclusive.IsZero() && !c.NFLGameDateEndInclusive.IsZero()
}

// HasNBADateRange returns true if a complete NBA date range is configured
func (c *BatchUpdateConfig) HasNBADateRange() bool {
	return !c.NBAGameDateStartInclusive.IsZero() && !c.NBAGameDateEndInclusive.IsZero()
}
