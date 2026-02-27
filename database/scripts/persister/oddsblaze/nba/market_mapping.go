package nba

import (
	"errors"
	"fmt"

	models_nba "github.com/openbook/shared/models/gen/nba"
)

// Known non-player-prop markets that should be silently skipped
var ignoreableMarkets = map[string]struct{}{
	"Moneyline":                        {},
	"Point Spread":                     {},
	"Total Points":                     {},
	"1st Half Point Spread":            {},
	"1st Half Total Points":            {},
	"1st Half Total Points Odd/Even":   {},
	"2nd Half Moneyline":               {},
	"2nd Half Point Spread":            {},
	"2nd Half Total Points":            {},
	"2nd Half Total Points Odd/Even":   {},
	"2nd Quarter Moneyline":            {},
	"2nd Quarter Point Spread":         {},
	"2nd Quarter Total Points":         {},
	"3rd Quarter Moneyline":            {},
	"3rd Quarter Point Spread":         {},
	"3rd Quarter Total Points":         {},
	"4th Quarter Moneyline":            {},
	"4th Quarter Point Spread":         {},
	"4th Quarter Total Points":         {},
	"First Basket":                     {},
	"First Basket Method 3-Way":        {},
	"First Field Goal":                 {},
	"1st Quarter Player Points":        {},
	"1st Quarter Player Assists":       {},
	"1st Quarter Player Rebounds":      {},
	"1st 3 Minutes Player Assists":    {},
	"1st 3 Minutes Player Points":     {},
	"1st 3 Minutes Player Rebounds":   {},
	"Home Team First Field Goal":              {},
	"Away Team First Field Goal":              {},
	"Home Team First Field Goal Method 2-Way": {},
	"Away Team First Field Goal Method 2-Way": {},
}

// marketToPlayerProp maps OddsBlaze market names to NBA player prop types
var marketToPlayerProp = map[string]models_nba.PlayerProp{
	"Player Points":                      models_nba.PlayerPropPoints,
	"Player Assists":                     models_nba.PlayerPropAssists,
	"Player Rebounds":                    models_nba.PlayerPropRebounds,
	"Player Blocks":                      models_nba.PlayerPropBlocks,
	"Player Steals":                      models_nba.PlayerPropSteals,
	"Player Threes Made":                 models_nba.PlayerPropThreePointMakes,
	"Player Points + Assists":            models_nba.PlayerPropPointsAssists,
	"Player Points + Rebounds":           models_nba.PlayerPropPointsRebounds,
	"Player Points + Rebounds + Assists": models_nba.PlayerPropPointsAssistsRebounds,
	"Player Rebounds + Assists":          models_nba.PlayerPropReboundsAssists,
	"Player Blocks + Steals":             models_nba.PlayerPropBlocksSteals,
	"Player Double Double":               models_nba.PlayerPropDoubleDouble,
	"Player Triple Double":               models_nba.PlayerPropTripleDouble,
}

// ErrNotPlayerProp is returned when a market is a known non-player-prop market.
var ErrNotPlayerProp = errors.New("not a player prop market")

// MapMarketToPlayerPropType maps an OddsBlaze market name to an NBA player prop type.
// Returns ErrNotPlayerProp for known non-player-prop markets.
// Returns an error for unknown markets.
func MapMarketToPlayerPropType(market string) (models_nba.PlayerProp, error) {
	if prop, ok := marketToPlayerProp[market]; ok {
		return prop, nil
	}

	if _, ok := ignoreableMarkets[market]; ok {
		return "", ErrNotPlayerProp
	}

	return "", fmt.Errorf("unknown NBA market: %q", market)
}
