package nfl

import (
	fetcher_nfl "github.com/openbook/population-scripts/fetcher/nfl"
)

// =============================================================================
// NFL Play-by-Play Decorator
// =============================================================================
// This package decorates (enriches) raw Sportradar API responses with data that
// is missing from the API but can be derived from other fields.
//
// The decorator sits between the fetcher and persister in the data flow:
//   fetch (raw API) → decorate (enriched) → persist (to database)
//
// Current decorations:
// - None (NFL API responses are currently complete)
//
// This package exists to maintain consistency with the NBA decorator and to
// provide a place for future NFL-specific decorations if needed.
// =============================================================================

// DecoratePlayByPlay decorates a play-by-play response with derived data.
// Currently a no-op for NFL as the API responses are complete.
func DecoratePlayByPlay(pbp *fetcher_nfl.PlayByPlayResponse) *fetcher_nfl.PlayByPlayResponse {
	// No decorations needed for NFL currently
	return pbp
}
