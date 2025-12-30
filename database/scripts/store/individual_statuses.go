package store

import (
	"context"
	"fmt"

	"github.com/openbook/shared/models"
)

// IndividualStatusForUpsert contains the data needed to upsert an individual status
type IndividualStatusForUpsert struct {
	IndividualID int
	Status       string // DB enum value as string (e.g., "active", "questionable")
}

// UpsertIndividualStatus inserts or updates an individual's status in the database
// Uses individual_id as the unique identifier (ON CONFLICT) - one status per player
// Returns the database ID of the individual status
func (s *Store) UpsertIndividualStatus(ctx context.Context, status *IndividualStatusForUpsert) (int, error) {
	query := `
		INSERT INTO individual_statuses (individual_id, status)
		VALUES ($1, $2::individual_status_type)
		ON CONFLICT (individual_id)
		DO UPDATE SET
			status = EXCLUDED.status
		RETURNING id
	`

	var id int
	err := s.pool.QueryRow(ctx, query,
		status.IndividualID,
		status.Status,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to upsert individual status (individual_id: %d): %w", status.IndividualID, err)
	}

	return id, nil
}

// GetIndividualStatusByIndividualID retrieves an individual's status by individual_id
func (s *Store) GetIndividualStatusByIndividualID(ctx context.Context, individualID int64) (*models.IndividualStatus, error) {
	query := `
		SELECT id, individual_id, status
		FROM individual_statuses
		WHERE individual_id = $1
	`

	var status models.IndividualStatus
	err := s.pool.QueryRow(ctx, query, individualID).Scan(
		&status.ID,
		&status.IndividualID,
		&status.Status,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get individual status with individual_id %d: %w", individualID, err)
	}

	return &status, nil
}
