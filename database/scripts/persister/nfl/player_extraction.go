package nfl

import (
	fetcher_nfl "github.com/openbook/population-scripts/fetcher/nfl"
)

// ExtractPlayerVendorIDs extracts all unique player vendor IDs from play-by-play data.
// Only extracts players from statistics that will actually be persisted (passes filter checks).
func ExtractPlayerVendorIDs(pbp *fetcher_nfl.PlayByPlayResponse) []string {
	seen := make(map[string]bool)
	var vendorIDs []string

	for _, period := range pbp.Periods {
		for _, drive := range period.PBP {
			if !shouldPersistDrive(&drive) {
				continue
			}

			// For standalone play entries (e.g., extra points after special teams TDs),
			// statistics are at the drive level, not nested in events.
			if drive.IsStandalonePlayDrive() {
				event := drive.AsEvent()
				extractPlayerVendorIDsFromEvent(event, seen, &vendorIDs)
				continue
			}

			// Process each event in the drive
			for _, event := range drive.Events {
				if !shouldPersistPlay(&event) {
					continue
				}
				extractPlayerVendorIDsFromEvent(&event, seen, &vendorIDs)
			}
		}
	}

	return vendorIDs
}

// extractPlayerVendorIDsFromEvent extracts player vendor IDs from a single event's statistics
func extractPlayerVendorIDsFromEvent(event *fetcher_nfl.Event, seen map[string]bool, vendorIDs *[]string) {
	for _, stat := range event.Statistics {
		if !shouldPersistPlayStatistic(&stat) {
			continue
		}
		if stat.Player != nil && stat.Player.ID != "" && !seen[stat.Player.ID] {
			seen[stat.Player.ID] = true
			*vendorIDs = append(*vendorIDs, stat.Player.ID)
		}
	}
}
