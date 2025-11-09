package client

import (
	"fmt"
	"time"

	"github.com/go-resty/resty/v2"
)

const (
	// BaseURL is the Sportradar API base URL
	BaseURL = "https://api.sportradar.com"

	// API version paths
	NBABasePath = "/nba/trial/v8/en"
	NFLBasePath = "/nfl/official/trial/v7/en"
)

// ClientConfig holds configuration for the API client
type ClientConfig struct {
	// RateLimitDelay is the delay between API requests to respect rate limits
	RateLimitDelay time.Duration
	// Timeout is the HTTP request timeout
	Timeout time.Duration
}

// DefaultConfig returns the default client configuration
func DefaultConfig() *ClientConfig {
	return &ClientConfig{
		RateLimitDelay: 2 * time.Second, // Conservative default to avoid 429 errors
		Timeout:        30 * time.Second,
	}
}

// Client is the Sportradar API client
type Client struct {
	httpClient *resty.Client
	apiKey     string
	config     *ClientConfig
}

// NewClient creates a new Sportradar API client with default configuration
func NewClient(apiKey string) *Client {
	return NewClientWithConfig(apiKey, DefaultConfig())
}

// NewClientWithDelay creates a new Sportradar API client with custom rate limit delay
func NewClientWithDelay(apiKey string, rateLimitDelayMs int) *Client {
	config := DefaultConfig()
	config.RateLimitDelay = time.Duration(rateLimitDelayMs) * time.Millisecond
	return NewClientWithConfig(apiKey, config)
}

// NewClientWithConfig creates a new Sportradar API client with custom configuration
func NewClientWithConfig(apiKey string, config *ClientConfig) *Client {
	httpClient := resty.New().
		SetBaseURL(BaseURL).
		SetTimeout(config.Timeout).
		SetHeader("Accept", "application/json")

	return &Client{
		httpClient: httpClient,
		apiKey:     apiKey,
		config:     config,
	}
}

// RateLimitWait waits for the configured rate limit delay
func (c *Client) RateLimitWait() {
	if c.config.RateLimitDelay > 0 {
		time.Sleep(c.config.RateLimitDelay)
	}
}

// formatAPIError creates a user-friendly error message based on HTTP status code
func formatAPIError(statusCode int, body string) error {
	switch statusCode {
	case 401:
		return fmt.Errorf("invalid API key (HTTP 401) - please check your SPORTRADAR_API_KEY")
	case 403:
		return fmt.Errorf("API access forbidden (HTTP 403) - check your API key permissions")
	case 429:
		return fmt.Errorf("API rate limit exceeded (HTTP 429) - try increasing RATE_LIMIT_DELAY_SECONDS")
	case 404:
		return fmt.Errorf("API endpoint not found (HTTP 404) - %s", body)
	case 500, 502, 503, 504:
		return fmt.Errorf("Sportradar API server error (HTTP %d) - try again later", statusCode)
	default:
		return fmt.Errorf("API returned status %d: %s", statusCode, body)
	}
}

// GetNBATeams retrieves all NBA teams
func (c *Client) GetNBATeams() ([]byte, error) {
	resp, err := c.httpClient.R().
		SetQueryParam("api_key", c.apiKey).
		Get(fmt.Sprintf("%s/league/hierarchy.json", NBABasePath))

	if err != nil {
		return nil, fmt.Errorf("failed to get NBA teams: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, formatAPIError(resp.StatusCode(), resp.String())
	}

	return resp.Body(), nil
}

// GetNBATeamRoster retrieves roster for a specific NBA team
func (c *Client) GetNBATeamRoster(teamID string) ([]byte, error) {
	resp, err := c.httpClient.R().
		SetQueryParam("api_key", c.apiKey).
		Get(fmt.Sprintf("%s/teams/%s/profile.json", NBABasePath, teamID))

	if err != nil {
		return nil, fmt.Errorf("failed to get NBA team roster: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, formatAPIError(resp.StatusCode(), resp.String())
	}

	return resp.Body(), nil
}

// GetNFLTeams retrieves all NFL teams
func (c *Client) GetNFLTeams() ([]byte, error) {
	resp, err := c.httpClient.R().
		SetQueryParam("api_key", c.apiKey).
		Get(fmt.Sprintf("%s/league/hierarchy.json", NFLBasePath))

	if err != nil {
		return nil, fmt.Errorf("failed to get NFL teams: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, formatAPIError(resp.StatusCode(), resp.String())
	}

	return resp.Body(), nil
}

// GetNFLTeamRoster retrieves roster for a specific NFL team
func (c *Client) GetNFLTeamRoster(teamID string) ([]byte, error) {
	resp, err := c.httpClient.R().
		SetQueryParam("api_key", c.apiKey).
		Get(fmt.Sprintf("%s/teams/%s/full_roster.json", NFLBasePath, teamID))

	if err != nil {
		return nil, fmt.Errorf("failed to get NFL team roster: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, formatAPIError(resp.StatusCode(), resp.String())
	}

	return resp.Body(), nil
}
