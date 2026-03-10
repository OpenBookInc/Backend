package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	models_nfl "github.com/openbook/shared/models/nfl"
)

// GameStatusForUpsert contains the data needed to upsert a game status
type GameStatusForUpsert struct {
	GameID string
	Status string // DB enum value as string (e.g., "scheduled", "in_progress")
}

// UpsertGameStatus inserts or updates a game's status in the database.
// Uses game_id as the unique identifier (primary key) for ON CONFLICT.
// This function accepts a transaction (pgx.Tx) to support atomic operations.
func (s *Store) UpsertGameStatus(ctx context.Context, tx pgx.Tx, status *GameStatusForUpsert) error {
	query := `
		INSERT INTO game_statuses (game_id, status, updated_at)
		VALUES ($1, $2::game_status_type, NOW())
		ON CONFLICT (game_id)
		DO UPDATE SET
			status = EXCLUDED.status
	`

	_, err := tx.Exec(ctx, query,
		status.GameID,
		status.Status,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert game status (game_id: %s): %w", status.GameID, err)
	}

	return nil
}

// GetGameStatusByGameID retrieves a game's status by game_id
func (s *Store) GetGameStatusByGameID(ctx context.Context, gameID string) (*models_nfl.GameStatus, error) {
	query := `
		SELECT game_id, status, updated_at
		FROM game_statuses
		WHERE game_id = $1
	`

	var status models_nfl.GameStatus
	err := s.pool.QueryRow(ctx, query, gameID).Scan(
		&status.GameID,
		&status.Status,
		&status.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get game status with game_id %s: %w", gameID, err)
	}

	return &status, nil
}
