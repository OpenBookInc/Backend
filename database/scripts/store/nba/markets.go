package nba

import (
	"context"
	"fmt"

	models_nba "github.com/openbook/shared/models/gen/nba"
	"github.com/openbook/population-scripts/store"
	"github.com/shopspring/decimal"
)

// NBAMarketForUpsert contains the data needed to upsert an NBA market
type NBAMarketForUpsert struct {
	GameID       int
	IndividualID int
	MarketType   models_nba.PlayerProp
	MarketLine   decimal.Decimal
}

// UpsertNBAMarket inserts or updates an NBA market in the database.
// Uses (game_id, individual_id, market_type, market_line) as the unique constraint for ON CONFLICT.
func UpsertNBAMarket(s *store.Store, ctx context.Context, m *NBAMarketForUpsert) error {
	query := `
		INSERT INTO nba_markets (
			game_id, individual_id, market_type, market_line,
			created_at, updated_at
		)
		VALUES ($1, $2, $3::nba_player_prop_type, $4, NOW(), NOW())
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
		return fmt.Errorf("failed to upsert NBA market (game_id=%d, individual_id=%d, type=%s, line=%s): %w",
			m.GameID, m.IndividualID, m.MarketType, m.MarketLine, err)
	}

	return nil
}
