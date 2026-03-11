package oddsblaze

import (
	"context"
	"errors"
	"fmt"

	fetcher_oddsblaze "github.com/openbook/population-scripts/fetcher/oddsblaze"
	persister_oddsblaze_nba "github.com/openbook/population-scripts/persister/oddsblaze/nba"
	persister_oddsblaze_nfl "github.com/openbook/population-scripts/persister/oddsblaze/nfl"
	"github.com/openbook/population-scripts/store"
	store_nba "github.com/openbook/population-scripts/store/nba"
	store_nfl "github.com/openbook/population-scripts/store/nfl"
	"github.com/openbook/shared/models/gen"
	models_nba "github.com/openbook/shared/models/gen/nba"
	models_nfl "github.com/openbook/shared/models/gen/nfl"
	"github.com/shopspring/decimal"
)

// PersistMarkets processes an OddsBlaze odds response and persists player prop markets.
// Returns the number of markets persisted.
func PersistMarkets(ctx context.Context, dbStore *store.Store, marketEntity gen.MarketEntity, sportsbook gen.Sportsbook, oddsResp *fetcher_oddsblaze.OddsResponse) (int, error) {
	count := 0

	for _, event := range oddsResp.Events {
		game, err := dbStore.GetGameByVendorID(ctx, gen.VendorOddsBlaze, event.ID)
		if err != nil {
			return count, fmt.Errorf("failed to look up game for event %s: %w", event.ID, err)
		}

		for i := range event.Odds {
			odd := &event.Odds[i]

			if !shouldPersistOdd(odd) {
				continue
			}

			propType, err := mapMarketToPlayerPropType(marketEntity, odd.Market)
			if errors.Is(err, persister_oddsblaze_nba.ErrNotPlayerProp) || errors.Is(err, persister_oddsblaze_nfl.ErrNotPlayerProp) {
				continue
			}
			if err != nil {
				return count, fmt.Errorf("failed to map market %q for event %s: %w", odd.Market, event.ID, err)
			}

			individual, err := dbStore.GetIndividualByVendorID(ctx, gen.VendorOddsBlaze, odd.Player.ID)
			if err != nil {
				return count, fmt.Errorf("failed to look up individual %s (%s) for event %s: %w",
					odd.Player.ID, odd.Player.Name, event.ID, err)
			}

			line := getMarketLine(odd, propType)

			var marketID string

			switch marketEntity {
			case gen.MarketEntityNbaMarket:
				nbaProp := models_nba.PlayerProp(propType)
				marketID, err = store_nba.UpsertNBAMarket(dbStore, ctx, &store_nba.NBAMarketForUpsert{
					GameID:       game.ID.String(),
					IndividualID: individual.ID.String(),
					MarketType:   nbaProp,
					MarketLine:   line,
				})
				if err != nil {
					return count, fmt.Errorf("failed to upsert NBA market: %w", err)
				}
			case gen.MarketEntityNflMarket:
				nflProp := models_nfl.PlayerProp(propType)
				marketID, err = store_nfl.UpsertNFLMarket(dbStore, ctx, &store_nfl.NFLMarketForUpsert{
					GameID:       game.ID.String(),
					IndividualID: individual.ID.String(),
					MarketType:   nflProp,
					MarketLine:   line,
				})
				if err != nil {
					return count, fmt.Errorf("failed to upsert NFL market: %w", err)
				}
			default:
				return count, fmt.Errorf("unsupported market entity type: %s", marketEntity)
			}

			// Persist the OddsBlaze market ID mapping
			side, err := mapSelectionSideToMarketSide(odd)
			if err != nil {
				return count, fmt.Errorf("failed to map selection side for odd %s in event %s: %w", odd.ID, event.ID, err)
			}

			if err := dbStore.UpsertOddsBlazeMarketID(ctx, &store.OddsBlazeMarketIDForUpsert{
				EntityType:  marketEntity,
				EntityID:    marketID,
				Sportsbook:  sportsbook,
				Side:        side,
				OddsBlazeID: odd.ID,
			}); err != nil {
				return count, fmt.Errorf("failed to upsert OddsBlaze market ID for odd %s: %w", odd.ID, err)
			}

			count++
		}
	}

	return count, nil
}

// mapMarketToPlayerPropType dispatches to the league-specific market mapping function.
// Returns the prop type as a string that can be cast to the league-specific type.
func mapMarketToPlayerPropType(marketEntity gen.MarketEntity, market string) (string, error) {
	switch marketEntity {
	case gen.MarketEntityNbaMarket:
		prop, err := persister_oddsblaze_nba.MapMarketToPlayerPropType(market)
		return string(prop), err
	case gen.MarketEntityNflMarket:
		prop, err := persister_oddsblaze_nfl.MapMarketToPlayerPropType(market)
		return string(prop), err
	default:
		return "", fmt.Errorf("unsupported market entity type: %s", marketEntity)
	}
}

// mapSelectionSideToMarketSide maps OddsBlaze selection side strings to our MarketSide enum.
// "Over" and "Yes" map to over; "Under" maps to under.
func mapSelectionSideToMarketSide(odd *fetcher_oddsblaze.Odd) (gen.MarketSide, error) {
	if odd.Selection == nil || odd.Selection.Side == "" {
		return "", fmt.Errorf("odd %s has no selection side", odd.ID)
	}

	switch odd.Selection.Side {
	case "Over", "Yes":
		return gen.MarketSideOver, nil
	case "Under":
		return gen.MarketSideUnder, nil
	default:
		return "", fmt.Errorf("unknown selection side %q for odd %s", odd.Selection.Side, odd.ID)
	}
}

// getMarketLine returns the appropriate line for a market.
// Yes/No props (double double, triple double) use 0.5.
// All other props use the selection line from the API.
func getMarketLine(odd *fetcher_oddsblaze.Odd, propType string) decimal.Decimal {
	if isYesNoPropType(propType) {
		return decimal.NewFromFloat(0.5)
	}

	if odd.Selection != nil && odd.Selection.Line != nil {
		return decimal.NewFromFloat(*odd.Selection.Line)
	}

	return decimal.Zero
}

// isYesNoPropType returns true if the prop type is a yes/no prop (no numeric line).
func isYesNoPropType(propType string) bool {
	return propType == string(models_nba.PlayerPropDoubleDouble) ||
		propType == string(models_nba.PlayerPropTripleDouble)
}
