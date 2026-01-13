package sportradar

import (
	"fmt"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
)

const (
	// BaseURL is the Sportradar API base URL
	BaseURL = "https://api.sportradar.com"
)

// ApiKeyParameters holds an API key and its request limit
type ApiKeyParameters struct {
	// ApiKey is the Sportradar API key
	ApiKey string
	// RequestLimit is the maximum number of requests allowed for this key
	RequestLimit int
}

// ClientConfig holds configuration for the API client
type ClientConfig struct {
	// AccessLevel is the Sportradar API access level (trial or production)
	AccessLevel AccessLevel
	// RateLimitDelay is the delay between API requests to respect rate limits
	RateLimitDelay time.Duration
	// Timeout is the HTTP request timeout
	Timeout time.Duration
	// ApiKeys is the list of API keys to rotate through (required, at least one)
	ApiKeys []ApiKeyParameters
}

// apiKeyRotation manages rotation through multiple API keys
type apiKeyRotation struct {
	keys                []ApiKeyParameters
	currentIndex        int
	currentRequestCount int
	mu                  sync.Mutex
}

// newApiKeyRotation creates a new API key rotator
func newApiKeyRotation(keys []ApiKeyParameters) *apiKeyRotation {
	return &apiKeyRotation{
		keys:                keys,
		currentIndex:        0,
		currentRequestCount: 0,
	}
}

// getKeyAndIncrement returns the current API key and increments the request count.
// If the current key's limit is reached, it rotates to the next key.
// Returns an error if all keys are exhausted.
// Note: Request count is incremented before the request is made for simplicity.
// This means failed requests still count toward the limit.
func (r *apiKeyRotation) getKeyAndIncrement() (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if we need to rotate to next key
	for r.currentIndex < len(r.keys) && r.currentRequestCount >= r.keys[r.currentIndex].RequestLimit {
		r.currentIndex++
		r.currentRequestCount = 0
		if r.currentIndex < len(r.keys) {
			fmt.Printf("API key rotation: switching to key %d of %d\n", r.currentIndex+1, len(r.keys))
		}
	}

	// Check if all keys are exhausted
	if r.currentIndex >= len(r.keys) {
		return "", fmt.Errorf("all API keys exhausted: used %d keys", len(r.keys))
	}

	r.currentRequestCount++
	return r.keys[r.currentIndex].ApiKey, nil
}

// Client is the Sportradar API client
type Client struct {
	httpClient  *resty.Client
	keyRotation *apiKeyRotation
	config      *ClientConfig
}

// getNBABasePath returns the NBA API base path for the configured access level
func (c *Client) getNBABasePath() string {
	return fmt.Sprintf("/nba/%s/v8/en", c.config.AccessLevel)
}

// getNFLBasePath returns the NFL API base path for the configured access level
func (c *Client) getNFLBasePath() string {
	return fmt.Sprintf("/nfl/official/%s/v7/en", c.config.AccessLevel)
}

// NewClientWithConfig creates a new Sportradar API client with custom configuration.
// ApiKeys must contain at least one key with a positive RequestLimit.
func NewClientWithConfig(config *ClientConfig) *Client {
	if len(config.ApiKeys) == 0 {
		panic("sportradar.NewClientWithConfig: ApiKeys must contain at least one key")
	}

	httpClient := resty.New().
		SetBaseURL(BaseURL).
		SetTimeout(config.Timeout).
		SetHeader("Accept", "application/json")

	return &Client{
		httpClient:  httpClient,
		keyRotation: newApiKeyRotation(config.ApiKeys),
		config:      config,
	}
}

// RateLimitWait waits for the configured rate limit delay
func (c *Client) RateLimitWait() {
	if c.config.RateLimitDelay > 0 {
		time.Sleep(c.config.RateLimitDelay)
	}
}

// formatAPIError creates a user-friendly error message based on HTTP status code
func formatAPIError(statusCode int, url string, body string) error {
	switch statusCode {
	case 401:
		return fmt.Errorf("invalid API key (HTTP 401) - please check your SPORTRADAR_API_KEY\n  URL: %s", url)
	case 403:
		return fmt.Errorf("API access forbidden (HTTP 403) - check your API key permissions\n  URL: %s", url)
	case 429:
		return fmt.Errorf("API rate limit exceeded (HTTP 429) - try increasing RATE_LIMIT_DELAY_MS\n  URL: %s", url)
	case 404:
		return fmt.Errorf("API endpoint not found (HTTP 404)\n  URL: %s\n  Response: %s", url, body)
	case 500, 502, 503, 504:
		return fmt.Errorf("Sportradar API server error (HTTP %d) - try again later\n  URL: %s", statusCode, url)
	default:
		return fmt.Errorf("API returned status %d\n  URL: %s\n  Response: %s", statusCode, url, body)
	}
}

