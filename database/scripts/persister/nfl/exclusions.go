package nfl

import fetcher_nfl "github.com/openbook/population-scripts/fetcher/nfl"

// Exclusion rules for filtering data during persistence.
// These exclusions prevent data from being written to the database.
// For fetch-level exclusions, see fetcher/exclusions.go.

// shouldPersistDrive determines whether a drive should be persisted to the database.
// Returns false for "event" type entries (timeouts, end-of-period markers, etc.).
// Returns true for "drive" type entries.
func shouldPersistDrive(drive *fetcher_nfl.Drive) bool {
	if drive.Type == "event" {
		return false
	}
	return true
}

// shouldPersistPlay determines whether a play (event) should be persisted to the database.
// Returns false for non-play events (timeouts, end-of-period markers, etc.) and unofficial plays.
// Returns true only for official, confirmed plays.
func shouldPersistPlay(event *fetcher_nfl.Event) bool {
	if event.Type != "play" {
		return false
	}

	if !event.Official {
		return false
	}

	return true
}

// shouldPersistPlayStatistic determines whether a play statistic should be persisted to the database.
func shouldPersistPlayStatistic(stat *fetcher_nfl.Statistic) bool {
	// Skip statistics without a player (team-level stats)
	if stat.Player == nil {
		return false
	}

	// Skip ignoreable stat types (stat types that we don't need for our use case)
	ignoreableStatTypes := []string{"return", "first_down", "kick", "punt", "penalty", "block", "conversion" /* refers to two-point conversions */, "defense_conversion" /* indicates a defender was targeted during a two-point conversion */}
	for _, ignoreType := range ignoreableStatTypes {
		if stat.StatType == ignoreType {
			return false
		}
	}

	return true
}
