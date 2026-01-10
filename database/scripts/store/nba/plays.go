package nba

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/openbook/population-scripts/store"
	"github.com/shopspring/decimal"
)

// NBAPlayForUpsert contains the data needed to upsert an NBA play
type NBAPlayForUpsert struct {
	GameID          int
	VendorID        string
	VendorSequence  decimal.Decimal
	PeriodType      string // DB enum value as string (e.g., "quarter", "overtime")
	PeriodNumber    int
	Description     string
	VendorCreatedAt time.Time
	VendorUpdatedAt time.Time
}

// UpsertNBAPlay inserts or updates an NBA play in the database.
// Uses (game_id, vendor_id) as the unique constraint for ON CONFLICT.
// Returns the database ID of the play for use as a foreign key in statistics.
// This function accepts a transaction (pgx.Tx) to support atomic operations.
func UpsertNBAPlay(s *store.Store, ctx context.Context, tx pgx.Tx, play *NBAPlayForUpsert) (int, error) {
	query := `
		INSERT INTO nba_plays (
			game_id, vendor_id, vendor_sequence,
			period_type, period_number,
			description,
			vendor_created_at, vendor_updated_at, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4::period_type, $5, $6, $7, $8, NOW(), NOW())
		ON CONFLICT (game_id, vendor_id)
		DO UPDATE SET
			vendor_sequence = EXCLUDED.vendor_sequence,
			period_type = EXCLUDED.period_type,
			period_number = EXCLUDED.period_number,
			description = EXCLUDED.description,
			vendor_created_at = EXCLUDED.vendor_created_at,
			vendor_updated_at = EXCLUDED.vendor_updated_at,
			updated_at = NOW()
		RETURNING id
	`

	var id int
	err := tx.QueryRow(ctx, query,
		play.GameID,
		play.VendorID,
		play.VendorSequence,
		play.PeriodType,
		play.PeriodNumber,
		play.Description,
		play.VendorCreatedAt,
		play.VendorUpdatedAt,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to upsert NBA play (vendor_id: %s): %w", play.VendorID, err)
	}

	return id, nil
}