// GetNBATeams retrieves all NBA teams
func (c *Client) GetNBATeams() ([]byte, error) {
	apiKey, err := c.keyRotation.getKeyAndIncrement()
	if err != nil {
		return nil, fmt.Errorf("failed to get NBA teams: %w", err)
	}

	url := fmt.Sprintf("%s/league/hierarchy.json", c.getNBABasePath())
	resp, err := c.httpClient.R().
		SetQueryParam("api_key", apiKey).
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("failed to get NBA teams: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, formatAPIError(resp.StatusCode(), BaseURL+url, resp.String())
	}

	return resp.Body(), nil
}

// GetNBATeamRoster retrieves roster for a specific NBA team
func (c *Client) GetNBATeamRoster(teamID string) ([]byte, error) {
	apiKey, err := c.keyRotation.getKeyAndIncrement()
	if err != nil {
		return nil, fmt.Errorf("failed to get NBA team roster: %w", err)
	}

	url := fmt.Sprintf("%s/teams/%s/profile.json", c.getNBABasePath(), teamID)
	resp, err := c.httpClient.R().
		SetQueryParam("api_key", apiKey).
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("failed to get NBA team roster: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, formatAPIError(resp.StatusCode(), BaseURL+url, resp.String())
	}

	return resp.Body(), nil
}

// GetNFLTeams retrieves all NFL teams
func (c *Client) GetNFLTeams() ([]byte, error) {
	apiKey, err := c.keyRotation.getKeyAndIncrement()
	if err != nil {
		return nil, fmt.Errorf("failed to get NFL teams: %w", err)
	}

	url := fmt.Sprintf("%s/league/hierarchy.json", c.getNFLBasePath())
	resp, err := c.httpClient.R().
		SetQueryParam("api_key", apiKey).
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("failed to get NFL teams: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, formatAPIError(resp.StatusCode(), BaseURL+url, resp.String())
	}

	return resp.Body(), nil
}

// GetNFLTeamRoster retrieves roster for a specific NFL team
func (c *Client) GetNFLTeamRoster(teamID string) ([]byte, error) {
	apiKey, err := c.keyRotation.getKeyAndIncrement()
	if err != nil {
		return nil, fmt.Errorf("failed to get NFL team roster: %w", err)
	}

	url := fmt.Sprintf("%s/teams/%s/full_roster.json", c.getNFLBasePath(), teamID)
	resp, err := c.httpClient.R().
		SetQueryParam("api_key", apiKey).
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("failed to get NFL team roster: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, formatAPIError(resp.StatusCode(), BaseURL+url, resp.String())
	}

	return resp.Body(), nil
}

// GetNFLSeasonSchedule retrieves the schedule for the current NFL season
func (c *Client) GetNFLSeasonSchedule(year int, seasonType string) ([]byte, error) {
	apiKey, err := c.keyRotation.getKeyAndIncrement()
	if err != nil {
		return nil, fmt.Errorf("failed to get NFL season schedule: %w", err)
	}

	url := fmt.Sprintf("%s/games/%d/%s/schedule.json", c.getNFLBasePath(), year, seasonType)
	resp, err := c.httpClient.R().
		SetQueryParam("api_key", apiKey).
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("failed to get NFL season schedule: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, formatAPIError(resp.StatusCode(), BaseURL+url, resp.String())
	}

	return resp.Body(), nil
}

// GetNFLWeeklyInjuries retrieves injury reports for a specific NFL week
func (c *Client) GetNFLWeeklyInjuries(year int, seasonType string, week int) ([]byte, error) {
	apiKey, err := c.keyRotation.getKeyAndIncrement()
	if err != nil {
		return nil, fmt.Errorf("failed to get NFL weekly injuries: %w", err)
	}

	url := fmt.Sprintf("%s/seasons/%d/%s/%d/injuries.json", c.getNFLBasePath(), year, seasonType, week)
	resp, err := c.httpClient.R().
		SetQueryParam("api_key", apiKey).
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("failed to get NFL weekly injuries: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, formatAPIError(resp.StatusCode(), BaseURL+url, resp.String())
	}

	return resp.Body(), nil
}

// GetNBASeasonSchedule retrieves the full NBA season schedule
func (c *Client) GetNBASeasonSchedule(year int, seasonType string) ([]byte, error) {
	apiKey, err := c.keyRotation.getKeyAndIncrement()
	if err != nil {
		return nil, fmt.Errorf("failed to get NBA season schedule: %w", err)
	}

	url := fmt.Sprintf("%s/games/%d/%s/schedule.json", c.getNBABasePath(), year, seasonType)
	resp, err := c.httpClient.R().
		SetQueryParam("api_key", apiKey).
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("failed to get NBA season schedule: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, formatAPIError(resp.StatusCode(), BaseURL+url, resp.String())
	}

	return resp.Body(), nil
}

