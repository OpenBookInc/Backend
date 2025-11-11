package store

import (
	"context"
	"fmt"

	"github.com/openbook/population-scripts/models"
)

// UpsertGame inserts or updates a game in the database
// Uses vendor_id as the unique identifier (ON CONFLICT)
// Returns the database ID of the game
func (s *Store) UpsertGame(ctx context.Context, game *models.Game) (int, error) {
	query := `
		INSERT INTO games (contender_id_a, contender_id_b, vendor_id, scheduled_start_time)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (vendor_id)
		DO UPDATE SET
			contender_id_a = EXCLUDED.contender_id_a,
			contender_id_b = EXCLUDED.contender_id_b,
			scheduled_start_time = EXCLUDED.scheduled_start_time
		RETURNING id
	`

	var id int
	err := s.pool.QueryRow(ctx, query,
		game.ContenderIDA,
		game.ContenderIDB,
		game.VendorID,
		game.ScheduledStartTime,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to upsert game (vendor_id: %s): %w", game.VendorID, err)
	}

	return id, nil
}

// GetGameByVendorID retrieves a game by vendor_id
func (s *Store) GetGameByVendorID(ctx context.Context, vendorID string) (*models.Game, error) {
	query := `
		SELECT id, contender_id_a, contender_id_b, vendor_id, scheduled_start_time
		FROM games
		WHERE vendor_id = $1
	`

	var game models.Game
	err := s.pool.QueryRow(ctx, query, vendorID).Scan(
		&game.ID,
		&game.ContenderIDA,
		&game.ContenderIDB,
		&game.VendorID,
		&game.ScheduledStartTime,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get game with vendor_id %s: %w", vendorID, err)
	}

	return &game, nil
}
