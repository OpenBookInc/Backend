package fetcher

import (
	"encoding/json"
	"fmt"

	"github.com/openbook/population-scripts/client/sportradar"
)

// FetchNFLGameStatistics fetches and parses NFL game statistics from Sportradar
func FetchNFLGameStatistics(client *sportradar.Client, gameSportradarID string) (*NFLGameStatistics, error) {
	data, err := client.GetNFLGameStatistics(gameSportradarID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch NFL game statistics for game %s: %w", gameSportradarID, err)
	}

	var stats NFLGameStatistics
	if err := json.Unmarshal(data, &stats); err != nil {
		return nil, fmt.Errorf("failed to parse NFL game statistics for game %s: %w", gameSportradarID, err)
	}

	return &stats, nil
}