// GetNBAInjuries retrieves current NBA injury reports for all teams
func (c *Client) GetNBAInjuries() ([]byte, error) {
	apiKey, err := c.keyRotation.getKeyAndIncrement()
	if err != nil {
		return nil, fmt.Errorf("failed to get NBA injuries: %w", err)
	}

	url := fmt.Sprintf("%s/league/injuries.json", c.getNBABasePath())
	resp, err := c.httpClient.R().
		SetQueryParam("api_key", apiKey).
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("failed to get NBA injuries: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, formatAPIError(resp.StatusCode(), BaseURL+url, resp.String())
	}

	return resp.Body(), nil
}

// GetNFLPlayByPlay retrieves play-by-play data for a specific NFL game
func (c *Client) GetNFLPlayByPlay(gameID string) ([]byte, error) {
	apiKey, err := c.keyRotation.getKeyAndIncrement()
	if err != nil {
		return nil, fmt.Errorf("failed to get NFL play-by-play: %w", err)
	}

	url := fmt.Sprintf("%s/games/%s/pbp.json", c.getNFLBasePath(), gameID)
	resp, err := c.httpClient.R().
		SetQueryParam("api_key", apiKey).
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("failed to get NFL play-by-play: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, formatAPIError(resp.StatusCode(), BaseURL+url, resp.String())
	}

	return resp.Body(), nil
}

// GetNBAPlayByPlay retrieves play-by-play data for a specific NBA game
func (c *Client) GetNBAPlayByPlay(gameID string) ([]byte, error) {
	apiKey, err := c.keyRotation.getKeyAndIncrement()
	if err != nil {
		return nil, fmt.Errorf("failed to get NBA play-by-play: %w", err)
	}

	url := fmt.Sprintf("%s/games/%s/pbp.json", c.getNBABasePath(), gameID)
	resp, err := c.httpClient.R().
		SetQueryParam("api_key", apiKey).
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("failed to get NBA play-by-play: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, formatAPIError(resp.StatusCode(), BaseURL+url, resp.String())
	}

	return resp.Body(), nil
}

// GetNFLPlayerProfile retrieves profile data for a specific NFL player
func (c *Client) GetNFLPlayerProfile(playerID string) ([]byte, error) {
	apiKey, err := c.keyRotation.getKeyAndIncrement()
	if err != nil {
		return nil, fmt.Errorf("failed to get NFL player profile: %w", err)
	}

	url := fmt.Sprintf("%s/players/%s/profile.json", c.getNFLBasePath(), playerID)
	resp, err := c.httpClient.R().
		SetQueryParam("api_key", apiKey).
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("failed to get NFL player profile: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, formatAPIError(resp.StatusCode(), BaseURL+url, resp.String())
	}

	return resp.Body(), nil
}

// GetNBAPlayerProfile retrieves profile data for a specific NBA player
func (c *Client) GetNBAPlayerProfile(playerID string) ([]byte, error) {
	apiKey, err := c.keyRotation.getKeyAndIncrement()
	if err != nil {
		return nil, fmt.Errorf("failed to get NBA player profile: %w", err)
	}

	url := fmt.Sprintf("%s/players/%s/profile.json", c.getNBABasePath(), playerID)
	resp, err := c.httpClient.R().
		SetQueryParam("api_key", apiKey).
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("failed to get NBA player profile: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, formatAPIError(resp.StatusCode(), BaseURL+url, resp.String())
	}

	return resp.Body(), nil
}

// GetNFLGameStatistics retrieves game statistics for a specific NFL game
func (c *Client) GetNFLGameStatistics(gameID string) ([]byte, error) {
	apiKey, err := c.keyRotation.getKeyAndIncrement()
	if err != nil {
		return nil, fmt.Errorf("failed to get NFL game statistics: %w", err)
	}

	url := fmt.Sprintf("%s/games/%s/statistics.json", c.getNFLBasePath(), gameID)
	resp, err := c.httpClient.R().
		SetQueryParam("api_key", apiKey).
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("failed to get NFL game statistics: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, formatAPIError(resp.StatusCode(), BaseURL+url, resp.String())
	}

	return resp.Body(), nil
}

// GetNBAGameSummary retrieves game summary for a specific NBA game
func (c *Client) GetNBAGameSummary(gameID string) ([]byte, error) {
	apiKey, err := c.keyRotation.getKeyAndIncrement()
	if err != nil {
		return nil, fmt.Errorf("failed to get NBA game summary: %w", err)
	}

	url := fmt.Sprintf("%s/games/%s/summary.json", c.getNBABasePath(), gameID)
	resp, err := c.httpClient.R().
		SetQueryParam("api_key", apiKey).
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("failed to get NBA game summary: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, formatAPIError(resp.StatusCode(), BaseURL+url, resp.String())
	}

	return resp.Body(), nil
}
