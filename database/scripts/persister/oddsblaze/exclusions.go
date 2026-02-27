package oddsblaze

import (
	fetcher_oddsblaze "github.com/openbook/population-scripts/fetcher/oddsblaze"
)

// shouldPersistOdd returns true if the odd should be persisted as a market.
// Currently filters to only player-associated odds.
func shouldPersistOdd(odd *fetcher_oddsblaze.Odd) bool {
	return odd.Player != nil
}
