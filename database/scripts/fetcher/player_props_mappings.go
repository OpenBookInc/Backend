package fetcher

import (
	"encoding/json"
	"fmt"

	"github.com/openbook/population-scripts/client/sportradar"
)

const (
	// defaultPageLimit is the number of mappings to fetch per page
	defaultPageLimit = 1000
)

// PlayerPropsMappingResponse represents the API response for any player props mapping endpoint.
// All three mapping endpoints (sport_events, competitors, players) share the same structure.
type PlayerPropsMappingResponse struct {
	GeneratedAt string               `json:"generated_at"`
	Mappings    []PlayerPropsMapping `json:"mappings"`
}

// PlayerPropsMapping represents a single mapping entry from the player props API.
// ExternalID is the Sportradar UUID used in sport-specific APIs (e.g., NFL/NBA).
// ID is the Sportradar player props ID (e.g., "sr:sport_event:12345").
type PlayerPropsMapping struct {
	ExternalID string `json:"external_id"`
	ID         string `json:"id"`
}

// fetchAllPages fetches all pages from a paginated mapping endpoint.
// The fetchPage function is called with start and limit parameters and returns the raw JSON response.
func fetchAllPages(fetchPage func(start, limit int) ([]byte, error)) (*PlayerPropsMappingResponse, error) {
	var allMappings []PlayerPropsMapping
	var generatedAt string
	start := 0

	for {
		data, err := fetchPage(start, defaultPageLimit)
		if err != nil {
			return nil, err
		}

		var response PlayerPropsMappingResponse
		if err := json.Unmarshal(data, &response); err != nil {
			return nil, fmt.Errorf("failed to parse mappings response: %w", err)
		}

		if generatedAt == "" {
			generatedAt = response.GeneratedAt
		}

		allMappings = append(allMappings, response.Mappings...)

		// If we got fewer results than the limit, we've reached the end
		if len(response.Mappings) < defaultPageLimit {
			break
		}

		start += defaultPageLimit
	}

	return &PlayerPropsMappingResponse{
		GeneratedAt: generatedAt,
		Mappings:    allMappings,
	}, nil
}

// FetchPlayerPropsSportEventMappings fetches all sport event mappings from the player props API.
// Automatically paginates to retrieve all results.
func FetchPlayerPropsSportEventMappings(client *sportradar.Client) (*PlayerPropsMappingResponse, error) {
	response, err := fetchAllPages(func(start, limit int) ([]byte, error) {
		return client.GetPlayerPropsSportEventMappings(start, limit)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch sport event mappings: %w", err)
	}
	return response, nil
}

// FetchPlayerPropsCompetitorMappings fetches all competitor mappings from the player props API.
// Automatically paginates to retrieve all results.
func FetchPlayerPropsCompetitorMappings(client *sportradar.Client) (*PlayerPropsMappingResponse, error) {
	response, err := fetchAllPages(func(start, limit int) ([]byte, error) {
		return client.GetPlayerPropsCompetitorMappings(start, limit)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch competitor mappings: %w", err)
	}
	return response, nil
}

// FetchPlayerPropsPlayerMappings fetches all player mappings from the player props API.
// Automatically paginates to retrieve all results.
func FetchPlayerPropsPlayerMappings(client *sportradar.Client) (*PlayerPropsMappingResponse, error) {
	response, err := fetchAllPages(func(start, limit int) ([]byte, error) {
		return client.GetPlayerPropsPlayerMappings(start, limit)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch player mappings: %w", err)
	}
	return response, nil
}
