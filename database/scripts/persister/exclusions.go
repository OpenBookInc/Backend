package persister

// Exclusion rules for filtering API data during persistence.
// These exclusions prevent data from being written to the database.
// For sport-specific persistence exclusions, see persister/nfl/exclusions.go
// and persister/nba/exclusions.go.

// shouldExcludeGame determines if a game should be excluded from persistence.
// Returns true if the game should be skipped.
func shouldExcludeGame(homeID, awayID, homeAlias, awayAlias string) bool {
	// Exclude games with TBD teams (e.g., NBA Cup games where competitors are not yet determined)
	if homeAlias == "TBD" || awayAlias == "TBD" {
		return true
	}

	// Exclude games with empty team IDs (e.g., Super Bowl before conference championships are played)
	if homeID == "" || awayID == "" {
		return true
	}

	return false
}
