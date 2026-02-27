package nfl

import (
	"errors"
	"fmt"

	models_nfl "github.com/openbook/shared/models/gen/nfl"
)

// Known non-player-prop markets that should be silently skipped
var ignoreableMarkets = map[string]struct{}{}

// ErrNotPlayerProp is returned when a market is a known non-player-prop market.
var ErrNotPlayerProp = errors.New("not a player prop market")

// MapMarketToPlayerPropType maps an OddsBlaze market name to an NFL player prop type.
// NFL market mapping is not yet implemented.
func MapMarketToPlayerPropType(market string) (models_nfl.PlayerProp, error) {
	if _, ok := ignoreableMarkets[market]; ok {
		return "", ErrNotPlayerProp
	}

	return "", fmt.Errorf("NFL market mapping not yet implemented for market: %q", market)
}
