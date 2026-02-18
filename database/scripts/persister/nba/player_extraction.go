package nba

import (
	fetcher_nba "github.com/openbook/population-scripts/fetcher/nba"
)

// ExtractPlayerSportradarIDs extracts all unique player vendor IDs from play-by-play data.
// Only extracts players from statistics that will actually be persisted (passes filter checks).
func ExtractPlayerSportradarIDs(pbp *fetcher_nba.PlayByPlayResponse) []string {
	seen := make(map[string]bool)
	var sportradarIDs []string

	for _, period := range pbp.Periods {
		for _, event := range period.Events {
			if !shouldPersistPlay(&event) {
				continue
			}

			for _, stat := range event.Statistics {
				if !shouldPersistPlayStatistic(&stat) {
					continue
				}
				if stat.Player != nil && stat.Player.ID != "" && !seen[stat.Player.ID] {
					seen[stat.Player.ID] = true
					sportradarIDs = append(sportradarIDs, stat.Player.ID)
				}
			}
		}
	}

	return sportradarIDs
}
