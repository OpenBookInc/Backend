package nfl

import (
	"context"
	"fmt"

	models_nfl "github.com/openbook/shared/models/gen/nfl"
	"github.com/openbook/population-scripts/store"
	"github.com/shopspring/decimal"
)

// NFLMarketForUpsert contains the data needed to upsert an NFL market
type NFLMarketForUpsert struct {
	GameID       int
	IndividualID int
	MarketType   models_nfl.PlayerProp
	MarketLine   decimal.Decimal
}

// UpsertNFLMarket inserts or updates an NFL market in the database.
// Uses (game_id, individual_id, market_type, market_line) as the unique constraint for ON CONFLICT.
func UpsertNFLMarket(s *store.Store, ctx context.Context, m *NFLMarketForUpsert) error {
	query := `
		INSERT INTO nfl_markets (
			game_id, individual_id, market_type, market_line,
			created_at, updated_at
		)
		VALUES ($1, $2, $3::nfl_player_prop_type, $4, NOW(), NOW())
		ON CONFLICT (game_id, individual_id, market_type, market_line)
		DO UPDATE SET updated_at = NOW()
	`

	_, err := s.Pool().Exec(ctx, query,
		m.GameID,
		m.IndividualID,
		m.MarketType,
		m.MarketLine,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert NFL market (game_id=%d, individual_id=%d, type=%s, line=%s): %w",
			m.GameID, m.IndividualID, m.MarketType, m.MarketLine, err)
	}

	return nil
}
