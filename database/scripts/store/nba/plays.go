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
	GameID          string
	SportradarID    string
	VendorSequence  decimal.Decimal
	PeriodType      string // DB enum value as string (e.g., "quarter", "overtime")
	PeriodNumber    int
	Description     string
	VendorCreatedAt time.Time
	VendorUpdatedAt time.Time
}

// UpsertNBAPlay inserts or updates an NBA play in the database.
// Uses (game_id, sportradar_id) as the unique constraint for ON CONFLICT.
// Returns the database ID of the play for use as a foreign key in statistics.
// This function accepts a transaction (pgx.Tx) to support atomic operations.
// Sets vendor_deleted = FALSE on both insert and update.
func UpsertNBAPlay(s *store.Store, ctx context.Context, tx pgx.Tx, play *NBAPlayForUpsert) (int, error) {
	query := `
		INSERT INTO nba_plays (
			game_id, sportradar_id, vendor_sequence,
			period_type, period_number,
			description,
			vendor_deleted, vendor_created_at, vendor_updated_at, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4::period_type, $5, $6, FALSE, $7, $8, NOW(), NOW())
		ON CONFLICT (game_id, sportradar_id)
		DO UPDATE SET
			vendor_sequence = EXCLUDED.vendor_sequence,
			period_type = EXCLUDED.period_type,
			period_number = EXCLUDED.period_number,
			description = EXCLUDED.description,
			vendor_deleted = FALSE,
			vendor_created_at = EXCLUDED.vendor_created_at,
			vendor_updated_at = EXCLUDED.vendor_updated_at,
			updated_at = NOW()
		RETURNING id
	`

	var id int
	err := tx.QueryRow(ctx, query,
		play.GameID,
		play.SportradarID,
		play.VendorSequence,
		play.PeriodType,
		play.PeriodNumber,
		play.Description,
		play.VendorCreatedAt,
		play.VendorUpdatedAt,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to upsert NBA play (sportradar_id: %s): %w", play.SportradarID, err)
	}

	return id, nil
}

// NBAPlayBasic holds the minimal play data needed for deletion checking
type NBAPlayBasic struct {
	ID int
}

// GetNBAPlayBySportradarID retrieves an NBA play by game_id and sportradar_id.
// Only returns plays where vendor_deleted = FALSE.
func GetNBAPlayBySportradarID(s *store.Store, ctx context.Context, gameID string, sportradarID string) (*NBAPlayBasic, error) {
	query := `
		SELECT id
		FROM nba_plays
		WHERE game_id = $1 AND sportradar_id = $2 AND vendor_deleted = FALSE
	`

	var play NBAPlayBasic
	err := s.Pool().QueryRow(ctx, query, gameID, sportradarID).Scan(&play.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get NBA play with sportradar_id %s: %w", sportradarID, err)
	}

	return &play, nil
}

// MarkNBAPlayDeleted marks an NBA play as vendor_deleted = TRUE.
// This function accepts a transaction (pgx.Tx) to support atomic operations.
func MarkNBAPlayDeleted(s *store.Store, ctx context.Context, tx pgx.Tx, playID int) error {
	query := `
		UPDATE nba_plays
		SET vendor_deleted = TRUE, updated_at = NOW()
		WHERE id = $1
	`
	_, err := tx.Exec(ctx, query, playID)
	if err != nil {
		return fmt.Errorf("failed to mark play as deleted (play_id: %d): %w", playID, err)
	}

	return nil
}
