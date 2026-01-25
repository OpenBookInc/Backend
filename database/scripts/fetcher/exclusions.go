package fetcher

// Exclusion rules for filtering API data during fetching.
// These exclusions prevent data from being added to the in-memory data store.
// For persistence-level exclusions, see persister/exclusions.go.

// shouldExcludeGame determines if a game should be excluded from fetching.
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
