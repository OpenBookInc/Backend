package persister

import (
	"errors"

	"github.com/openbook/population-scripts/store"
)

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

// shouldExcludeGameForTeamLookupErr checks whether a team lookup error indicates
// the team doesn't exist in our database. The hierarchy and team profile
// Sportradar endpoints don't include special teams (e.g., All-Star teams), so
// we can't persist them to the database and thus can't persist games that
// reference those teams either.
func shouldExcludeGameForTeamLookupErr(err error) bool {
	return errors.Is(err, store.ErrTeamNotFound)
}
