package fetcher

import (
	"encoding/json"
	"fmt"

	"github.com/openbook/population-scripts/client/sportradar"
)

// PlayerPropsMappingResponse represents the API response for any player props mapping endpoint.
// All three mapping endpoints (sport_events, competitors, players) share the same structure.
type PlayerPropsMappingResponse struct {
	GeneratedAt string                `json:"generated_at"`
	Mappings    []PlayerPropsMapping  `json:"mappings"`
}

// PlayerPropsMapping represents a single mapping entry from the player props API.
// ExternalID is the Sportradar UUID used in sport-specific APIs (e.g., NFL/NBA).
// ID is the Sportradar player props ID (e.g., "sr:sport_event:12345").
type PlayerPropsMapping struct {
	ExternalID string `json:"external_id"`
	ID         string `json:"id"`
}

// FetchPlayerPropsSportEventMappings fetches sport event mappings from the player props API.
// Returns the raw API response struct.
func FetchPlayerPropsSportEventMappings(client *sportradar.Client) (*PlayerPropsMappingResponse, error) {
	data, err := client.GetPlayerPropsSportEventMappings()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch sport event mappings: %w", err)
	}

	var response PlayerPropsMappingResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse sport event mappings response: %w", err)
	}

	return &response, nil
}

// FetchPlayerPropsCompetitorMappings fetches competitor mappings from the player props API.
// Returns the raw API response struct.
func FetchPlayerPropsCompetitorMappings(client *sportradar.Client) (*PlayerPropsMappingResponse, error) {
	data, err := client.GetPlayerPropsCompetitorMappings()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch competitor mappings: %w", err)
	}

	var response PlayerPropsMappingResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse competitor mappings response: %w", err)
	}

	return &response, nil
}

// FetchPlayerPropsPlayerMappings fetches player mappings from the player props API.
// Returns the raw API response struct.
func FetchPlayerPropsPlayerMappings(client *sportradar.Client) (*PlayerPropsMappingResponse, error) {
	data, err := client.GetPlayerPropsPlayerMappings()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch player mappings: %w", err)
	}

	var response PlayerPropsMappingResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse player mappings response: %w", err)
	}

	return &response, nil
}
