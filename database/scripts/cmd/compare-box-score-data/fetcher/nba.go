package fetcher

import (
	"encoding/json"
	"fmt"

	"github.com/openbook/population-scripts/client/sportradar"
)

// FetchNBAGameSummary fetches and parses NBA game summary from Sportradar
func FetchNBAGameSummary(client *sportradar.Client, gameVendorID string) (*NBAGameSummary, error) {
	data, err := client.GetNBAGameSummary(gameVendorID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch NBA game summary for game %s: %w", gameVendorID, err)
	}

	var summary NBAGameSummary
	if err := json.Unmarshal(data, &summary); err != nil {
		return nil, fmt.Errorf("failed to parse NBA game summary for game %s: %w", gameVendorID, err)
	}

	return &summary, nil
}
