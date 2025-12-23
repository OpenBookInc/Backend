package fetcher

// Exclusion rules for filtering API data during population
// This file centralizes all manual exclusions to make them easy to review and modify

// shouldExcludeGame determines if a game should be excluded from population
// Returns true if the game should be skipped
func shouldExcludeGame(homeAlias, awayAlias string) bool {
	// Exclude games with TBD teams (e.g., NBA Cup games where competitors are not yet determined)
	if homeAlias == "TBD" || awayAlias == "TBD" {
		return true
	}

	return false
}

// shouldExcludeTeam determines if a team should be excluded from population
// Returns true if the team should be skipped
func shouldExcludeTeam(alias string) bool {
	// Currently no team exclusions
	return false
}

// shouldExcludePlayer determines if a player should be excluded from population
// Returns true if the player should be skipped
func shouldExcludePlayer(playerID string) bool {
	// Currently no player exclusions
	return false
}
