package oddsblaze

import (
	"fmt"
	"time"

	"github.com/go-resty/resty/v2"
)

const (
	BaseURL       = "https://odds.oddsblaze.com"
	RewindBaseURL = "https://rewind.odds.oddsblaze.com"
	GraderBaseURL = "https://grader.oddsblaze.com"
)

// ClientConfig holds configuration for the OddsBlaze API client
type ClientConfig struct {
	APIKey         string
	Timeout        time.Duration
	RateLimitDelay time.Duration
}

// Client is the OddsBlaze API client
type Client struct {
	httpClient     *resty.Client
	apiKey         string
	rateLimitDelay time.Duration
}

// NewClient creates a new OddsBlaze API client
func NewClient(config *ClientConfig) *Client {
	httpClient := resty.New().
		SetTimeout(config.Timeout).
		SetHeader("Accept", "application/json")

	return &Client{
		httpClient:     httpClient,
		apiKey:         config.APIKey,
		rateLimitDelay: config.RateLimitDelay,
	}
}

// rateLimitWait waits for the configured rate limit delay.
// Called automatically before each API request.
func (c *Client) rateLimitWait() {
	if c.rateLimitDelay > 0 {
		time.Sleep(c.rateLimitDelay)
	}
}

// GetOdds fetches odds data for a given sportsbook and league.
// If timestamp is non-nil, uses the rewind endpoint for historical data.
// Returns raw JSON bytes.
func (c *Client) GetOdds(sportsbook, league string, timestamp *string) ([]byte, error) {
	c.rateLimitWait()

	baseURL := BaseURL
	if timestamp != nil {
		baseURL = RewindBaseURL
	}

	req := c.httpClient.R().
		SetQueryParam("key", c.apiKey).
		SetQueryParam("sportsbook", sportsbook).
		SetQueryParam("league", league).
		SetQueryParam("price", "decimal")

	if timestamp != nil {
		req.SetQueryParam("timestamp", *timestamp)
	}

	resp, err := req.Get(baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get OddsBlaze odds for %s/%s: %w", sportsbook, league, err)
	}

	if resp.StatusCode() != 200 {
		return nil, formatAPIError(resp.StatusCode(), baseURL, resp.String())
	}

	return resp.Body(), nil
}

// GetGraderResult fetches the grading result for a single OddsBlaze market ID.
// Returns raw JSON bytes.
func (c *Client) GetGraderResult(oddsBlazeID string) ([]byte, error) {
	c.rateLimitWait()

	resp, err := c.httpClient.R().
		SetQueryParam("key", c.apiKey).
		SetQueryParam("id", oddsBlazeID).
		Get(GraderBaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get OddsBlaze grader result for id %s: %w", oddsBlazeID, err)
	}

	if resp.StatusCode() != 200 {
		return nil, formatAPIError(resp.StatusCode(), GraderBaseURL, resp.String())
	}

	return resp.Body(), nil
}

// formatAPIError creates a user-friendly error message based on HTTP status code
func formatAPIError(statusCode int, url string, body string) error {
	switch statusCode {
	case 401:
		return fmt.Errorf("invalid API key (HTTP 401) - please check your ODDS_BLAZE_API_KEY\n  URL: %s", url)
	case 403:
		return fmt.Errorf("API access forbidden (HTTP 403) - check your API key permissions\n  URL: %s", url)
	case 429:
		return fmt.Errorf("API rate limit exceeded (HTTP 429) - try again later\n  URL: %s", url)
	default:
		return fmt.Errorf("OddsBlaze API returned status %d\n  URL: %s\n  Response: %s", statusCode, url, body)
	}
}
