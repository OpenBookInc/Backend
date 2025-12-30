package fetcher

// Exclusion rules for filtering API data during fetching.
// These exclusions prevent data from being added to the in-memory data store.
// For persistence-level exclusions, see persister/exclusions.go.

// shouldExcludeGame determines if a game should be excluded from fetching.
// Returns true if the game should be skipped.
func shouldExcludeGame(homeAlias, awayAlias string) bool {
	// Exclude games with TBD teams (e.g., NBA Cup games where competitors are not yet determined)
	if homeAlias == "TBD" || awayAlias == "TBD" {
		return true
	}

	return false
}
